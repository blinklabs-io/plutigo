package syn

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/bits"
	"unicode/utf8"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
)

// Default resource bounds for untrusted programs, applied to both the FLAT
// decoder and the text parser before the protocol's own limits are known. They
// sit far above any script a transaction can carry, so they only stop inputs
// that would otherwise allocate in proportion to attacker-chosen counts.
const (
	// maxInputBytes bounds the encoded or textual size of a program.
	maxInputBytes = 16 << 20
	// maxProgramNodes bounds the terms plus constant and PlutusData collection
	// items of a program.
	maxProgramNodes = 1 << 20
	// maxCollectionWidth bounds term lists and constant or PlutusData
	// collections. A unit list item encodes in one bit, so a list this wide
	// needs 64 KiB, four times the largest script a transaction carries today.
	maxCollectionWidth = 1 << 19
)

func errTooMany(what string, limit int) error {
	return fmt.Errorf("too many %s: limit is %d", what, limit)
}

// constrFieldLimit returns the widest constr the protocol admits. The
// protocol bound is enforced while decoding so an over-wide field list is
// rejected before it is allocated.
func constrFieldLimit(protocolMajor uint) int {
	if protocolMajor >= builtin.VanRossemProtoVersion {
		return maxConstrFieldsPV11
	}
	return maxCollectionWidth
}

const (
	// Arena chunks grow geometrically from decodeTermMinChunkSize to
	// decodeTermChunkSize. A script populates a handful of node types densely
	// and most others with a few nodes or none, so a fixed full-size first
	// chunk per type wasted most of a fresh decode's allocation on small
	// populations.
	decodeTermMinChunkSize = 64
	decodeTermChunkSize    = 384
	decodeRetainVarCap     = 16
	decodeRetainApplyCap   = 16
	decodeRetainListCap    = 32
	decodeRetainBytesCap   = 8
)

var (
	emptyDeBruijnTermList = []Term[DeBruijn]{}
	emptyConstantList     = []IConstant{}
	emptyByteArray        = []byte{}
)

type DeBruijnDecoder struct {
	decoder decoder
	arena   termArena[DeBruijn]
	consts  constantArena
}

func NewDeBruijnDecoder() *DeBruijnDecoder {
	return &DeBruijnDecoder{}
}

func DecodeDeBruijn(bytes []byte) (*Program[DeBruijn], error) {
	return NewDeBruijnDecoder().Decode(bytes)
}

func decodeDeBruijn(bytes []byte) (*Program[DeBruijn], error) {
	d := newDecoder(bytes)
	arena := newTermArena[DeBruijn]()
	consts := &constantArena{}
	return decodeDeBruijnProgram(d, arena, consts, nil)
}

func (d *DeBruijnDecoder) Decode(bytes []byte) (*Program[DeBruijn], error) {
	d.decoder.reset(bytes)
	d.arena.reset()
	d.consts.reset()
	return decodeDeBruijnProgram(&d.decoder, &d.arena, &d.consts, nil)
}

func Decode[T Binder](bytes []byte) (*Program[T], error) {
	return decodeProgram[T](bytes, nil)
}

func decodeWithContext[T Binder](bytes []byte, context ProgramContext) (*Program[T], error) {
	return decodeProgram[T](bytes, &context)
}

func decodeProgram[T Binder](bytes []byte, context *ProgramContext) (*Program[T], error) {
	var zero T
	if _, ok := any(zero).(DeBruijn); ok {
		d := newDecoder(bytes)
		arena := newTermArena[DeBruijn]()
		consts := &constantArena{}
		program, err := decodeDeBruijnProgram(d, arena, consts, context)
		if err != nil {
			return nil, err
		}
		return any(program).(*Program[T]), nil
	}

	d := newDecoder(bytes)
	arena := newTermArena[T]()
	version, err := decodeProgramVersion(d, context)
	if err != nil {
		return nil, err
	}
	arena.sizeForInput(len(bytes))
	terms, err := decodeTermWithArena[T](d, arena)
	if err != nil {
		return nil, err
	}
	program := &Program[T]{Version: version, Term: terms}
	if err := d.filler(); err != nil {
		return nil, err
	}
	if err := d.ensureFullyConsumed(); err != nil {
		return nil, err
	}
	return program, nil
}

func decodeDeBruijnProgram(
	d *decoder,
	arena *termArena[DeBruijn],
	consts *constantArena,
	context *ProgramContext,
) (*Program[DeBruijn], error) {
	version, err := decodeProgramVersion(d, context)
	if err != nil {
		return nil, err
	}

	arena.sizeForInput(len(d.buffer))
	terms, err := decodeTermDeBruijnWithArena(d, arena, consts, 0)
	if err != nil {
		return nil, err
	}

	program := &Program[DeBruijn]{
		Version: version,
		Term:    terms,
	}

	if err := d.filler(); err != nil {
		return nil, err
	}

	if err := d.ensureFullyConsumed(); err != nil {
		return nil, err
	}

	return program, nil
}

func DecodeTerm[T Binder](d *decoder) (Term[T], error) {
	return decodeTermWithArena(d, newTermArena[T]())
}

func decodeProgramVersion(
	d *decoder,
	context *ProgramContext,
) (lang.LanguageVersion, error) {
	if len(d.buffer) > maxInputBytes {
		return lang.LanguageVersion{}, errors.New("input too large")
	}
	major, err := d.word()
	if err != nil {
		return lang.LanguageVersion{}, err
	}
	minor, err := d.word()
	if err != nil {
		return lang.LanguageVersion{}, err
	}
	patch, err := d.word()
	if err != nil {
		return lang.LanguageVersion{}, err
	}
	if major > math.MaxUint32 || minor > math.MaxUint32 ||
		patch > math.MaxUint32 {
		return lang.LanguageVersion{}, errors.New("version numbers too large")
	}
	version := lang.LanguageVersion{uint32(major), uint32(minor), uint32(patch)}
	if context != nil {
		if err := validateLedgerLanguageAvailability(*context); err != nil {
			return lang.LanguageVersion{}, err
		}
		d.maxConstrFields = constrFieldLimit(context.ProtocolMajor)
	}
	return version, nil
}

func decodeTermDeBruijnWithArena(
	d *decoder,
	arena *termArena[DeBruijn],
	consts *constantArena,
	depth int,
) (Term[DeBruijn], error) {
	// Depth is threaded as a parameter rather than tracked via
	// d.enter()/defer d.leave(): a per-node defer in this hot recursive
	// decoder is not open-coded and routes every return through the Go
	// runtime's panic/defer machinery, which measured ~39% of decode time.
	if depth >= maxFlatDecodeDepth {
		return nil, errors.New("term nesting too deep")
	}
	if err := d.node(); err != nil {
		return nil, err
	}

	tag, err := d.bits4()
	if err != nil {
		return nil, err
	}

	switch tag {
	case VarTag:
		name, err := decodeDeBruijnBinder(d)
		if err != nil {
			return nil, err
		}
		return arena.allocVar(name), nil
	case DelayTag:
		t, err := decodeTermDeBruijnWithArena(d, arena, consts, depth+1)
		if err != nil {
			return nil, err
		}
		return arena.allocDelay(t), nil
	case LambdaTag:
		t, err := decodeTermDeBruijnWithArena(d, arena, consts, depth+1)
		if err != nil {
			return nil, err
		}
		return arena.allocLambda(DeBruijn(0), t), nil
	case ApplyTag:
		function, err := decodeTermDeBruijnWithArena(d, arena, consts, depth+1)
		if err != nil {
			return nil, err
		}
		argument, err := decodeTermDeBruijnWithArena(d, arena, consts, depth+1)
		if err != nil {
			return nil, err
		}
		return arena.allocApply(function, argument), nil
	case ConstantTag:
		constant, err := decodeConstantWithArena(d, consts)
		if err != nil {
			return nil, err
		}
		return arena.allocConstant(constant), nil
	case ForceTag:
		t, err := decodeTermDeBruijnWithArena(d, arena, consts, depth+1)
		if err != nil {
			return nil, err
		}
		return arena.allocForce(t), nil
	case ErrorTag:
		return arena.allocError(), nil
	case BuiltinTag:
		builtinTag, err := d.bits7()
		if err != nil {
			return nil, err
		}
		fn, err := builtin.FromByte(builtinTag)
		if err != nil {
			return nil, err
		}
		return arena.allocBuiltin(fn), nil
	case ConstrTag:
		constrTag, err := d.word64()
		if err != nil {
			return nil, err
		}
		fields, err := decodeTermListDeBruijnWithArena(
			d, arena, consts, depth+1, d.maxConstrFields,
		)
		if err != nil {
			return nil, err
		}
		return arena.allocConstr(constrTag, fields), nil
	case CaseTag:
		constr, err := decodeTermDeBruijnWithArena(d, arena, consts, depth+1)
		if err != nil {
			return nil, err
		}
		branches, err := decodeTermListDeBruijnWithArena(
			d, arena, consts, depth+1, maxCollectionWidth,
		)
		if err != nil {
			return nil, err
		}
		return arena.allocCase(constr, branches), nil
	default:
		return nil, fmt.Errorf("invalid term tag: %d", tag)
	}
}

func decodeTermWithArena[T Binder](d *decoder, arena *termArena[T]) (Term[T], error) {
	if err := d.enter(); err != nil {
		return nil, err
	}
	defer d.leave()
	if err := d.node(); err != nil {
		return nil, err
	}

	tag, e := d.bits4()
	if e != nil {
		return nil, e
	}

	var term Term[T]

	switch tag {
	case VarTag:
		name, err := decodeVarBinder[T](d)
		if err != nil {
			return nil, err
		}

		term = arena.allocVar(name)
	case DelayTag:
		t, err := decodeTermWithArena[T](d, arena)
		if err != nil {
			return nil, err
		}

		term = arena.allocDelay(t)
	case LambdaTag:
		name, err := decodeParameterBinder[T](d)
		if err != nil {
			return nil, err
		}

		t, err := decodeTermWithArena[T](d, arena)
		if err != nil {
			return nil, err
		}

		term = arena.allocLambda(name, t)
	case ApplyTag:
		function, err := decodeTermWithArena[T](d, arena)
		if err != nil {
			return nil, err
		}

		argument, err := decodeTermWithArena[T](d, arena)
		if err != nil {
			return nil, err
		}

		term = arena.allocApply(function, argument)
	case ConstantTag:
		constant, err := DecodeConstant(d)
		if err != nil {
			return nil, err
		}

		term = arena.allocConstant(constant)
	case ForceTag:
		t, err := decodeTermWithArena[T](d, arena)
		if err != nil {
			return nil, err
		}

		term = arena.allocForce(t)
	case ErrorTag:
		term = arena.allocError()
	case BuiltinTag:
		builtinTag, err := d.bits7()
		if err != nil {
			return nil, err
		}

		fn, err := builtin.FromByte(builtinTag)
		if err != nil {
			return nil, err
		}

		term = arena.allocBuiltin(fn)
	case ConstrTag:
		constrTag, err := d.word64()
		if err != nil {
			return nil, err
		}

		fields, err := decodeTermListWithArena(d, arena, d.maxConstrFields)
		if err != nil {
			return nil, err
		}

		term = arena.allocConstr(constrTag, fields)
	case CaseTag:
		constr, err := decodeTermWithArena[T](d, arena)
		if err != nil {
			return nil, err
		}

		branches, err := decodeTermListWithArena(d, arena, maxCollectionWidth)
		if err != nil {
			return nil, err
		}

		term = arena.allocCase(constr, branches)
	default:
		return nil, fmt.Errorf("invalid term tag: %d", tag)
	}

	return term, nil
}

func decodeTermListWithArena[T Binder](
	d *decoder,
	arena *termArena[T],
	maxWidth int,
) ([]Term[T], error) {
	result := arena.allocTermList(4)[:0]

	for {
		bit, err := d.bit()
		if err != nil {
			return nil, err
		}
		if !bit {
			break
		}
		if len(result) >= maxWidth {
			return nil, errTooMany("term list items", maxWidth)
		}
		item, err := decodeTermWithArena(d, arena)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, nil
}

func decodeTermListDeBruijnWithArena(
	d *decoder,
	arena *termArena[DeBruijn],
	consts *constantArena,
	depth int,
	maxWidth int,
) ([]Term[DeBruijn], error) {
	var result []Term[DeBruijn]

	for {
		bit, err := d.bit()
		if err != nil {
			return nil, err
		}
		if !bit {
			if result == nil {
				return emptyDeBruijnTermList, nil
			}
			return result, nil
		}
		if len(result) >= maxWidth {
			return nil, errTooMany("term list items", maxWidth)
		}
		if result == nil {
			result = arena.allocTermList(4)[:0]
		}
		item, err := decodeTermDeBruijnWithArena(d, arena, consts, depth)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
}

type arenaChunks[S any] struct {
	chunks      [][]S
	chunkIdx    int
	offset      int
	activeChunk []S // cached pointer to chunks[chunkIdx]; nil when chunkIdx >= len(chunks)
	// firstSize, when larger than decodeTermMinChunkSize, sizes the first
	// chunk of an empty arena. Later chunks still grow geometrically.
	firstSize int
}

func (a *arenaChunks[S]) used() int {
	if len(a.chunks) == 0 {
		return 0
	}
	used := a.offset
	for i := 0; i < a.chunkIdx && i < len(a.chunks); i++ {
		used += len(a.chunks[i])
	}
	return used
}

// nextArenaChunkSize doubles the previous chunk's length, bounded by
// [decodeTermMinChunkSize, decodeTermChunkSize].
func nextArenaChunkSize(prev int) int {
	size := prev * 2
	if size < decodeTermMinChunkSize {
		return decodeTermMinChunkSize
	}
	if size > decodeTermChunkSize {
		return decodeTermChunkSize
	}
	return size
}

// alloc returns the next free slot. The fast path uses the cached
// activeChunk so we avoid reloading chunks[chunkIdx] on every call.
func (a *arenaChunks[S]) alloc() *S {
	if a.offset < len(a.activeChunk) {
		slot := &a.activeChunk[a.offset]
		a.offset++
		return slot
	}
	return a.allocSlow()
}

//go:noinline
func (a *arenaChunks[S]) allocSlow() *S {
	if nextIdx := a.chunkIdx + 1; nextIdx < len(a.chunks) {
		chunk := a.chunks[nextIdx]
		if chunk != nil {
			a.chunkIdx = nextIdx
			a.activeChunk = chunk
			a.offset = 1
			return &chunk[0]
		}
	}

	var size int
	if n := len(a.chunks); n > 0 {
		size = nextArenaChunkSize(len(a.chunks[n-1]))
	} else {
		size = max(a.firstSize, decodeTermMinChunkSize)
	}
	chunk := make([]S, size)
	a.chunks = append(a.chunks, chunk)
	a.chunkIdx = len(a.chunks) - 1
	a.activeChunk = chunk
	a.offset = 1
	return &chunk[0]
}

func (a *arenaChunks[S]) setActiveChunkAfterReset() {
	if len(a.chunks) > 0 {
		a.activeChunk = a.chunks[0]
	} else {
		a.activeChunk = nil
	}
}

func (a *arenaChunks[S]) reset(retainCap int) {
	used := a.used()
	retained := len(a.chunks)
	if retained > retainCap {
		retained = retainCap
	}
	if used > 0 && retained > 0 {
		remaining := used
		for i := 0; i < retained && remaining > 0; i++ {
			chunk := a.chunks[i]
			if chunk == nil {
				continue
			}
			clearCount := len(chunk)
			if remaining < clearCount {
				clearCount = remaining
			}
			clear(chunk[:clearCount])
			remaining -= clearCount
		}
	}
	if len(a.chunks) > retainCap {
		for i := retainCap; i < len(a.chunks); i++ {
			a.chunks[i] = nil
		}
		a.chunks = a.chunks[:retainCap]
	}
	a.chunkIdx = 0
	a.offset = 0
	a.setActiveChunkAfterReset()
}

func resetBigIntChunks(a *arenaChunks[big.Int], retainCap int) {
	// Keep big.Int word backing arrays across decoder reuse; integer decode
	// overwrites every allocated value before returning it.
	if len(a.chunks) > retainCap {
		for i := retainCap; i < len(a.chunks); i++ {
			a.chunks[i] = nil
		}
		a.chunks = a.chunks[:retainCap]
	}
	a.chunkIdx = 0
	a.offset = 0
	a.setActiveChunkAfterReset()
}

type arenaSlices[S any] struct {
	chunks   [][]S
	chunkIdx int
	offset   int
}

func (a *arenaSlices[S]) alloc(n int) []S {
	if n == 0 {
		return make([]S, 0)
	}

	for a.chunkIdx < len(a.chunks) {
		chunk := a.chunks[a.chunkIdx]
		if chunk == nil {
			a.chunkIdx++
			a.offset = 0
			continue
		}
		avail := len(chunk) - a.offset
		if n <= avail {
			start := a.offset
			a.offset += n
			return chunk[start : start+n : start+n]
		}
		// Not enough space in current chunk, advance to next
		a.chunkIdx++
		a.offset = 0
	}

	prev := 0
	if k := len(a.chunks); k > 0 {
		prev = len(a.chunks[k-1])
	}
	size := nextArenaChunkSize(prev)
	if n > size {
		size = n
	}
	chunk := make([]S, size)
	a.chunks = append(a.chunks, chunk)
	a.chunkIdx = len(a.chunks) - 1
	a.offset = n
	return chunk[:n:n]
}

func (a *arenaSlices[S]) reset(retainCap int) {
	retained := len(a.chunks)
	if retained > retainCap {
		retained = retainCap
	}
	for i := 0; i < retained; i++ {
		chunk := a.chunks[i]
		if chunk == nil {
			continue
		}
		if i < a.chunkIdx {
			clear(chunk)
		} else if i == a.chunkIdx {
			if a.offset > 0 {
				clear(chunk[:a.offset])
			}
			break
		}
	}
	if len(a.chunks) > retainCap {
		for i := retainCap; i < len(a.chunks); i++ {
			a.chunks[i] = nil
		}
		a.chunks = a.chunks[:retainCap]
	}
	a.chunkIdx = 0
	a.offset = 0
}

func resetWordSlices(a *arenaSlices[big.Word], retainCap int) {
	if len(a.chunks) > retainCap {
		for i := retainCap; i < len(a.chunks); i++ {
			a.chunks[i] = nil
		}
		a.chunks = a.chunks[:retainCap]
	}
	a.chunkIdx = 0
	a.offset = 0
}

func resetByteSlices(a *arenaSlices[byte], retainCap int) {
	if len(a.chunks) > retainCap {
		for i := retainCap; i < len(a.chunks); i++ {
			a.chunks[i] = nil
		}
		a.chunks = a.chunks[:retainCap]
	}
	a.chunkIdx = 0
	a.offset = 0
}

type termArena[T Binder] struct {
	vars      arenaChunks[Var[T]]
	delays    arenaChunks[Delay[T]]
	forces    arenaChunks[Force[T]]
	lambdas   arenaChunks[Lambda[T]]
	applies   arenaChunks[Apply[T]]
	constrs   arenaChunks[Constr[T]]
	cases     arenaChunks[Case[T]]
	errors    arenaChunks[Error]
	constants arenaChunks[Constant]
	builtins  arenaChunks[Builtin]
	termLists arenaSlices[Term[T]]
}

func newTermArena[T Binder]() *termArena[T] {
	return &termArena[T]{}
}

// sizeForInput sizes the first chunk of the densest node arenas from the
// FLAT input length. Each divisor is below the smallest nodes-per-byte ratio
// measured on mainnet and benchmark validators (applications 0.28-0.43,
// variables 0.17-0.32, lambdas 0.06-0.22, builtins 0.04-0.13), so the first
// chunk rarely exceeds what the script needs; any shortfall is covered by
// geometric growth. Hints are a fixed fraction of the input length, so a
// script cannot make the decoder reserve more than a constant multiple of
// its own size. Arenas that already hold chunks from a previous decode are
// unaffected.
func (a *termArena[T]) sizeForInput(inputLen int) {
	a.applies.firstSize = inputLen / 4
	a.vars.firstSize = inputLen / 8
	a.lambdas.firstSize = inputLen / 16
	a.builtins.firstSize = inputLen / 32
}

func (a *termArena[T]) reset() {
	a.vars.reset(decodeRetainVarCap)
	a.delays.reset(decodeRetainVarCap)
	a.forces.reset(decodeRetainVarCap)
	a.lambdas.reset(decodeRetainVarCap)
	a.applies.reset(decodeRetainApplyCap)
	a.constrs.reset(decodeRetainVarCap)
	a.cases.reset(decodeRetainVarCap)
	a.errors.reset(decodeRetainVarCap)
	a.constants.reset(decodeRetainVarCap)
	a.builtins.reset(decodeRetainVarCap)
	a.termLists.reset(decodeRetainListCap)
}

func (a *termArena[T]) allocVar(name T) *Var[T] {
	term := a.vars.alloc()
	term.Name = name
	return term
}

func (a *termArena[T]) allocDelay(term Term[T]) *Delay[T] {
	delay := a.delays.alloc()
	delay.Term = term
	return delay
}

func (a *termArena[T]) allocForce(term Term[T]) *Force[T] {
	force := a.forces.alloc()
	force.Term = term
	return force
}

func (a *termArena[T]) allocLambda(name T, body Term[T]) *Lambda[T] {
	lambda := a.lambdas.alloc()
	lambda.ParameterName = name
	lambda.Body = body
	return lambda
}

func (a *termArena[T]) allocApply(function Term[T], argument Term[T]) *Apply[T] {
	apply := a.applies.alloc()
	apply.Function = function
	apply.Argument = argument
	return apply
}

func (a *termArena[T]) allocConstr(tag uint64, fields []Term[T]) *Constr[T] {
	constr := a.constrs.alloc()
	constr.Tag = tag
	constr.Fields = fields
	return constr
}

func (a *termArena[T]) allocCase(constr Term[T], branches []Term[T]) *Case[T] {
	caseTerm := a.cases.alloc()
	caseTerm.Constr = constr
	caseTerm.Branches = branches
	return caseTerm
}

func (a *termArena[T]) allocError() *Error {
	return a.errors.alloc()
}

func (a *termArena[T]) allocConstant(constant IConstant) *Constant {
	term := a.constants.alloc()
	term.Con = constant
	return term
}

func (a *termArena[T]) allocBuiltin(fn builtin.DefaultFunction) *Builtin {
	term := a.builtins.alloc()
	term.DefaultFunction = fn
	return term
}

func (a *termArena[T]) allocTermList(n int) []Term[T] {
	return a.termLists.alloc(n)
}

type constantArena struct {
	bigInts     arenaChunks[big.Int]
	integers    arenaChunks[Integer]
	byteStrings arenaChunks[ByteString]
	strings     arenaChunks[String]
	units       arenaChunks[Unit]
	bools       arenaChunks[Bool]
	protoLists  arenaChunks[ProtoList]
	protoArrays arenaChunks[ProtoArray]
	protoPairs  arenaChunks[ProtoPair]
	// Composite constant types come from the arena so that every decoded
	// constant owns its own TList/TPair wrappers without a heap allocation
	// apiece; TList and TPair have exported writable fields.
	tLists arenaChunks[TList]
	tPairs arenaChunks[TPair]
	values arenaChunks[Value]
	datas  arenaChunks[Data]
	lists  arenaSlices[IConstant]
	bytes  arenaSlices[byte]
	// words backs the magnitude of every decoded integer, so integer
	// decoding allocates from the arena rather than once per big.Int.
	words       arenaSlices[big.Word]
	wordScratch [16]big.Word
	dataDecoder data.Decoder
}

func (a *constantArena) reset() {
	resetBigIntChunks(&a.bigInts, decodeRetainVarCap)
	a.integers.reset(decodeRetainVarCap)
	a.byteStrings.reset(decodeRetainVarCap)
	a.strings.reset(decodeRetainVarCap)
	a.units.reset(1)
	a.bools.reset(1)
	a.protoLists.reset(decodeRetainVarCap)
	a.protoArrays.reset(decodeRetainVarCap)
	a.protoPairs.reset(decodeRetainVarCap)
	a.tLists.reset(decodeRetainVarCap)
	a.tPairs.reset(decodeRetainVarCap)
	a.values.reset(decodeRetainVarCap)
	a.datas.reset(decodeRetainVarCap)
	a.lists.reset(decodeRetainVarCap)
	resetByteSlices(&a.bytes, decodeRetainBytesCap)
	resetWordSlices(&a.words, decodeRetainBytesCap)
	a.dataDecoder.Reset()
}

// listType returns a new list type over elem, from the arena when there is
// one.
func (a *constantArena) listType(elem Typ) Typ {
	if a == nil {
		return &TList{Typ: elem}
	}
	t := a.tLists.alloc()
	t.Typ = elem
	return t
}

// pairType returns a new pair type, from the arena when there is one.
func (a *constantArena) pairType(first, second Typ) Typ {
	if a == nil {
		return &TPair{First: first, Second: second}
	}
	t := a.tPairs.alloc()
	t.First = first
	t.Second = second
	return t
}

func (a *constantArena) allocBigInt() *big.Int {
	return a.bigInts.alloc()
}

func (a *constantArena) allocInteger(inner *big.Int) *Integer {
	integer := a.integers.alloc()
	integer.SetInner(inner)
	return integer
}

func (a *constantArena) allocByteString(inner []byte) *ByteString {
	value := a.byteStrings.alloc()
	value.Inner = inner
	return value
}

func (a *constantArena) allocString(inner string) *String {
	value := a.strings.alloc()
	value.Inner = inner
	return value
}

func (a *constantArena) allocUnit() *Unit {
	return a.units.alloc()
}

func (a *constantArena) allocBool(inner bool) *Bool {
	value := a.bools.alloc()
	value.Inner = inner
	return value
}

func (a *constantArena) allocProtoList(typ Typ, items []IConstant) *ProtoList {
	value := a.protoLists.alloc()
	value.LTyp = typ
	value.List = items
	return value
}

func (a *constantArena) allocProtoArray(typ Typ, items []IConstant) *ProtoArray {
	value := a.protoArrays.alloc()
	value.ATyp = typ
	value.Array = items
	return value
}

func (a *constantArena) allocValue(entries []IConstant) *Value {
	value := a.values.alloc()
	value.Entries = entries
	return value
}

func (a *constantArena) allocProtoPair(
	firstType Typ,
	secondType Typ,
	first IConstant,
	second IConstant,
) *ProtoPair {
	value := a.protoPairs.alloc()
	value.FstType = firstType
	value.SndType = secondType
	value.First = first
	value.Second = second
	return value
}

func (a *constantArena) allocData(inner data.PlutusData) *Data {
	value := a.datas.alloc()
	value.Inner = inner
	return value
}

func (a *constantArena) allocList(n int) []IConstant {
	return a.lists.alloc(n)
}

func (a *constantArena) allocBytes(n int) []byte {
	return a.bytes.alloc(n)
}

func decodeDeBruijnBinder(d *decoder) (DeBruijn, error) {
	i, err := d.word()
	if err != nil {
		return 0, err
	}
	if i > math.MaxInt {
		return 0, fmt.Errorf("DeBruijn index too large: %d", i)
	}
	return DeBruijn(i), nil
}

func decodeVarBinder[T Binder](d *decoder) (T, error) {
	var zero T

	switch any(zero).(type) {
	case DeBruijn:
		i, err := d.word()
		if err != nil {
			return zero, err
		}
		if i > math.MaxInt {
			return zero, fmt.Errorf("DeBruijn index too large: %d", i)
		}
		return any(DeBruijn(i)).(T), nil
	case NamedDeBruijn:
		text, err := d.utf8()
		if err != nil {
			return zero, err
		}
		i, err := d.word()
		if err != nil {
			return zero, err
		}
		if i > math.MaxInt {
			return zero, fmt.Errorf("DeBruijn index too large: %d", i)
		}
		return any(NamedDeBruijn{
			Text:  text,
			Index: DeBruijn(i),
		}).(T), nil
	case Name:
		text, err := d.utf8()
		if err != nil {
			return zero, err
		}
		i, err := d.word()
		if err != nil {
			return zero, err
		}
		return any(Name{
			Text:   text,
			Unique: Unique(i),
		}).(T), nil
	default:
		binder, err := zero.VarDecode(d)
		if err != nil {
			return zero, err
		}
		name, ok := binder.(T)
		if !ok {
			return zero, fmt.Errorf(
				"VarDecode returned wrong type: got %T, want %T",
				binder,
				zero,
			)
		}
		return name, nil
	}
}

func decodeParameterBinder[T Binder](d *decoder) (T, error) {
	var zero T

	switch any(zero).(type) {
	case DeBruijn:
		return any(DeBruijn(0)).(T), nil
	case NamedDeBruijn:
		text, err := d.utf8()
		if err != nil {
			return zero, err
		}
		i, err := d.word()
		if err != nil {
			return zero, err
		}
		if i > math.MaxInt {
			return zero, fmt.Errorf("DeBruijn index too large: %d", i)
		}
		return any(NamedDeBruijn{
			Text:  text,
			Index: DeBruijn(i),
		}).(T), nil
	case Name:
		text, err := d.utf8()
		if err != nil {
			return zero, err
		}
		i, err := d.word()
		if err != nil {
			return zero, err
		}
		return any(Name{
			Text:   text,
			Unique: Unique(i),
		}).(T), nil
	default:
		binder, err := zero.ParameterDecode(d)
		if err != nil {
			return zero, err
		}
		name, ok := binder.(T)
		if !ok {
			return zero, fmt.Errorf(
				"ParameterDecode returned wrong type: got %T, want %T",
				binder,
				zero,
			)
		}
		return name, nil
	}
}

func DecodeConstant(d *decoder) (IConstant, error) {
	var tags constantTagSeq
	err := decodeConstantTags(d, &tags)
	if err != nil {
		return nil, err
	}
	typ, err := decodeConstantType(&tags, nil)
	if err != nil {
		return nil, err
	}
	return decodeConstantValue(d, typ)
}

func decodeConstantWithArena(d *decoder, arena *constantArena) (IConstant, error) {
	var tags constantTagSeq
	err := decodeConstantTags(d, &tags)
	if err != nil {
		return nil, err
	}
	typ, err := decodeConstantType(&tags, arena)
	if err != nil {
		return nil, err
	}
	return decodeConstantValueWithArena(d, typ, arena)
}

func decodeConstantValueWithArena(
	d *decoder,
	typ Typ,
	arena *constantArena,
) (IConstant, error) {
	switch t := typ.(type) {
	case *TInteger:
		return decodeIntegerWithArena(d, arena)
	case *TByteString:
		b, err := d.bytesWithArena(arena)
		if err != nil {
			return nil, err
		}
		return arena.allocByteString(b), nil
	case *TString:
		s, err := d.utf8WithArena(arena)
		if err != nil {
			return nil, err
		}
		return arena.allocString(s), nil
	case *TUnit:
		return arena.allocUnit(), nil
	case *TBool:
		v, err := d.bit()
		if err != nil {
			return nil, err
		}
		return arena.allocBool(v), nil
	case *TList:
		items, err := decodeConstantListWithArena(d, t.Typ, arena)
		if err != nil {
			return nil, err
		}
		return arena.allocProtoList(t.Typ, items), nil
	case *TArray:
		items, err := decodeConstantListWithArena(d, t.Typ, arena)
		if err != nil {
			return nil, err
		}
		return arena.allocProtoArray(t.Typ, items), nil
	case *TPair:
		first, err := decodeConstantValueWithArena(d, t.First, arena)
		if err != nil {
			return nil, err
		}
		second, err := decodeConstantValueWithArena(d, t.Second, arena)
		if err != nil {
			return nil, err
		}
		return arena.allocProtoPair(t.First, t.Second, first, second), nil
	case *TData:
		cborBytes, err := d.bytesWithArena(arena)
		if err != nil {
			return nil, err
		}
		pd, err := arena.dataDecoder.Decode(cborBytes)
		if err != nil {
			return nil, err
		}
		return arena.allocData(pd), nil
	case *TValue:
		entries, err := decodeConstantListWithArena(d, valueEntryType, arena)
		if err != nil {
			return nil, err
		}
		if err := validateValueEntries(entries); err != nil {
			return nil, err
		}
		return arena.allocValue(entries), nil
	case *TBls12_381G1Element:
		return nil, errors.New("cannot decode bls12_381_G1_element constants")
	case *TBls12_381G2Element:
		return nil, errors.New("cannot decode bls12_381_G2_element constants")
	case *TBls12_381MlResult:
		return nil, errors.New("cannot decode bls12_381_mlresult constants")
	default:
		return nil, errors.New("unknown constant constructor")
	}
}

func decodeIntegerWithArena(
	d *decoder,
	arena *constantArena,
) (*Integer, error) {
	small, words, err := d.bigWordSmallInto(arena.wordScratch[:0])
	if err != nil {
		return nil, err
	}

	inner := arena.allocBigInt()
	if words == nil {
		if smallInt, ok := unzigzagUint64(small); ok {
			arena.setInt64(inner, smallInt)
			return arena.allocInteger(inner), nil
		}
		words = append(arena.wordScratch[:0], big.Word(small))
		if bits.UintSize == 32 {
			words = append(words, big.Word(small>>32))
		}
	}
	// One spare word lets unzigzagInPlace's increment carry without
	// reallocating. The three-index slice keeps every integer's magnitude
	// disjoint from its neighbours in the shared arena chunk.
	n := len(words)
	magnitude := arena.words.alloc(n + 1)
	copy(magnitude, words)
	inner.SetBits(magnitude[: n : n+1])
	unzigzagInPlace(inner)
	return arena.allocInteger(inner), nil
}

// setInt64 sets inner to v with its magnitude backed by the arena's word
// slab instead of a per-integer allocation.
func (a *constantArena) setInt64(inner *big.Int, v int64) {
	if v == 0 {
		inner.SetBits(nil)
		return
	}
	abs := uint64(v)
	if v < 0 {
		abs = uint64(-v) // wraps correctly for math.MinInt64
	}
	var magnitude []big.Word
	if bits.UintSize == 32 && abs>>32 != 0 {
		magnitude = a.words.alloc(2)
		magnitude[0] = big.Word(abs)
		magnitude[1] = big.Word(abs >> 32)
	} else {
		magnitude = a.words.alloc(1)
		magnitude[0] = big.Word(abs)
	}
	inner.SetBits(magnitude)
	if v < 0 {
		inner.Neg(inner)
	}
}

func decodeConstantListWithArena(
	d *decoder,
	itemType Typ,
	arena *constantArena,
) ([]IConstant, error) {
	var result []IConstant

	for {
		bit, err := d.bit()
		if err != nil {
			return nil, err
		}
		if !bit {
			if result == nil {
				return emptyConstantList, nil
			}
			return result, nil
		}
		if len(result) >= maxCollectionWidth {
			return nil, errTooMany("constant list items", maxCollectionWidth)
		}
		if err := d.node(); err != nil {
			return nil, err
		}
		if result == nil {
			result = arena.allocList(4)[:0]
		}
		item, err := decodeConstantValueWithArena(d, itemType, arena)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
}

func decodeConstantValue(d *decoder, typ Typ) (IConstant, error) {
	var constant IConstant

	switch t := typ.(type) {
	// Integer
	case *TInteger:
		i, err := d.integer()
		if err != nil {
			return nil, err
		}

		constant = newInteger(i)

	// ByteString
	case *TByteString:
		b, err := d.bytes()
		if err != nil {
			return nil, err
		}

		constant = &ByteString{b}

	// String
	case *TString:
		s, err := d.utf8()
		if err != nil {
			return nil, err
		}

		constant = &String{s}

	// Unit
	case *TUnit:
		constant = &Unit{}

	// Bool
	case *TBool:
		v, err := d.bit()
		if err != nil {
			return nil, err
		}

		constant = &Bool{v}

	// ProtoList
	case *TList:
		items, err := DecodeList(d, func(d *decoder) (IConstant, error) { return decodeConstantValue(d, t.Typ) })
		if err != nil {
			return nil, err
		}
		constant = &ProtoList{
			LTyp: t.Typ,
			List: items,
		}

	case *TArray:
		items, err := DecodeList(d, func(d *decoder) (IConstant, error) {
			return decodeConstantValue(d, t.Typ)
		})
		if err != nil {
			return nil, err
		}
		constant = &ProtoArray{ATyp: t.Typ, Array: items}

	// ProtoPair
	case *TPair:
		first, err := decodeConstantValue(d, t.First)
		if err != nil {
			return nil, err
		}
		second, err := decodeConstantValue(d, t.Second)
		if err != nil {
			return nil, err
		}
		constant = &ProtoPair{
			FstType: t.First,
			SndType: t.Second,
			First:   first,
			Second:  second,
		}

	// Data
	case *TData:
		cborBytes, err := d.bytes()
		if err != nil {
			return nil, err
		}

		pd, err := data.Decode(cborBytes)
		if err != nil {
			return nil, err
		}

		constant = &Data{pd}

	case *TValue:
		entries, err := DecodeList(d, func(d *decoder) (IConstant, error) {
			return decodeConstantValue(d, valueEntryType)
		})
		if err != nil {
			return nil, err
		}
		if err := validateValueEntries(entries); err != nil {
			return nil, err
		}
		constant = &Value{Entries: entries}

	case *TBls12_381G1Element:
		return nil, errors.New("cannot decode bls12_381_G1_element constants")
	case *TBls12_381G2Element:
		return nil, errors.New("cannot decode bls12_381_G2_element constants")
	case *TBls12_381MlResult:
		return nil, errors.New("cannot decode bls12_381_mlresult constants")

	default:
		return nil, errors.New("unknown constant constructor")
	}

	return constant, nil
}

// validateValueEntries checks the shape of Value entries and then applies the
// canonical Value rules shared with the data package.
func validateValueEntries(entries []IConstant) error {
	var prevPolicy []byte
	for policyIndex, entry := range entries {
		policy, ok := entry.(*ProtoPair)
		if !ok || policy == nil {
			return fmt.Errorf("value policy entry %d is not a pair", policyIndex)
		}
		policyID, ok := policy.First.(*ByteString)
		if !ok || policyID == nil {
			return fmt.Errorf("value policy entry %d has a non-bytestring key", policyIndex)
		}
		if err := data.CheckValueKey("policy", policyIndex, policyID.Inner, prevPolicy); err != nil {
			return err
		}
		prevPolicy = policyID.Inner
		tokens, ok := policy.Second.(*ProtoList)
		if !ok || tokens == nil {
			return fmt.Errorf("value policy entry %d has a non-list payload", policyIndex)
		}
		if err := data.CheckValueTokenCount(policyIndex, len(tokens.List)); err != nil {
			return err
		}
		var prevToken []byte
		for tokenIndex, entry := range tokens.List {
			token, ok := entry.(*ProtoPair)
			if !ok || token == nil {
				return fmt.Errorf("value token entry %d:%d is not a pair", policyIndex, tokenIndex)
			}
			tokenID, ok := token.First.(*ByteString)
			if !ok || tokenID == nil {
				return fmt.Errorf("value token entry %d:%d has a non-bytestring key", policyIndex, tokenIndex)
			}
			if err := data.CheckValueKey("token", tokenIndex, tokenID.Inner, prevToken); err != nil {
				return err
			}
			prevToken = tokenID.Inner
			quantity, ok := token.Second.(*Integer)
			if !ok || quantity == nil || quantity.Inner == nil {
				return fmt.Errorf("value token entry %d:%d has a non-integer quantity", policyIndex, tokenIndex)
			}
			if err := data.CheckValueQuantity(policyIndex, tokenIndex, quantity.Inner); err != nil {
				return err
			}
		}
	}
	return nil
}

var (
	cachedTInteger    Typ = &TInteger{}
	cachedTByteString Typ = &TByteString{}
	cachedTString     Typ = &TString{}
	cachedTUnit       Typ = &TUnit{}
	cachedTBool       Typ = &TBool{}
	cachedTData       Typ = &TData{}
	cachedTBlsG1      Typ = &TBls12_381G1Element{}
	cachedTBlsG2      Typ = &TBls12_381G2Element{}
	cachedTBlsMl      Typ = &TBls12_381MlResult{}
	cachedTValue      Typ = &TValue{}
)

var (
	valueTokenType = &TPair{First: &TByteString{}, Second: &TInteger{}}
	valueEntryType = &TPair{
		First:  &TByteString{},
		Second: &TList{Typ: valueTokenType},
	}
)

type constantTagSeq struct {
	small [8]byte
	extra []byte
	n     int
}

func (s *constantTagSeq) append(tag byte) {
	if s.n < len(s.small) {
		s.small[s.n] = tag
	} else {
		s.extra = append(s.extra, tag)
	}
	s.n++
}

func (s *constantTagSeq) at(idx int) byte {
	if idx < len(s.small) {
		return s.small[idx]
	}
	return s.extra[idx-len(s.small)]
}

func (s *constantTagSeq) len() int {
	return s.n
}

func decodeConstantType(tags *constantTagSeq, arena *constantArena) (Typ, error) {
	typ, next, err := decodeConstantTypeAt(tags, 0, arena)
	if err != nil {
		return nil, err
	}
	if next != tags.len() {
		return nil, errors.New("unknown type tag")
	}
	return typ, nil
}

func decodeConstantTypeAt(
	tags *constantTagSeq,
	idx int,
	arena *constantArena,
) (Typ, int, error) {
	if idx >= tags.len() {
		return nil, idx, errors.New("unknown type tag")
	}

	next := tags.at(idx)
	idx++

	switch next {
	case IntegerTag:
		return cachedTInteger, idx, nil
	case ByteStringTag:
		return cachedTByteString, idx, nil
	case StringTag:
		return cachedTString, idx, nil
	case UnitTag:
		return cachedTUnit, idx, nil
	case BoolTag:
		return cachedTBool, idx, nil
	case DataTag:
		return cachedTData, idx, nil
	case Bls12_381G1Tag:
		return cachedTBlsG1, idx, nil
	case Bls12_381G2Tag:
		return cachedTBlsG2, idx, nil
	case Bls12_381MlTag:
		return cachedTBlsMl, idx, nil
	case ValueTag:
		return cachedTValue, idx, nil
	// NOTE: this also covers ProtoPairOneTag, but it's the same value as ProtoListOneTag.
	case ProtoListOneTag:
		if idx >= tags.len() {
			return nil, idx, errors.New("unknown type tag")
		}
		switch tags.at(idx) {
		case ProtoListTwoTag:
			subType, next, err := decodeConstantTypeAt(tags, idx+1, arena)
			if err != nil {
				return nil, next, err
			}
			return arena.listType(subType), next, nil
		case ProtoArrayTag:
			subType, next, err := decodeConstantTypeAt(tags, idx+1, arena)
			if err != nil {
				return nil, next, err
			}
			return &TArray{Typ: subType}, next, nil
		case ProtoPairTwoTag:
			idx++
			if idx >= tags.len() || tags.at(idx) != ProtoPairThreeTag {
				return nil, idx, errors.New("unknown type tag")
			}
			first, next, err := decodeConstantTypeAt(tags, idx+1, arena)
			if err != nil {
				return nil, next, err
			}
			second, next, err := decodeConstantTypeAt(tags, next, arena)
			if err != nil {
				return nil, next, err
			}
			return arena.pairType(first, second), next, nil
		default:
			return nil, idx, errors.New("unknown type tag")
		}
	default:
		return nil, idx, errors.New("unknown type tag")
	}
}

func decodeConstantTags(d *decoder, tags *constantTagSeq) error {
	for {
		bit, err := d.bit()
		if err != nil {
			return err
		}
		if !bit {
			return nil
		}
		// Each nested list/pair constant type adds at least one tag, so the
		// tag count bounds the recursion depth of both decodeConstantTypeAt and
		// decodeConstantValue. Cap it to prevent stack-overflow on a
		// pathologically nested constant type.
		if tags.len() >= maxFlatDecodeDepth {
			return errors.New("constant type nesting too deep")
		}
		tag, err := d.bits4()
		if err != nil {
			return err
		}
		tags.append(tag)
	}
}

// Decode a list of items with a decoder function.
// This is byte alignment agnostic.
// Decode a bit from the buffer.
// If 0 then stop.
// Otherwise we decode an item in the list with the decoder function passed
// in. Then decode the next bit in the buffer and repeat above.
// Returns a list of items decoded with the decoder function.
func DecodeList[T any](
	d *decoder,
	decoderFunc func(*decoder) (T, error),
) ([]T, error) {
	result := make([]T, 0, 4)

	for {
		bit, err := d.bit()
		if err != nil {
			return nil, err
		}
		if !bit {
			break
		}
		if len(result) >= maxCollectionWidth {
			return nil, errTooMany("constant list items", maxCollectionWidth)
		}
		if err := d.node(); err != nil {
			return nil, err
		}
		item, err := decoderFunc(d)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, nil
}

// maxFlatDecodeDepth bounds the nesting depth of decoded terms (and, via the
// constant type-tag cap below, of decoded constants). The flat term decoder is
// recursive, so without this bound a small attacker-controlled program of
// deeply nested terms (e.g. a long chain of Delay tags, ~2 per input byte)
// drives the Go call stack to a fatal, unrecoverable stack overflow. The limit
// is chosen to be far above any realistic on-chain script's nesting while
// staying comfortably below the stack-overflow point on both 32-bit and 64-bit
// targets. It is a safety backstop and should be kept in sync with the
// protocol's script-size limits.
const maxFlatDecodeDepth = 100_000

type decoder struct {
	buffer   []byte
	usedBits int64
	pos      int
	depth    int
	// nodes counts the terms and constant list items decoded so far.
	nodes int
	// maxConstrFields is the widest constr field list accepted.
	maxConstrFields int
}

func newDecoder(bytes []byte) *decoder {
	d := &decoder{}
	d.reset(bytes)
	return d
}

func (d *decoder) reset(bytes []byte) {
	*d = decoder{buffer: bytes, maxConstrFields: maxCollectionWidth}
}

// node charges one term or constant list item against the program's node
// budget.
func (d *decoder) node() error {
	d.nodes++
	if d.nodes > maxProgramNodes {
		return errors.New("program has too many nodes")
	}
	return nil
}

// enter records descent into a nested term and fails if the nesting limit is
// exceeded. Each successful enter must be paired with a leave (typically via
// defer) so sibling terms do not accumulate depth.
func (d *decoder) enter() error {
	if d.depth >= maxFlatDecodeDepth {
		return errors.New("term nesting too deep")
	}
	d.depth++
	return nil
}

func (d *decoder) leave() {
	d.depth--
}

// Decodes a filler of max one byte size.
// Decodes bits until we hit a bit that is 1.
// Expects that the 1 is at the end of the current byte in the buffer.
func (d *decoder) filler() error {
	for {
		ok, err := d.zero()
		if err != nil {
			return err
		}

		if !ok {
			break
		}
	}

	return nil
}

// ensureFullyConsumed verifies the decoder consumed the entire input buffer.
// A valid flat-encoded program ends with a byte-aligned filler, so after the
// final filler the decoder must sit exactly at the end of the buffer with no
// partially consumed byte. Any leftover bytes (or stray bits from a filler
// whose terminating 1-bit was not byte-aligned) are trailing garbage and the
// input is rejected.
func (d *decoder) ensureFullyConsumed() error {
	if d.usedBits != 0 || d.pos != len(d.buffer) {
		return fmt.Errorf(
			"trailing bytes after program: %d byte(s) not consumed",
			len(d.buffer)-d.pos,
		)
	}
	return nil
}

// Decode the next bit in the buffer.
// If the bit was 0 then return true.
// Otherwise return false.
// Throws EndOfBuffer error if used at the end of the array.
func (d *decoder) zero() (bool, error) {
	currentBit, err := d.bit()
	if err != nil {
		return false, err
	}

	return !currentBit, nil
}

// Decode the next bit in the buffer.
// If the bit was 1 then return true.
// Otherwise return false.
// Throws EndOfBuffer error if used at the end of the array.
func (d *decoder) bit() (bool, error) {
	if d.pos >= len(d.buffer) {
		return false, errors.New("end of buffer")
	}

	b := d.buffer[d.pos]&(128>>d.usedBits) > 0

	d.incrementBufferByBit()

	return b, nil
}

// Decode a word of any size.
// This is byte alignment agnostic.
// First we decode the next 8 bits of the buffer.
// We take the 7 least significant bits as the 7 least significant bits of
// the current unsigned integer. If the most significant bit of the 8
// bits is 1 then we take the next 8 and repeat the process above,
// filling in the next 7 least significant bits of the unsigned integer and
// so on. If the most significant bit was instead 0 we stop decoding
// any more bits.
func (d *decoder) word() (uint, error) {
	word, err := d.word64()
	if err != nil {
		return 0, err
	}
	if word > math.MaxUint {
		return 0, errors.New("varint overflows machine word")
	}
	return uint(word), nil
}

func (d *decoder) word64() (uint64, error) {
	var finalWord uint64
	shl := 0

	if d.usedBits == 0 {
		if d.pos >= len(d.buffer) {
			return 0, errors.New("end of buffer")
		}
		word8 := d.buffer[d.pos]
		if word8&128 == 0 {
			d.pos++
			return uint64(word8), nil
		}
		if d.pos+1 < len(d.buffer) {
			nextWord8 := d.buffer[d.pos+1]
			if nextWord8&128 == 0 {
				d.pos += 2
				return uint64(word8&127) | uint64(nextWord8)<<7, nil
			}
		}
		for {
			if d.pos >= len(d.buffer) {
				return 0, errors.New("end of buffer")
			}
			word8 := d.buffer[d.pos]
			d.pos++

			word7 := uint64(word8 & 127)
			if err := checkWord64Shift(word7, shl); err != nil {
				return 0, err
			}
			finalWord |= word7 << shl
			shl += 7
			if word8&128 == 0 {
				return finalWord, nil
			}
		}
	}

	word8, err := d.bits8(8)
	if err != nil {
		return 0, err
	}
	word7 := uint64(word8 & 127)
	if word8&128 == 0 {
		return word7, nil
	}
	finalWord = word7
	shl = 7

	for {
		word8, err := d.bits8(8)
		if err != nil {
			return 0, err
		}

		word7 := uint64(word8 & 127)
		if err := checkWord64Shift(word7, shl); err != nil {
			return 0, err
		}
		finalWord |= word7 << shl

		shl += 7

		leadingBit := word8 & 128

		if leadingBit == 0 {
			break
		}
	}

	return finalWord, nil
}

// checkWord64Shift reports an overflow error if placing the 7-bit group word7
// at bit offset shl would discard any set bit beyond 64 bits. Go defines shifts
// of >= the integer width as yielding 0, so without this guard a varint with too
// many continuation bytes would be silently truncated rather than rejected.
func checkWord64Shift(word7 uint64, shl int) error {
	if word7 == 0 {
		return nil
	}
	if shl >= 64 || word7>>(64-shl) != 0 {
		return errors.New("varint overflows uint64")
	}
	return nil
}

// Decode up to 8 bits.
// This is byte alignment agnostic.
// If num_bits is greater than the 8 we throw an IncorrectNumBits error.
// First we decode the next num_bits of bits in the buffer.
// If there are less unused bits in the current byte in the buffer than
// num_bits, then we decode the remaining bits from the most
// significant bits in the next byte in the buffer. Otherwise we decode
// the unused bits from the current byte. Returns the decoded value up
// to a byte in size.
func (d *decoder) bits8(numBits byte) (byte, error) {
	if numBits > 8 {
		return 0, errors.New("IncorrectNumBits")
	}
	if d.pos >= len(d.buffer) {
		return 0, errors.New("end of buffer")
	}
	if numBits == 8 {
		if d.usedBits == 0 {
			x := d.buffer[d.pos]
			d.pos++
			return x, nil
		}
		if d.pos+1 >= len(d.buffer) {
			return 0, fmt.Errorf("NotEnoughBits(%d)", numBits)
		}
		x := (d.buffer[d.pos] << byte(d.usedBits)) | (d.buffer[d.pos+1] >> (8 - byte(d.usedBits)))
		d.pos++
		return x, nil
	}

	remainingBits := (len(d.buffer)-d.pos)*8 - int(d.usedBits)
	if int(numBits) > remainingBits {
		return 0, fmt.Errorf("NotEnoughBits(%d)", numBits)
	}

	unusedBits := 8 - d.usedBits
	leadingZeros := 8 - numBits

	r := (d.buffer[d.pos] << byte(d.usedBits)) >> leadingZeros

	var x byte

	if numBits > byte(unusedBits) {
		x = r | (d.buffer[d.pos+1] >> (byte(unusedBits) + leadingZeros))
	} else {
		x = r
	}

	d.dropBits(uint(numBits))

	return x, nil
}

func (d *decoder) bits4() (byte, error) {
	if d.pos >= len(d.buffer) {
		return 0, errors.New("end of buffer")
	}

	b0 := d.buffer[d.pos]
	switch d.usedBits {
	case 0:
		d.usedBits = 4
		return b0 >> 4, nil
	case 4:
		d.usedBits = 0
		d.pos++
		return b0 & 0x0f, nil
	}

	unusedBits := 8 - d.usedBits
	if unusedBits < 4 && d.pos+1 >= len(d.buffer) {
		return 0, fmt.Errorf("NotEnoughBits(%d)", 4)
	}

	x := (b0 << byte(d.usedBits)) >> 4
	if unusedBits < 4 {
		x |= d.buffer[d.pos+1] >> (unusedBits + 4)
	}

	allUsedBits := d.usedBits + 4
	d.usedBits = allUsedBits % 8
	d.pos += int(allUsedBits / 8)
	return x, nil
}

func (d *decoder) bits7() (byte, error) {
	if d.pos >= len(d.buffer) {
		return 0, errors.New("end of buffer")
	}

	b0 := d.buffer[d.pos]
	if d.usedBits == 0 {
		d.usedBits = 7
		return b0 >> 1, nil
	}

	unusedBits := 8 - d.usedBits
	if unusedBits < 7 && d.pos+1 >= len(d.buffer) {
		return 0, fmt.Errorf("NotEnoughBits(%d)", 7)
	}
	x := (b0 << byte(d.usedBits)) >> 1
	if unusedBits < 7 {
		x |= d.buffer[d.pos+1] >> (unusedBits + 1)
	}

	allUsedBits := d.usedBits + 7
	d.usedBits = allUsedBits % 8
	d.pos += int(allUsedBits / 8)
	return x, nil
}

func (d *decoder) dropBits(numBits uint) {
	allUsedBits := int64(numBits) + d.usedBits //nolint:gosec

	d.usedBits = allUsedBits % 8

	d.pos += int(allUsedBits / 8)
}

// Decode a string.
// Convert to byte array and then use byte array decoding.
// Decodes a filler to byte align the buffer,
// then decodes the next byte to get the array length up to a max of 255.
// We decode bytes equal to the array length to form the byte array.
// If the following byte for array length is not 0 we decode it and repeat
// above to continue decoding the byte array. We stop once we hit a
// byte array length of 0. If array length is 0 for first byte array
// length the we return a empty array.
func (d *decoder) utf8() (string, error) {
	b, err := d.bytes()
	if err != nil {
		return "", err
	}

	if !utf8.Valid(b) {
		return "", fmt.Errorf("bytes are not valid utf8 %v", b)
	}

	return string(b), nil
}

func (d *decoder) utf8WithArena(arena *constantArena) (string, error) {
	b, err := d.bytesWithArena(arena)
	if err != nil {
		return "", err
	}

	if !utf8.Valid(b) {
		return "", fmt.Errorf("bytes are not valid utf8 %v", b)
	}

	return string(b), nil
}

// Decode a byte array.
// Decodes a filler to byte align the buffer,
// then decodes the next byte to get the array length up to a max of 255.
// We decode bytes equal to the array length to form the byte array.
// If the following byte for array length is not 0 we decode it and repeat
// above to continue decoding the byte array. We stop once we hit a
// byte array length of 0. If array length is 0 for first byte array
// length the we return a empty array.
func (d *decoder) bytes() ([]byte, error) {
	if err := d.filler(); err != nil {
		return nil, err
	}

	return d.byteArray()
}

func (d *decoder) bytesWithArena(arena *constantArena) ([]byte, error) {
	if err := d.filler(); err != nil {
		return nil, err
	}

	return d.byteArrayWithArena(arena)
}

// Decode a byte array.
// Throws a BufferNotByteAligned error if the buffer is not byte aligned
// Decodes the next byte to get the array length up to a max of 255.
// We decode bytes equal to the array length to form the byte array.
// If the following byte for array length is not 0 we decode it and repeat
// above to continue decoding the byte array. We stop once we hit a
// byte array length of 0. If array length is 0 for first byte array
// length the we return a empty array.
func (d *decoder) byteArray() ([]byte, error) {
	if d.usedBits != 0 {
		return nil, errors.New("buffer not byte aligned")
	}

	if err := d.ensureBytes(1); err != nil {
		return nil, err
	}

	blkLen := int(d.buffer[d.pos])
	d.pos++
	result := make([]byte, 0, blkLen)
	for blkLen != 0 {
		if err := d.ensureBytes(blkLen + 1); err != nil {
			return nil, err
		}

		decodedArray := d.buffer[d.pos : d.pos+blkLen]
		result = append(result, decodedArray...)

		d.pos += blkLen
		blkLen = int(d.buffer[d.pos])
		d.pos++
	}

	return result, nil
}

func (d *decoder) byteArrayWithArena(arena *constantArena) ([]byte, error) {
	if d.usedBits != 0 {
		return nil, errors.New("buffer not byte aligned")
	}

	if err := d.ensureBytes(1); err != nil {
		return nil, err
	}

	scan := d.pos
	total := 0
	for {
		blkLen := int(d.buffer[scan])
		scan++
		if blkLen == 0 {
			break
		}
		if blkLen+1 > len(d.buffer)-scan {
			return nil, fmt.Errorf("NotEnoughBytes(%d)", blkLen+1)
		}
		if blkLen > math.MaxInt-total {
			return nil, errors.New("byte array too large")
		}
		total += blkLen
		scan += blkLen
	}

	if total == 0 {
		d.pos = scan
		return emptyByteArray, nil
	}

	result := arena.allocBytes(total)
	offset := 0
	for {
		blkLen := int(d.buffer[d.pos])
		d.pos++
		if blkLen == 0 {
			return result, nil
		}
		copy(result[offset:], d.buffer[d.pos:d.pos+blkLen])
		offset += blkLen
		d.pos += blkLen
	}
}

// integer decodes a variable-length signed integer from the buffer.
// It is byte-alignment agnostic. Reads 8 bits at a time, using the 7 least
// significant bits for the unsigned integer, continuing if the MSB is 1,
// stopping if 0, then applies zigzag decoding to get the signed integer.
func (d *decoder) integer() (*big.Int, error) {
	small, word, err := d.bigWordSmall()
	if err != nil {
		return nil, err
	}

	if word == nil {
		if smallInt, ok := unzigzagUint64(small); ok {
			return big.NewInt(smallInt), nil
		}
		word = new(big.Int).SetUint64(small)
	}

	return unzigzag(word), nil
}

func (d *decoder) bigWordSmall() (uint64, *big.Int, error) {
	small, words, err := d.bigWordSmallInto(nil)
	if err != nil || words == nil {
		return small, nil, err
	}
	return small, new(big.Int).SetBits(words), nil
}

// bigWordSmallInto decodes a base-128 natural. A value that fits the uint64
// fast path is returned as small with nil words; a larger one is returned as
// little-endian words accumulated into scratch, which the caller must copy
// before reusing scratch.
func (d *decoder) bigWordSmallInto(scratch []big.Word) (uint64, []big.Word, error) {
	accumulator := bigWordAccumulator{scratch: scratch}

	if d.usedBits == 0 {
		for {
			if d.pos >= len(d.buffer) {
				return 0, nil, errors.New("end of buffer")
			}
			word8 := d.buffer[d.pos]
			d.pos++

			accumulator.add(uint64(word8 & 0x7F))
			if word8&0x80 == 0 {
				small, word := accumulator.value()
				return small, word, nil
			}
		}
	}

	for {
		word8, err := d.bits8(8)
		if err != nil {
			return 0, nil, err
		}

		accumulator.add(uint64(word8 & 0x7F))
		if word8&0x80 == 0 {
			small, word := accumulator.value()
			return small, word, nil
		}
	}
}

// bigWordAccumulator packs little-endian base-128 groups directly into the
// native-word representation consumed by big.Int.SetBits. Each input group is
// written once, avoiding repeated operations over the growing integer.
type bigWordAccumulator struct {
	small   uint64
	words   []big.Word
	scratch []big.Word
	group   int
}

func (a *bigWordAccumulator) add(chunk uint64) {
	if a.words == nil && canAccumulateUint64(chunk, a.group) {
		if chunk != 0 {
			a.small |= chunk << uint(a.group*7)
		}
		a.group++
		return
	}

	if a.words == nil {
		a.words = append(a.scratch[:0], big.Word(a.small))
		if bits.UintSize == 32 {
			a.words = append(a.words, big.Word(a.small>>32))
		}
	}
	if chunk != 0 {
		a.words = accumulateBigWordChunk(a.words, a.group, chunk)
	}
	a.group++
}

func (a *bigWordAccumulator) value() (uint64, []big.Word) {
	return a.small, a.words
}

func accumulateBigWordChunk(
	words []big.Word,
	group int,
	chunk uint64,
) []big.Word {
	groupRemainder := group % bits.UintSize
	bitInBlock := groupRemainder * 7
	wordIndex := (group/bits.UintSize)*7 + bitInBlock/bits.UintSize
	shift := uint(bitInBlock % bits.UintSize)
	crossesWord := int(shift)+7 > bits.UintSize
	requiredWords := wordIndex + 1
	if crossesWord {
		requiredWords++
	}
	for len(words) < requiredWords {
		words = append(words, 0)
	}

	wordChunk := big.Word(chunk)
	words[wordIndex] |= wordChunk << shift
	if crossesWord {
		words[wordIndex+1] |= wordChunk >> uint(bits.UintSize-int(shift))
	}
	return words
}

func canAccumulateUint64(chunk uint64, group int) bool {
	if chunk == 0 {
		return true
	}
	if group >= 10 {
		return false
	}
	shift := uint(group * 7)
	return chunk <= (math.MaxUint64 >> shift)
}

func unzigzagUint64(n uint64) (int64, bool) {
	shifted := n >> 1
	if shifted > math.MaxInt64 {
		return 0, false
	}
	if n&1 == 0 {
		return int64(shifted), true
	}
	return -1 - int64(shifted), true
}

func unzigzagInPlace(n *big.Int) *big.Int {
	if n.Bit(0) == 0 {
		return n.Rsh(n, 1)
	}
	n.Rsh(n, 1)
	n.Add(n, bigOne)
	return n.Neg(n)
}

// Increment used bits by 1.
// If all 8 bits are used then increment buffer position by 1.
func (d *decoder) incrementBufferByBit() {
	if d.usedBits == 7 {
		d.pos += 1

		d.usedBits = 0
	} else {
		d.usedBits += 1
	}
}

// Ensures the buffer has the required bytes passed in by required_bytes.
// Throws a NotEnoughBytes error if there are less bytes remaining in the
// buffer than required_bytes.
func (d *decoder) ensureBytes(requiredBytes int) error {
	if requiredBytes > len(d.buffer)-d.pos {
		return fmt.Errorf("NotEnoughBytes(%d)", requiredBytes)
	} else {
		return nil
	}
}
