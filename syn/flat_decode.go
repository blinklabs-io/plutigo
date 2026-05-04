package syn

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"unicode/utf8"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
)

const (
	decodeTermChunkSize  = 384
	decodeRetainVarCap   = 16
	decodeRetainApplyCap = 16
	decodeRetainListCap  = 32
	decodeRetainBytesCap = 8
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
	return decodeDeBruijnProgram(d, arena, consts)
}

func (d *DeBruijnDecoder) Decode(bytes []byte) (*Program[DeBruijn], error) {
	d.decoder.reset(bytes)
	d.arena.reset()
	d.consts.reset()
	return decodeDeBruijnProgram(&d.decoder, &d.arena, &d.consts)
}

func Decode[T Binder](bytes []byte) (*Program[T], error) {
	var zero T
	switch any(zero).(type) {
	case DeBruijn:
		program, err := decodeDeBruijn(bytes)
		if err != nil {
			return nil, err
		}
		return any(program).(*Program[T]), nil
	}

	d := newDecoder(bytes)
	arena := newTermArena[T]()

	major, err := d.word()
	if err != nil {
		return nil, err
	}

	minor, err := d.word()
	if err != nil {
		return nil, err
	}

	patch, err := d.word()
	if err != nil {
		return nil, err
	}

	terms, err := decodeTermWithArena[T](d, arena)
	if err != nil {
		return nil, err
	}

	if major > math.MaxUint32 || minor > math.MaxUint32 ||
		patch > math.MaxUint32 {
		return nil, errors.New("version numbers too large")
	}

	version := lang.LanguageVersion{uint32(major), uint32(minor), uint32(patch)}

	program := &Program[T]{
		Version: version,
		Term:    terms,
	}

	if err := d.filler(); err != nil {
		return nil, err
	}

	return program, nil
}

func decodeDeBruijnProgram(
	d *decoder,
	arena *termArena[DeBruijn],
	consts *constantArena,
) (*Program[DeBruijn], error) {
	major, err := d.word()
	if err != nil {
		return nil, err
	}

	minor, err := d.word()
	if err != nil {
		return nil, err
	}

	patch, err := d.word()
	if err != nil {
		return nil, err
	}

	terms, err := decodeTermDeBruijnWithArena(d, arena, consts)
	if err != nil {
		return nil, err
	}

	if major > math.MaxUint32 || minor > math.MaxUint32 ||
		patch > math.MaxUint32 {
		return nil, errors.New("version numbers too large")
	}

	program := &Program[DeBruijn]{
		Version: lang.LanguageVersion{uint32(major), uint32(minor), uint32(patch)},
		Term:    terms,
	}

	if err := d.filler(); err != nil {
		return nil, err
	}

	return program, nil
}

func DecodeTerm[T Binder](d *decoder) (Term[T], error) {
	return decodeTermWithArena(d, newTermArena[T]())
}

func decodeTermDeBruijnWithArena(
	d *decoder,
	arena *termArena[DeBruijn],
	consts *constantArena,
) (Term[DeBruijn], error) {
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
		t, err := decodeTermDeBruijnWithArena(d, arena, consts)
		if err != nil {
			return nil, err
		}
		return arena.allocDelay(t), nil
	case LambdaTag:
		t, err := decodeTermDeBruijnWithArena(d, arena, consts)
		if err != nil {
			return nil, err
		}
		return arena.allocLambda(DeBruijn(0), t), nil
	case ApplyTag:
		function, err := decodeTermDeBruijnWithArena(d, arena, consts)
		if err != nil {
			return nil, err
		}
		argument, err := decodeTermDeBruijnWithArena(d, arena, consts)
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
		t, err := decodeTermDeBruijnWithArena(d, arena, consts)
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
		constrTag, err := d.word()
		if err != nil {
			return nil, err
		}
		fields, err := decodeTermListDeBruijnWithArena(d, arena, consts)
		if err != nil {
			return nil, err
		}
		return arena.allocConstr(constrTag, fields), nil
	case CaseTag:
		constr, err := decodeTermDeBruijnWithArena(d, arena, consts)
		if err != nil {
			return nil, err
		}
		branches, err := decodeTermListDeBruijnWithArena(d, arena, consts)
		if err != nil {
			return nil, err
		}
		return arena.allocCase(constr, branches), nil
	default:
		return nil, fmt.Errorf("invalid term tag: %d", tag)
	}
}

func decodeTermWithArena[T Binder](d *decoder, arena *termArena[T]) (Term[T], error) {
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
		constrTag, err := d.word()
		if err != nil {
			return nil, err
		}

		fields, err := decodeTermListWithArena(d, arena)
		if err != nil {
			return nil, err
		}

		term = arena.allocConstr(constrTag, fields)
	case CaseTag:
		constr, err := decodeTermWithArena[T](d, arena)
		if err != nil {
			return nil, err
		}

		branches, err := decodeTermListWithArena(d, arena)
		if err != nil {
			return nil, err
		}

		term = arena.allocCase(constr, branches)
	default:
		return nil, fmt.Errorf("invalid term tag: %d", tag)
	}

	return term, nil
}

func decodeTermListWithArena[T Binder](d *decoder, arena *termArena[T]) ([]Term[T], error) {
	result := arena.allocTermList(4)[:0]

	for {
		bit, err := d.bit()
		if err != nil {
			return nil, err
		}
		if !bit {
			break
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
		if result == nil {
			result = arena.allocTermList(4)[:0]
		}
		item, err := decodeTermDeBruijnWithArena(d, arena, consts)
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
}

func (a *arenaChunks[S]) used() int {
	if len(a.chunks) == 0 {
		return 0
	}
	return a.chunkIdx*decodeTermChunkSize + a.offset
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

	chunk := make([]S, decodeTermChunkSize)
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
		maxRetained := retained * decodeTermChunkSize
		if remaining > maxRetained {
			remaining = maxRetained
		}
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
		return nil
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

	size := decodeTermChunkSize
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

func (a *termArena[T]) allocConstr(tag uint, fields []Term[T]) *Constr[T] {
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
	protoPairs  arenaChunks[ProtoPair]
	datas       arenaChunks[Data]
	lists       arenaSlices[IConstant]
	bytes       arenaSlices[byte]
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
	a.protoPairs.reset(decodeRetainVarCap)
	a.datas.reset(decodeRetainVarCap)
	a.lists.reset(decodeRetainVarCap)
	resetByteSlices(&a.bytes, decodeRetainBytesCap)
	a.dataDecoder.Reset()
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
	typ, err := decodeConstantType(&tags)
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
	typ, err := decodeConstantType(&tags)
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
	default:
		return nil, errors.New("unknown constant constructor")
	}
}

func decodeIntegerWithArena(
	d *decoder,
	arena *constantArena,
) (*Integer, error) {
	small, word, err := d.bigWordSmall()
	if err != nil {
		return nil, err
	}

	inner := arena.allocBigInt()
	if word == nil {
		if smallInt, ok := unzigzagUint64(small); ok {
			inner.SetInt64(smallInt)
			return arena.allocInteger(inner), nil
		}
		inner.SetUint64(small)
	} else {
		inner.Set(word)
	}
	unzigzagInPlace(inner)
	return arena.allocInteger(inner), nil
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

	default:
		return nil, errors.New("unknown constant constructor")
	}

	return constant, nil
}

var (
	cachedTInteger    Typ = &TInteger{}
	cachedTByteString Typ = &TByteString{}
	cachedTString     Typ = &TString{}
	cachedTUnit       Typ = &TUnit{}
	cachedTBool       Typ = &TBool{}
	cachedTData       Typ = &TData{}
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

func decodeConstantType(tags *constantTagSeq) (Typ, error) {
	typ, next, err := decodeConstantTypeAt(tags, 0)
	if err != nil {
		return nil, err
	}
	if next != tags.len() {
		return nil, errors.New("unknown type tag")
	}
	return typ, nil
}

func decodeConstantTypeAt(tags *constantTagSeq, idx int) (Typ, int, error) {
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
	// NOTE: this also covers ProtoPairOneTag, but it's the same value as ProtoListOneTag.
	case ProtoListOneTag:
		if idx >= tags.len() {
			return nil, idx, errors.New("unknown type tag")
		}
		switch tags.at(idx) {
		case ProtoListTwoTag:
			subType, next, err := decodeConstantTypeAt(tags, idx+1)
			if err != nil {
				return nil, next, err
			}
			return &TList{Typ: subType}, next, nil
		case ProtoPairTwoTag:
			idx++
			if idx >= tags.len() || tags.at(idx) != ProtoPairThreeTag {
				return nil, idx, errors.New("unknown type tag")
			}
			first, next, err := decodeConstantTypeAt(tags, idx+1)
			if err != nil {
				return nil, next, err
			}
			second, next, err := decodeConstantTypeAt(tags, next)
			if err != nil {
				return nil, next, err
			}
			return &TPair{First: first, Second: second}, next, nil
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
		item, err := decoderFunc(d)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, nil
}

type decoder struct {
	buffer   []byte
	usedBits int64
	pos      int
}

func newDecoder(bytes []byte) *decoder {
	return &decoder{
		buffer:   bytes,
		usedBits: 0,
		pos:      0,
	}
}

func (d *decoder) reset(bytes []byte) {
	d.buffer = bytes
	d.usedBits = 0
	d.pos = 0
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
	var finalWord uint
	shl := 0

	if d.usedBits == 0 {
		for {
			if d.pos >= len(d.buffer) {
				return 0, errors.New("end of buffer")
			}
			word8 := d.buffer[d.pos]
			d.pos++

			finalWord |= uint(word8&127) << shl
			shl += 7
			if word8&128 == 0 {
				return finalWord, nil
			}
		}
	}

	for {
		word8, err := d.bits8(8)
		if err != nil {
			return 0, err
		}

		word7 := word8 & 127

		finalWord |= uint(word7) << shl

		shl += 7

		leadingBit := word8 & 128

		if leadingBit == 0 {
			break
		}
	}

	return finalWord, nil
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
	var small uint64
	var bigWord *big.Int
	shift := uint(0)

	if d.usedBits == 0 {
		for {
			if d.pos >= len(d.buffer) {
				return 0, nil, errors.New("end of buffer")
			}
			word8 := d.buffer[d.pos]
			d.pos++

			chunk := uint64(word8 & 0x7F)
			if bigWord == nil {
				if canAccumulateUint64(chunk, shift) {
					small |= chunk << shift
				} else {
					bigWord = new(big.Int).SetUint64(small)
					if chunk != 0 {
						part := new(big.Int).SetUint64(chunk)
						part.Lsh(part, shift)
						bigWord.Or(bigWord, part)
					}
				}
			} else if chunk != 0 {
				part := new(big.Int).SetUint64(chunk)
				part.Lsh(part, shift)
				bigWord.Or(bigWord, part)
			}

			shift += 7
			if word8&0x80 == 0 {
				return small, bigWord, nil
			}
		}
	}

	for {
		word8, err := d.bits8(8)
		if err != nil {
			return 0, nil, err
		}

		chunk := uint64(word8 & 0x7F)
		if bigWord == nil {
			if canAccumulateUint64(chunk, shift) {
				small |= chunk << shift
			} else {
				bigWord = new(big.Int).SetUint64(small)
				if chunk != 0 {
					part := new(big.Int).SetUint64(chunk)
					part.Lsh(part, shift)
					bigWord.Or(bigWord, part)
				}
			}
		} else if chunk != 0 {
			part := new(big.Int).SetUint64(chunk)
			part.Lsh(part, shift)
			bigWord.Or(bigWord, part)
		}

		shift += 7
		if word8&0x80 == 0 {
			return small, bigWord, nil
		}
	}
}

func canAccumulateUint64(chunk uint64, shift uint) bool {
	if chunk == 0 {
		return true
	}
	if shift >= 64 {
		return false
	}
	return chunk <= (math.MaxUint64 >> shift)
}

// bigWord decodes a variable-length unsigned integer from the buffer.
// It is byte-alignment agnostic. Reads 8 bits at a time, using the 7 least
// significant bits for the integer, continuing if the MSB is 1, stopping if 0.
func (d *decoder) bigWord() (*big.Int, error) {
	finalWord := new(big.Int)
	shift := uint(0)

	if d.usedBits == 0 {
		for {
			if d.pos >= len(d.buffer) {
				return nil, errors.New("end of buffer")
			}
			word8 := d.buffer[d.pos]
			d.pos++

			finalWord.Or(finalWord, new(big.Int).Lsh(new(big.Int).SetInt64(int64(word8&0x7F)), shift))
			shift += 7
			if word8&0x80 == 0 {
				return finalWord, nil
			}
		}
	}

	for {
		word8, err := d.bits8(8)
		if err != nil {
			return nil, err
		}
		// Get 7 least significant bits: word8 & 0x7F
		word7 := word8 & 0x7F

		// Shift and OR into finalWord
		part := new(big.Int).SetInt64(int64(word7))
		shiftedPart := new(big.Int).Lsh(part, shift)
		finalWord.Or(finalWord, shiftedPart)

		// Increment shift by 7
		shift += 7

		// Check MSB: word8 & 0x80
		leadingBit := word8 & 0x80
		if leadingBit == 0 {
			break
		}
	}

	return finalWord, nil
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
