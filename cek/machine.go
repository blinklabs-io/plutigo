package cek

import (
	"fmt"
	"log"
	"math"
	"math/big"
	"unsafe"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

// Debug mode for additional runtime checks
const debug = false

// DebugBudget enables verbose budget logging for debugging cost calculation issues
const DebugBudget = false

var (
	sharedBuiltinTable           = newBuiltins[syn.DeBruijn]()
	sharedBuiltinValueTable      = newBuiltinValueTable[syn.DeBruijn]()
	sharedBuiltinNoArgValueTable = newBuiltinNoArgValueTable[syn.DeBruijn]()
)

// Machine is only instantiated with syn.DeBruijn in this codebase. These
// helpers reuse shared syn.DeBruijn builtin tables across Machine[T] instances,
// and NewMachine panics if T does not match syn.DeBruijn.
func getSharedBuiltins[T syn.Eval]() *Builtins[T] {
	return (*Builtins[T])(unsafe.Pointer(&sharedBuiltinTable))
}

func getSharedBuiltinValues[T syn.Eval]() *[builtin.TotalBuiltinCount]*Builtin[T] {
	return (*[builtin.TotalBuiltinCount]*Builtin[T])(unsafe.Pointer(&sharedBuiltinValueTable))
}

func getSharedBuiltinNoArgValues[T syn.Eval]() *[builtin.TotalBuiltinCount][3]*Builtin[T] {
	return (*[builtin.TotalBuiltinCount][3]*Builtin[T])(unsafe.Pointer(&sharedBuiltinNoArgValueTable))
}

// See getSharedBuiltins for the syn.DeBruijn invariant behind this cast.
func getFilteredBuiltins[T syn.Eval](
	version lang.LanguageVersion,
	protoMajor uint,
) *Builtins[T] {
	if protoMajor == 0 {
		switch version {
		case lang.LanguageVersionV1:
			return (*Builtins[T])(unsafe.Pointer(filteredBuiltinsV10))
		case lang.LanguageVersionV2:
			return (*Builtins[T])(unsafe.Pointer(filteredBuiltinsV20))
		case lang.LanguageVersionV3:
			return (*Builtins[T])(unsafe.Pointer(filteredBuiltinsV30))
		case lang.LanguageVersionV4:
			return (*Builtins[T])(unsafe.Pointer(filteredBuiltinsV40))
		}
	}
	return nil
}

type Machine[T syn.Eval] struct {
	costs              CostModel
	stepCosts          [9]ExBudget
	stepCostCpu        [9]int64
	stepCostMem        [9]int64
	builtins           *Builtins[T]
	builtinValues      *[builtin.TotalBuiltinCount]*Builtin[T]
	builtinNoArgValues *[builtin.TotalBuiltinCount][3]*Builtin[T]
	oneArgCosts        [builtin.TotalBuiltinCount]oneArgCost
	twoArgCosts        [builtin.TotalBuiltinCount]twoArgCost
	threeArgCosts      [builtin.TotalBuiltinCount]threeArgCost
	available          *[builtin.TotalBuiltinCount]bool
	slippage           uint32
	version            lang.LanguageVersion
	semantics          SemanticsVariant
	protoMajor         uint
	ExBudget           ExBudget
	Logs               []string

	argHolder       argHolder[T]
	frameStack      []stackFrame[T]
	frameStackUsed  int
	unbudgetedSteps [9]uint32
	unbudgetedTotal uint32

	freeCompute            []*Compute[T]
	freeReturn             []*Return[T]
	freeDone               []*Done[T]
	freeFrameAwaitArg      []*FrameAwaitArg[T]
	freeFrameAwaitFunTerm  []*FrameAwaitFunTerm[T]
	freeFrameAwaitFunValue []*FrameAwaitFunValue[T]
	freeFrameForce         []*FrameForce[T]
	freeFrameConstr        []*FrameConstr[T]
	freeFrameCases         []*FrameCases[T]
	dynamicIntConstants    map[int64]*Constant
	constrChunks           [][]Constr[T]
	constrChunkPos         int
	delayChunks            [][]Delay[T]
	delayChunkPos          int
	lambdaChunks           [][]Lambda[T]
	lambdaChunkPos         int
	constantChunks         [][]Constant
	constantChunkPos       int
	dataChunks             [][]syn.Data
	dataChunkPos           int
	integerChunks          [][]syn.Integer
	integerChunkPos        int
	byteStringChunks       [][]syn.ByteString
	byteStringChunkPos     int
	protoListChunks        [][]syn.ProtoList
	protoListChunkPos      int
	protoPairChunks        [][]syn.ProtoPair
	protoPairChunkPos      int
	dataValueChunks        [][]dataValue[T]
	dataValueChunkPos      int
	dataListValueChunks    [][]dataListValue[T]
	dataListValueChunkPos  int
	dataMapValueChunks     [][]dataMapValue[T]
	dataMapValueChunkPos   int
	pairValueChunks        [][]pairValue[T]
	pairValueChunkPos      int
	constantElemChunks     [][]syn.IConstant
	constantElemChunkPos   int
	valueElemChunks        [][]Value[T]
	valueElemChunkPos      int
	builtinChunks          [][]Builtin[T]
	builtinChunkPos        int
	builtinArgChunks       [][]BuiltinArgs[T]
	builtinArgChunkPos     int
	valueArenaChunkSize    int
	envChunks              [][]Env[T]
	envActiveChunk         []Env[T]
	envActiveChunkLimit    int
	envChunkPos            int
	budgetTemplate         ExBudget
	lastRunRemaining       ExBudget
	hasRun                 bool
}

const (
	envChunkSize          = 2048
	envRetainChunkCap     = 64
	valueColdChunkSize    = 512
	valueMaxChunkSize     = 262144
	valueElemChunkSize    = 1024
	valueRetainChunkCap   = 8
	constantElemChunkSize = 4096
	int64ConstantCacheCap = 4096
)

func (m *Machine[T]) getCompute() *Compute[T] {
	n := len(m.freeCompute)
	if n == 0 {
		return &Compute[T]{}
	}
	c := m.freeCompute[n-1]
	m.freeCompute = m.freeCompute[:n-1]
	return c
}

func (m *Machine[T]) putCompute(c *Compute[T]) {
	c.Ctx = nil
	c.Env = nil
	c.Term = nil
	m.freeCompute = append(m.freeCompute, c)
}

func (m *Machine[T]) getReturn() *Return[T] {
	n := len(m.freeReturn)
	if n == 0 {
		return &Return[T]{}
	}
	r := m.freeReturn[n-1]
	m.freeReturn = m.freeReturn[:n-1]
	return r
}

func (m *Machine[T]) putReturn(r *Return[T]) {
	r.Ctx = nil
	r.Value = nil
	m.freeReturn = append(m.freeReturn, r)
}

func (m *Machine[T]) getDone() *Done[T] {
	n := len(m.freeDone)
	if n == 0 {
		return &Done[T]{}
	}
	d := m.freeDone[n-1]
	m.freeDone = m.freeDone[:n-1]
	return d
}

func (m *Machine[T]) putDone(d *Done[T]) {
	d.term = nil
	m.freeDone = append(m.freeDone, d)
}

func (m *Machine[T]) getFrameAwaitArg() *FrameAwaitArg[T] {
	n := len(m.freeFrameAwaitArg)
	if n == 0 {
		return &FrameAwaitArg[T]{}
	}
	f := m.freeFrameAwaitArg[n-1]
	m.freeFrameAwaitArg = m.freeFrameAwaitArg[:n-1]
	return f
}

func (m *Machine[T]) putFrameAwaitArg(f *FrameAwaitArg[T]) {
	f.Value = nil
	f.Ctx = nil
	m.freeFrameAwaitArg = append(m.freeFrameAwaitArg, f)
}

func (m *Machine[T]) getFrameAwaitFunTerm() *FrameAwaitFunTerm[T] {
	n := len(m.freeFrameAwaitFunTerm)
	if n == 0 {
		return &FrameAwaitFunTerm[T]{}
	}
	f := m.freeFrameAwaitFunTerm[n-1]
	m.freeFrameAwaitFunTerm = m.freeFrameAwaitFunTerm[:n-1]
	return f
}

func (m *Machine[T]) putFrameAwaitFunTerm(f *FrameAwaitFunTerm[T]) {
	f.Env = nil
	f.Term = nil
	f.Ctx = nil
	m.freeFrameAwaitFunTerm = append(m.freeFrameAwaitFunTerm, f)
}

func (m *Machine[T]) getFrameAwaitFunValue() *FrameAwaitFunValue[T] {
	n := len(m.freeFrameAwaitFunValue)
	if n == 0 {
		return &FrameAwaitFunValue[T]{}
	}
	f := m.freeFrameAwaitFunValue[n-1]
	m.freeFrameAwaitFunValue = m.freeFrameAwaitFunValue[:n-1]
	return f
}

func (m *Machine[T]) putFrameAwaitFunValue(f *FrameAwaitFunValue[T]) {
	f.Value = nil
	f.Ctx = nil
	m.freeFrameAwaitFunValue = append(m.freeFrameAwaitFunValue, f)
}

func (m *Machine[T]) getFrameForce() *FrameForce[T] {
	n := len(m.freeFrameForce)
	if n == 0 {
		return &FrameForce[T]{}
	}
	f := m.freeFrameForce[n-1]
	m.freeFrameForce = m.freeFrameForce[:n-1]
	return f
}

func (m *Machine[T]) putFrameForce(f *FrameForce[T]) {
	f.Ctx = nil
	m.freeFrameForce = append(m.freeFrameForce, f)
}

func (m *Machine[T]) getFrameConstr() *FrameConstr[T] {
	n := len(m.freeFrameConstr)
	if n == 0 {
		return &FrameConstr[T]{}
	}
	f := m.freeFrameConstr[n-1]
	m.freeFrameConstr = m.freeFrameConstr[:n-1]
	return f
}

func (m *Machine[T]) putFrameConstr(f *FrameConstr[T]) {
	f.Env = nil
	f.Tag = 0
	f.Fields = nil
	f.ResolvedFields = nil
	f.Ctx = nil
	m.freeFrameConstr = append(m.freeFrameConstr, f)
}

func (m *Machine[T]) getFrameCases() *FrameCases[T] {
	n := len(m.freeFrameCases)
	if n == 0 {
		return &FrameCases[T]{}
	}
	f := m.freeFrameCases[n-1]
	m.freeFrameCases = m.freeFrameCases[:n-1]
	return f
}

func newBuiltinValueTable[T syn.Eval]() [builtin.TotalBuiltinCount]*Builtin[T] {
	var ret [builtin.TotalBuiltinCount]*Builtin[T]
	for i := 0; i < int(builtin.TotalBuiltinCount); i++ {
		ret[i] = &Builtin[T]{Func: builtin.DefaultFunction(i)}
	}
	return ret
}

func newBuiltinNoArgValueTable[T syn.Eval]() [builtin.TotalBuiltinCount][3]*Builtin[T] {
	var ret [builtin.TotalBuiltinCount][3]*Builtin[T]
	for i := 0; i < int(builtin.TotalBuiltinCount); i++ {
		fn := builtin.DefaultFunction(i)
		for forces := 0; forces < len(ret[i]); forces++ {
			ret[i][forces] = &Builtin[T]{
				Func:   fn,
				Forces: uint(forces),
			}
		}
	}
	return ret
}

// allocArenaSlot allocates one slot of S from a chunked arena.
// chunkSize must be a positive power of two; the value-arena chunk sizes
// returned by nextValueArenaChunkSize satisfy this. We exploit that to
// compute the in-chunk offset with a cheap mask instead of a modulo.
func allocArenaSlot[S any](chunks *[][]S, pos *int, chunkSize int) *S {
	if chunkSize <= 0 {
		chunkSize = valueColdChunkSize
	}
	posVal := *pos
	chunkMask := chunkSize - 1
	chunkIdx := posVal / chunkSize
	if chunkIdx == len(*chunks) {
		*chunks = append(*chunks, make([]S, chunkSize))
	}
	slot := &(*chunks)[chunkIdx][posVal&chunkMask]
	*pos = posVal + 1
	return slot
}

func allocArenaSlice[S any](chunks *[][]S, pos *int, n int, chunkSize int) []S {
	if n == 0 {
		return nil
	}

	remaining := *pos
	for i := range *chunks {
		chunk := (*chunks)[i]
		if chunk == nil {
			continue
		}
		if remaining < len(chunk) {
			if remaining+n <= len(chunk) {
				start := remaining
				*pos += n
				return chunk[start : start+n]
			}
			// Skip past the unused tail of this chunk so subsequent
			// allocations don't overlap with the new chunk.
			*pos += len(chunk) - remaining
			remaining = 0
			continue
		}
		remaining -= len(chunk)
	}

	if n > chunkSize {
		chunkSize = n
	}
	chunk := make([]S, chunkSize)
	*chunks = append(*chunks, chunk)
	*pos += n
	return chunk[:n]
}

func clearArenaChunks[S any](chunks [][]S, usedTotal int) {
	if len(chunks) == 0 {
		return
	}

	remaining := usedTotal
	for i := range chunks {
		chunk := chunks[i]
		if chunk == nil {
			continue
		}
		if remaining <= 0 {
			break
		}
		used := len(chunk)
		if remaining < used {
			used = remaining
		}
		clear(chunk[:used])
		remaining -= used
	}
}

func resetArenaChunks[S any](
	chunks *[][]S,
	pos *int,
	retainCap int,
) {
	retainedUsed := *pos
	if len(*chunks) > retainCap {
		maxRetained := 0
		for i := range retainCap {
			maxRetained += len((*chunks)[i])
		}
		if retainedUsed > maxRetained {
			retainedUsed = maxRetained
		}
	}
	clearArenaChunks(*chunks, retainedUsed)
	if len(*chunks) > retainCap {
		retained := make([][]S, retainCap)
		copy(retained, (*chunks)[:retainCap])
		*chunks = retained
	}
	*pos = 0
}

// lazyPrepareArenaChunks releases the previous run's references from a
// per-arena chunk list without reallocating the slice header. It is called
// from Machine.Run's defer so that a long-lived or pooled Machine does not
// pin the previous evaluation's Value, Env, or syn.Term graph between runs.
//
// Chunks beyond retainCap are nil-cleared so the GC can reclaim them. The
// used prefix of the retained chunks is cleared in place so the retained
// slots no longer hold live pointers into the previous run's graph.
func lazyPrepareArenaChunks[S any](
	chunks *[][]S,
	pos *int,
	retainCap int,
) {
	retainedUsed := *pos
	if len(*chunks) > retainCap {
		for i := retainCap; i < len(*chunks); i++ {
			(*chunks)[i] = nil
		}
		*chunks = (*chunks)[:retainCap]
		maxRetained := 0
		for i := range retainCap {
			maxRetained += len((*chunks)[i])
		}
		if retainedUsed > maxRetained {
			retainedUsed = maxRetained
		}
	}
	clearArenaChunks(*chunks, retainedUsed)
	*pos = 0
}

func nextValueArenaChunkSize(used int) int {
	if used <= valueColdChunkSize {
		return valueColdChunkSize
	}
	chunkSize := valueColdChunkSize
	for chunkSize < used && chunkSize < valueMaxChunkSize {
		if chunkSize > valueMaxChunkSize/2 {
			return valueMaxChunkSize
		}
		chunkSize *= 2
	}
	return chunkSize
}

func (m *Machine[T]) valueArenaHighWatermark() int {
	used := m.constrChunkPos
	used = max(used, m.delayChunkPos)
	used = max(used, m.lambdaChunkPos)
	used = max(used, m.constantChunkPos)
	used = max(used, m.dataChunkPos)
	used = max(used, m.integerChunkPos)
	used = max(used, m.byteStringChunkPos)
	used = max(used, m.protoListChunkPos)
	used = max(used, m.protoPairChunkPos)
	used = max(used, m.dataValueChunkPos)
	used = max(used, m.dataListValueChunkPos)
	used = max(used, m.dataMapValueChunkPos)
	used = max(used, m.pairValueChunkPos)
	used = max(used, m.builtinChunkPos)
	used = max(used, m.builtinArgChunkPos)
	return used
}

func (m *Machine[T]) allocDelay(term *syn.Delay[T], env *Env[T]) *Delay[T] {
	delay := allocArenaSlot(&m.delayChunks, &m.delayChunkPos, m.valueArenaChunkSize)
	delay.AST = term
	delay.Env = env
	return delay
}

func (m *Machine[T]) allocConstr(tag uint, fields []Value[T]) *Constr[T] {
	constr := allocArenaSlot(&m.constrChunks, &m.constrChunkPos, m.valueArenaChunkSize)
	constr.Tag = tag
	constr.Fields = fields
	return constr
}

func (m *Machine[T]) allocLambda(term *syn.Lambda[T], env *Env[T]) *Lambda[T] {
	lambda := allocArenaSlot(&m.lambdaChunks, &m.lambdaChunkPos, m.valueArenaChunkSize)
	lambda.AST = term
	lambda.Env = env
	return lambda
}

func (m *Machine[T]) allocConstant(con syn.IConstant) *Constant {
	constant := allocArenaSlot(&m.constantChunks, &m.constantChunkPos, m.valueArenaChunkSize)
	constant.Constant = con
	return constant
}

func (m *Machine[T]) allocDataConstant(inner data.PlutusData) *syn.Data {
	dataConstant := allocArenaSlot(&m.dataChunks, &m.dataChunkPos, m.valueArenaChunkSize)
	dataConstant.Inner = inner
	return dataConstant
}

func (m *Machine[T]) allocIntegerConstant(inner *big.Int) *syn.Integer {
	integerConstant := allocArenaSlot(&m.integerChunks, &m.integerChunkPos, m.valueArenaChunkSize)
	integerConstant.SetInner(inner)
	return integerConstant
}

func (m *Machine[T]) allocByteStringConstant(inner []byte) *syn.ByteString {
	byteStringConstant := allocArenaSlot(&m.byteStringChunks, &m.byteStringChunkPos, m.valueArenaChunkSize)
	byteStringConstant.Inner = inner
	return byteStringConstant
}

func (m *Machine[T]) allocProtoListConstant(typ syn.Typ, list []syn.IConstant) *syn.ProtoList {
	listConstant := allocArenaSlot(&m.protoListChunks, &m.protoListChunkPos, m.valueArenaChunkSize)
	listConstant.LTyp = typ
	listConstant.List = list
	return listConstant
}

func (m *Machine[T]) allocProtoPairConstant(
	firstType syn.Typ,
	secondType syn.Typ,
	first syn.IConstant,
	second syn.IConstant,
) *syn.ProtoPair {
	pairConstant := allocArenaSlot(&m.protoPairChunks, &m.protoPairChunkPos, m.valueArenaChunkSize)
	pairConstant.FstType = firstType
	pairConstant.SndType = secondType
	pairConstant.First = first
	pairConstant.Second = second
	return pairConstant
}

func (m *Machine[T]) allocDataListValue(
	items []data.PlutusData,
) *dataListValue[T] {
	listValue := allocArenaSlot(&m.dataListValueChunks, &m.dataListValueChunkPos, m.valueArenaChunkSize)
	listValue.items = items
	return listValue
}

func (m *Machine[T]) allocDataValue(item data.PlutusData) *dataValue[T] {
	dataVal := allocArenaSlot(&m.dataValueChunks, &m.dataValueChunkPos, m.valueArenaChunkSize)
	dataVal.item = item
	return dataVal
}

func (m *Machine[T]) allocDataMapValue(
	items [][2]data.PlutusData,
) *dataMapValue[T] {
	listValue := allocArenaSlot(&m.dataMapValueChunks, &m.dataMapValueChunkPos, m.valueArenaChunkSize)
	listValue.items = items
	return listValue
}

func (m *Machine[T]) allocDataPairValue(
	first data.PlutusData,
	second data.PlutusData,
) *pairValue[T] {
	return m.allocPairValue(
		m.allocDataValue(first),
		m.allocDataValue(second),
	)
}

func (m *Machine[T]) allocPairValue(
	first Value[T],
	second Value[T],
) *pairValue[T] {
	pair := allocArenaSlot(&m.pairValueChunks, &m.pairValueChunkPos, m.valueArenaChunkSize)
	pair.first = first
	pair.second = second
	return pair
}

func (m *Machine[T]) allocConstantElems(n int) []syn.IConstant {
	return allocArenaSlice(
		&m.constantElemChunks,
		&m.constantElemChunkPos,
		n,
		constantElemChunkSize,
	)
}

func (m *Machine[T]) allocValueElems(n int) []Value[T] {
	return allocArenaSlice(
		&m.valueElemChunks,
		&m.valueElemChunkPos,
		n,
		valueElemChunkSize,
	)
}

func (m *Machine[T]) extendBuiltinArgs(
	next *BuiltinArgs[T],
	data Value[T],
) *BuiltinArgs[T] {
	args := allocArenaSlot(&m.builtinArgChunks, &m.builtinArgChunkPos, m.valueArenaChunkSize)
	args.data = data
	args.next = next
	return args
}

func (m *Machine[T]) allocBuiltin(
	fn builtin.DefaultFunction,
	forces uint,
	argCount uint,
	args *BuiltinArgs[T],
) *Builtin[T] {
	if argCount == 0 && args == nil && forces < 3 {
		return m.builtinNoArgValues[fn][forces]
	}
	builtinValue := allocArenaSlot(&m.builtinChunks, &m.builtinChunkPos, m.valueArenaChunkSize)
	builtinValue.Func = fn
	builtinValue.Forces = forces
	builtinValue.ArgCount = argCount
	builtinValue.Args = args
	return builtinValue
}

func (m *Machine[T]) putFrameCases(f *FrameCases[T]) {
	f.Env = nil
	f.Branches = nil
	f.Ctx = nil
	m.freeFrameCases = append(m.freeFrameCases, f)
}

// NewMachine creates a CEK machine for a De Bruijn-indexed program.
// The second argument is slippage, which controls batched budget checking.
// Protocol-version-dependent semantics and builtin availability come from
// evalContext.ProtoMajor, not from slippage.
func NewMachine[T syn.Eval](
	version lang.LanguageVersion,
	slippage uint32,
	evalContext *EvalContext,
) *Machine[T] {
	var zero T
	if _, ok := any(zero).(syn.DeBruijn); !ok {
		panic(
			fmt.Sprintf(
				"cek.NewMachine requires T == syn.DeBruijn, got %T",
				any(zero),
			),
		)
	}
	if evalContext == nil {
		// Use the default V3 cost models and semantics variant if no eval context is provided
		evalContext = &EvalContext{
			CostModel:        DefaultCostModel,
			SemanticsVariant: SemanticsVariantC,
		}
	}
	stepCosts := [9]ExBudget{
		evalContext.CostModel.machineCosts.get(ExConstant),
		evalContext.CostModel.machineCosts.get(ExVar),
		evalContext.CostModel.machineCosts.get(ExLambda),
		evalContext.CostModel.machineCosts.get(ExApply),
		evalContext.CostModel.machineCosts.get(ExDelay),
		evalContext.CostModel.machineCosts.get(ExForce),
		evalContext.CostModel.machineCosts.get(ExBuiltin),
		evalContext.CostModel.machineCosts.get(ExConstr),
		evalContext.CostModel.machineCosts.get(ExCase),
	}
	var stepCostCpu [9]int64
	var stepCostMem [9]int64
	for i, s := range stepCosts {
		stepCostCpu[i] = s.Cpu
		stepCostMem[i] = s.Mem
	}
	return &Machine[T]{
		costs:              evalContext.CostModel,
		stepCosts:          stepCosts,
		stepCostCpu:        stepCostCpu,
		stepCostMem:        stepCostMem,
		builtins:           chooseBuiltins[T](version, evalContext.ProtoMajor),
		builtinValues:      getSharedBuiltinValues[T](),
		builtinNoArgValues: getSharedBuiltinNoArgValues[T](),
		oneArgCosts:        newOneArgCostCache(evalContext.CostModel.builtinCosts),
		twoArgCosts:        newTwoArgCostCache(evalContext.CostModel.builtinCosts),
		threeArgCosts:      newThreeArgCostCache(evalContext.CostModel.builtinCosts),
		available:          chooseAvailableBuiltins(version, evalContext.ProtoMajor),
		slippage:           slippage,
		version:            version,
		semantics:          evalContext.SemanticsVariant,
		protoMajor:         evalContext.ProtoMajor,
		ExBudget:           DefaultExBudget,
		Logs:               make([]string, 0),

		argHolder:       newArgHolder[T](),
		frameStack:      make([]stackFrame[T], 0, 32),
		frameStackUsed:  0,
		unbudgetedSteps: [9]uint32{0, 0, 0, 0, 0, 0, 0, 0, 0},
		unbudgetedTotal: 0,

		constrChunks:          make([][]Constr[T], 0, 8),
		constrChunkPos:        0,
		delayChunks:           make([][]Delay[T], 0, 8),
		delayChunkPos:         0,
		lambdaChunks:          make([][]Lambda[T], 0, 8),
		lambdaChunkPos:        0,
		constantChunks:        make([][]Constant, 0, 8),
		constantChunkPos:      0,
		dataChunks:            make([][]syn.Data, 0, 8),
		dataChunkPos:          0,
		integerChunks:         make([][]syn.Integer, 0, 4),
		integerChunkPos:       0,
		byteStringChunks:      make([][]syn.ByteString, 0, 4),
		byteStringChunkPos:    0,
		protoListChunks:       make([][]syn.ProtoList, 0, 8),
		protoListChunkPos:     0,
		protoPairChunks:       make([][]syn.ProtoPair, 0, 8),
		protoPairChunkPos:     0,
		dataValueChunks:       make([][]dataValue[T], 0, 8),
		dataValueChunkPos:     0,
		dataListValueChunks:   make([][]dataListValue[T], 0, 8),
		dataListValueChunkPos: 0,
		dataMapValueChunks:    make([][]dataMapValue[T], 0, 8),
		dataMapValueChunkPos:  0,
		pairValueChunks:       make([][]pairValue[T], 0, 8),
		pairValueChunkPos:     0,
		constantElemChunks:    make([][]syn.IConstant, 0, 8),
		constantElemChunkPos:  0,
		valueElemChunks:       make([][]Value[T], 0, 8),
		valueElemChunkPos:     0,
		builtinChunks:         make([][]Builtin[T], 0, 8),
		builtinChunkPos:       0,
		builtinArgChunks:      make([][]BuiltinArgs[T], 0, 8),
		builtinArgChunkPos:    0,
		valueArenaChunkSize:   valueColdChunkSize,
		envChunks:             make([][]Env[T], 0, 8),
		envChunkPos:           0,
		budgetTemplate:        DefaultExBudget,
		lastRunRemaining:      DefaultExBudget,
		hasRun:                false,
	}
}

func chooseBuiltins[T syn.Eval](
	version lang.LanguageVersion,
	protoMajor uint,
) *Builtins[T] {
	if filtered := getFilteredBuiltins[T](version, protoMajor); filtered != nil {
		return filtered
	}
	return getSharedBuiltins[T]()
}

func chooseAvailableBuiltins(
	version lang.LanguageVersion,
	protoMajor uint,
) *[builtin.TotalBuiltinCount]bool {
	if getFilteredBuiltins[syn.DeBruijn](version, protoMajor) != nil {
		return nil
	}
	return newAvailableBuiltins(version, protoMajor)
}

func (m *Machine[T]) extendEnv(parent *Env[T], data Value[T]) *Env[T] {
	pos := m.envChunkPos
	chunk := m.envActiveChunk
	if chunk == nil || pos == m.envActiveChunkLimit {
		chunkIdx := pos / envChunkSize
		if chunkIdx == len(m.envChunks) {
			m.envChunks = append(m.envChunks, make([]Env[T], envChunkSize))
		}
		chunk = m.envChunks[chunkIdx]
		if chunk == nil {
			chunk = make([]Env[T], envChunkSize)
			m.envChunks[chunkIdx] = chunk
		}
		m.envActiveChunk = chunk
		m.envActiveChunkLimit = (chunkIdx + 1) * envChunkSize
	}
	env := &chunk[pos&(envChunkSize-1)]
	m.envChunkPos = pos + 1
	env.data = data
	env.next = parent
	if parent != nil {
		skip := parent.next
		if skip != nil {
			skip = skip.next
			if skip != nil {
				env.skip4 = skip.next
			}
		}
	}
	return env
}

func (m *Machine[T]) resetEnvArena() {
	retainedUsed := m.envChunkPos
	maxRetained := envRetainChunkCap * envChunkSize
	if retainedUsed > maxRetained {
		retainedUsed = maxRetained
	}
	clearArenaChunks(m.envChunks, retainedUsed)
	if len(m.envChunks) > envRetainChunkCap {
		retained := make([][]Env[T], envRetainChunkCap)
		copy(retained, m.envChunks[:envRetainChunkCap])
		m.envChunks = retained
	}
	m.envActiveChunk = nil
	m.envActiveChunkLimit = 0
	m.envChunkPos = 0
}

// lazyPrepareEnvArena trims chunks beyond envRetainChunkCap and clears the
// used prefix of the retained env chunks so Run does not leave pointers into
// the previous run's Env/Value graph pinned inside the Machine.
func (m *Machine[T]) lazyPrepareEnvArena() {
	lazyPrepareArenaChunks(&m.envChunks, &m.envChunkPos, envRetainChunkCap)
	m.envActiveChunk = nil
	m.envActiveChunkLimit = 0
}

func (m *Machine[T]) dropEnvArena() {
	m.envChunks = nil
	m.envActiveChunk = nil
	m.envActiveChunkLimit = 0
	m.envChunkPos = 0
}

func (m *Machine[T]) resetValueArenas() {
	resetArenaChunks(&m.constrChunks, &m.constrChunkPos, valueRetainChunkCap)
	resetArenaChunks(&m.delayChunks, &m.delayChunkPos, valueRetainChunkCap)
	resetArenaChunks(&m.lambdaChunks, &m.lambdaChunkPos, valueRetainChunkCap)
	resetArenaChunks(&m.constantChunks, &m.constantChunkPos, valueRetainChunkCap)
	resetArenaChunks(&m.dataChunks, &m.dataChunkPos, valueRetainChunkCap)
	resetArenaChunks(&m.integerChunks, &m.integerChunkPos, valueRetainChunkCap)
	resetArenaChunks(&m.byteStringChunks, &m.byteStringChunkPos, valueRetainChunkCap)
	resetArenaChunks(&m.protoListChunks, &m.protoListChunkPos, valueRetainChunkCap)
	resetArenaChunks(&m.protoPairChunks, &m.protoPairChunkPos, valueRetainChunkCap)
	resetArenaChunks(&m.dataValueChunks, &m.dataValueChunkPos, valueRetainChunkCap)
	resetArenaChunks(&m.dataListValueChunks, &m.dataListValueChunkPos, valueRetainChunkCap)
	resetArenaChunks(&m.dataMapValueChunks, &m.dataMapValueChunkPos, valueRetainChunkCap)
	resetArenaChunks(&m.pairValueChunks, &m.pairValueChunkPos, valueRetainChunkCap)
	resetArenaChunks(&m.constantElemChunks, &m.constantElemChunkPos, valueRetainChunkCap)
	resetArenaChunks(&m.valueElemChunks, &m.valueElemChunkPos, valueRetainChunkCap)
	resetArenaChunks(&m.builtinChunks, &m.builtinChunkPos, valueRetainChunkCap)
	resetArenaChunks(&m.builtinArgChunks, &m.builtinArgChunkPos, valueRetainChunkCap)
}

func (m *Machine[T]) lazyPrepareValueArenas() {
	lazyPrepareArenaChunks(&m.constrChunks, &m.constrChunkPos, valueRetainChunkCap)
	lazyPrepareArenaChunks(&m.delayChunks, &m.delayChunkPos, valueRetainChunkCap)
	lazyPrepareArenaChunks(&m.lambdaChunks, &m.lambdaChunkPos, valueRetainChunkCap)
	lazyPrepareArenaChunks(&m.constantChunks, &m.constantChunkPos, valueRetainChunkCap)
	lazyPrepareArenaChunks(&m.dataChunks, &m.dataChunkPos, valueRetainChunkCap)
	lazyPrepareArenaChunks(&m.integerChunks, &m.integerChunkPos, valueRetainChunkCap)
	lazyPrepareArenaChunks(&m.byteStringChunks, &m.byteStringChunkPos, valueRetainChunkCap)
	lazyPrepareArenaChunks(&m.protoListChunks, &m.protoListChunkPos, valueRetainChunkCap)
	lazyPrepareArenaChunks(&m.protoPairChunks, &m.protoPairChunkPos, valueRetainChunkCap)
	lazyPrepareArenaChunks(&m.dataValueChunks, &m.dataValueChunkPos, valueRetainChunkCap)
	lazyPrepareArenaChunks(&m.dataListValueChunks, &m.dataListValueChunkPos, valueRetainChunkCap)
	lazyPrepareArenaChunks(&m.dataMapValueChunks, &m.dataMapValueChunkPos, valueRetainChunkCap)
	lazyPrepareArenaChunks(&m.pairValueChunks, &m.pairValueChunkPos, valueRetainChunkCap)
	lazyPrepareArenaChunks(&m.constantElemChunks, &m.constantElemChunkPos, valueRetainChunkCap)
	lazyPrepareArenaChunks(&m.valueElemChunks, &m.valueElemChunkPos, valueRetainChunkCap)
	lazyPrepareArenaChunks(&m.builtinChunks, &m.builtinChunkPos, valueRetainChunkCap)
	lazyPrepareArenaChunks(&m.builtinArgChunks, &m.builtinArgChunkPos, valueRetainChunkCap)
}

func (m *Machine[T]) dropValueArenas() {
	m.constrChunks = nil
	m.constrChunkPos = 0
	m.delayChunks = nil
	m.delayChunkPos = 0
	m.lambdaChunks = nil
	m.lambdaChunkPos = 0
	m.constantChunks = nil
	m.constantChunkPos = 0
	m.dataChunks = nil
	m.dataChunkPos = 0
	m.integerChunks = nil
	m.integerChunkPos = 0
	m.byteStringChunks = nil
	m.byteStringChunkPos = 0
	m.protoListChunks = nil
	m.protoListChunkPos = 0
	m.protoPairChunks = nil
	m.protoPairChunkPos = 0
	m.dataValueChunks = nil
	m.dataValueChunkPos = 0
	m.dataListValueChunks = nil
	m.dataListValueChunkPos = 0
	m.dataMapValueChunks = nil
	m.dataMapValueChunkPos = 0
	m.pairValueChunks = nil
	m.pairValueChunkPos = 0
	m.constantElemChunks = nil
	m.constantElemChunkPos = 0
	m.valueElemChunks = nil
	m.valueElemChunkPos = 0
	m.builtinChunks = nil
	m.builtinChunkPos = 0
	m.builtinArgChunks = nil
	m.builtinArgChunkPos = 0
}

func (m *Machine[T]) finishReturn(ret *Return[T]) (syn.Term[T], error) {
	term, err := m.finishValue(ret.Value)
	m.putReturn(ret)
	return term, err
}

func (m *Machine[T]) finishValue(value Value[T]) (syn.Term[T], error) {
	if m.unbudgetedTotal > 0 {
		if err := m.spendUnbudgetedSteps(); err != nil {
			return nil, err
		}
	}
	return dischargeValue[T](value)
}

// Run executes a Plutus term using the CEK (Control, Environment, Kontinuation) abstract machine.
// This implementation now uses an explicit machine-owned frame stack on the hot
// path while preserving the existing setup, teardown, budget, and discharge
// behavior.
//
// Arena teardown runs in the deferred block so a long-lived or pooled
// Machine does not retain references into the previous evaluation's
// Value/Env/syn.Term graph after Run returns. The entry path only handles
// budget restoration and frame-stack reset (the latter also defends against
// tests or other callers that tamper with the frame stack between runs).
func (m *Machine[T]) Run(term syn.Term[T]) (syn.Term[T], error) {
	firstRun := !m.hasRun
	if m.hasRun {
		if m.valueArenaChunkSize <= 0 {
			m.valueArenaChunkSize = valueColdChunkSize
		}
		if m.ExBudget != m.lastRunRemaining {
			m.budgetTemplate = m.ExBudget
		} else {
			m.ExBudget = m.budgetTemplate
		}
		m.resetFrameStack()
	} else {
		m.valueArenaChunkSize = valueColdChunkSize
		m.budgetTemplate = m.ExBudget
	}
	runValueArenaChunkSize := m.valueArenaChunkSize
	m.Logs = m.Logs[:0]
	clear(m.unbudgetedSteps[:])
	m.unbudgetedTotal = 0
	defer func() {
		nextChunkSize := nextValueArenaChunkSize(m.valueArenaHighWatermark())
		m.lastRunRemaining = m.ExBudget
		m.hasRun = true
		m.resetFrameStack()
		m.valueArenaChunkSize = nextChunkSize
		if firstRun {
			m.dropValueArenas()
			m.dropEnvArena()
			return
		}
		if nextChunkSize != runValueArenaChunkSize {
			m.dropValueArenas()
			m.lazyPrepareEnvArena()
			return
		}
		m.lazyPrepareValueArenas()
		m.lazyPrepareEnvArena()
	}()

	// Spend initial startup budget for machine initialization
	startupBudget := m.costs.machineCosts.startup
	if err := m.spendBudget(startupBudget); err != nil {
		return nil, err
	}
	if m.slippage <= 1 {
		dbMachine := (*Machine[syn.DeBruijn])(unsafe.Pointer(m))
		dbTerm, ok := any(term).(syn.Term[syn.DeBruijn])
		if !ok {
			return nil, &InternalError{
				Code: ErrCodeInternalError,
				Message: fmt.Sprintf(
					"DeBruijn evaluator expected syn.Term[syn.DeBruijn], got %T",
					term,
				),
			}
		}
		dbResult, err := runStackNoSlippageDeBruijn(dbMachine, dbTerm)
		if err != nil {
			return nil, err
		}
		result, ok := any(dbResult).(syn.Term[T])
		if !ok {
			return nil, &InternalError{
				Code: ErrCodeInternalError,
				Message: fmt.Sprintf(
					"DeBruijn evaluator produced incompatible term type %T",
					dbResult,
				),
			}
		}
		return result, nil
	}
	return m.runStack(term)
}

// compute handles the Compute state of the CEK machine.
// It takes the current evaluation context, environment, and term,
// and returns the next machine state after processing the term.
//
// The method implements the core evaluation rules for each Plutus term type:
// - Variables: look up in environment
// - Constants: wrap in Constant value
// - Lambdas: create closure with current environment
// - Applications: evaluate function first, then argument
// - Delays: create suspended computation
// - Forces: trigger evaluation of delayed terms
// - Builtins: create builtin function applications
// - Constructors: evaluate fields sequentially
// - Case expressions: evaluate scrutinee then match branches
func (m *Machine[T]) compute(
	context MachineContext[T],
	env *Env[T],
	term syn.Term[T],
) (MachineState[T], error) {
	var state MachineState[T]
	var err error

	switch t := term.(type) {
	case *syn.Var[T]:
		// Variable lookup: spend budget and retrieve value from environment
		if err := m.stepAndMaybeSpend(ExVar); err != nil {
			return nil, err
		}

		value, ok := lookupEnv(env, t.Name.LookupIndex())
		if !ok {
			return nil, &TypeError{Code: ErrCodeOpenTerm, Message: "open term evaluated"}
		}

		state, err = m.returnValueState(context, value)
		if err != nil {
			return nil, err
		}
	case *syn.Delay[T]:
		// Delay creates a suspended computation that can be forced later
		if err := m.stepAndMaybeSpend(ExDelay); err != nil {
			return nil, err
		}

		value := m.allocDelay(t, env)

		state, err = m.returnValueState(context, value)
		if err != nil {
			return nil, err
		}
	case *syn.Lambda[T]:
		// Lambda creates a closure capturing the current environment
		if err := m.stepAndMaybeSpend(ExLambda); err != nil {
			return nil, err
		}

		value := m.allocLambda(t, env)

		state, err = m.returnValueState(context, value)
		if err != nil {
			return nil, err
		}
	case *syn.Apply[T]:
		// Application: evaluate function term first, then argument
		// Uses FrameAwaitFunTerm to remember argument for later evaluation
		if err := m.stepAndMaybeSpend(ExApply); err != nil {
			return nil, err
		}

		funValue, ok, err := m.computeImmediateValue(env, t.Function)
		if err != nil {
			return nil, err
		}
		if ok {
			argValue, argImmediate, err := m.computeImmediateValue(env, t.Argument)
			if err != nil {
				return nil, err
			}
			if argImmediate {
				return m.applyEvaluate(context, funValue, argValue)
			}

			frame := m.getFrameAwaitArg()
			frame.Ctx = context
			frame.Value = funValue

			comp := m.getCompute()
			comp.Ctx = frame
			comp.Env = env
			comp.Term = t.Argument
			state = comp
			break
		}

		frame := m.getFrameAwaitFunTerm()
		frame.Env = env
		frame.Term = t.Argument // Remember argument to evaluate later
		frame.Ctx = context

		comp := m.getCompute()
		comp.Ctx = frame
		comp.Env = env
		comp.Term = t.Function
		state = comp
	case *syn.Constant:
		// Constants are already evaluated values
		if err := m.stepAndMaybeSpend(ExConstant); err != nil {
			return nil, err
		}

		state, err = m.returnValueState(context, machineConstantValue(m, t.Con))
		if err != nil {
			return nil, err
		}
	case *syn.Force[T]:
		// Force triggers evaluation of a delayed computation
		// Uses FrameForce to handle the result
		if err := m.stepAndMaybeSpend(ExForce); err != nil {
			return nil, err
		}

		forcedValue, ok, err := m.computeImmediateValue(env, t.Term)
		if err != nil {
			return nil, err
		}
		if ok {
			return m.forceEvaluate(context, forcedValue)
		}

		frame := m.getFrameForce()
		frame.Ctx = context

		comp := m.getCompute()
		comp.Ctx = frame
		comp.Env = env
		comp.Term = t.Term
		state = comp
	case *syn.Error:
		// Explicit error term - evaluation fails
		return nil, &ScriptError{Code: ErrCodeExplicitError, Message: "error explicitly called"}

	case *syn.Builtin:
		// Builtin functions are treated as values
		if err := m.stepAndMaybeSpend(ExBuiltin); err != nil {
			return nil, err
		}

		state, err = m.returnValueState(context, m.builtinValues[t.DefaultFunction])
		if err != nil {
			return nil, err
		}
	case *syn.Constr[T]:
		// Constructor: evaluate all fields sequentially
		// If no fields, create constructor value immediately
		// Otherwise, use FrameConstr to evaluate fields one by one
		if err := m.stepAndMaybeSpend(ExConstr); err != nil {
			return nil, err
		}

		fields := t.Fields

		if len(fields) == 0 {
			// No fields to evaluate
			state, err = m.returnValueState(context, m.allocConstr(t.Tag, nil))
			if err != nil {
				return nil, err
			}
		} else {
			// Evaluate fields sequentially using FrameConstr
			firstField := fields[0]

			rest := fields[1:]

			frame := m.getFrameConstr()
			frame.Ctx = context
			frame.Tag = t.Tag
			frame.Fields = rest // Remaining fields to evaluate
			frame.ResolvedFields = m.allocValueElems(len(t.Fields))[:0]
			frame.Env = env

			comp := m.getCompute()
			comp.Ctx = frame
			comp.Env = env
			comp.Term = firstField
			state = comp
		}
	case *syn.Case[T]:
		// Case expression: evaluate scrutinee, then match against branches
		// Uses FrameCases to handle branching logic
		if err := m.stepAndMaybeSpend(ExCase); err != nil {
			return nil, err
		}

		scrutinee, ok, err := m.computeImmediateValue(env, t.Constr)
		if err != nil {
			return nil, err
		}
		if ok {
			return m.caseEvaluate(env, t.Branches, context, scrutinee)
		}

		frame := m.getFrameCases()
		frame.Env = env
		frame.Ctx = context
		frame.Branches = t.Branches

		comp := m.getCompute()
		comp.Ctx = frame
		comp.Env = env
		comp.Term = t.Constr
		state = comp
	default:
		panic(fmt.Sprintf("unknown term: %T: %v", term, term))
	}

	if state == nil {
		return nil, &InternalError{
			Code:    ErrCodeInternalError,
			Message: "compute: state is nil",
		}
	}

	return state, nil
}

func (m *Machine[T]) returnValueState(
	context MachineContext[T],
	value Value[T],
) (MachineState[T], error) {
	if _, ok := context.(*NoFrame); ok {
		ret := m.getReturn()
		ret.Ctx = context
		ret.Value = value
		return ret, nil
	}
	return m.returnCompute(context, value)
}

func (m *Machine[T]) computeImmediateValue(
	env *Env[T],
	term syn.Term[T],
) (Value[T], bool, error) {
	switch t := term.(type) {
	case *syn.Var[T]:
		if err := m.stepAndMaybeSpend(ExVar); err != nil {
			return nil, true, err
		}
		value, ok := lookupEnv(env, t.Name.LookupIndex())
		if !ok {
			return nil, true, &TypeError{Code: ErrCodeOpenTerm, Message: "open term evaluated"}
		}
		return value, true, nil
	case *syn.Delay[T]:
		if err := m.stepAndMaybeSpend(ExDelay); err != nil {
			return nil, true, err
		}
		return m.allocDelay(t, env), true, nil
	case *syn.Lambda[T]:
		if err := m.stepAndMaybeSpend(ExLambda); err != nil {
			return nil, true, err
		}
		return m.allocLambda(t, env), true, nil
	case *syn.Constant:
		if err := m.stepAndMaybeSpend(ExConstant); err != nil {
			return nil, true, err
		}
		return machineConstantValue(m, t.Con), true, nil
	case *syn.Error:
		return nil, true, &ScriptError{Code: ErrCodeExplicitError, Message: "error explicitly called"}
	case *syn.Builtin:
		if err := m.stepAndMaybeSpend(ExBuiltin); err != nil {
			return nil, true, err
		}
		return m.builtinValues[t.DefaultFunction], true, nil
	case *syn.Constr[T]:
		if len(t.Fields) != 0 {
			return nil, false, nil
		}
		if err := m.stepAndMaybeSpend(ExConstr); err != nil {
			return nil, true, err
		}
		return m.allocConstr(t.Tag, nil), true, nil
	default:
		return nil, false, nil
	}
}

func (m *Machine[T]) caseEvaluate(
	env *Env[T],
	branches []syn.Term[T],
	context MachineContext[T],
	value Value[T],
) (MachineState[T], error) {
	switch v := value.(type) {
	case *Constr[T]:
		if v.Tag > math.MaxInt {
			return nil, &ScriptError{Code: ErrCodeMaxIntExceeded, Message: "MaxIntExceeded"}
		}
		if indexExists(branches, int(v.Tag)) {
			comp := m.getCompute()
			comp.Ctx = m.transferArgStack(v.Fields, context)
			comp.Env = env
			comp.Term = branches[v.Tag]
			return comp, nil
		}
		return nil, &ScriptError{Code: ErrCodeMissingCaseBranch, Message: "MissingCaseBranch"}
	case *Constant:
		var tag int
		var args []Value[T]
		branchRule := 0

		switch cval := v.Constant.(type) {
		case *syn.Bool:
			branchRule = 2
			if cval.Inner {
				tag = 1
			} else {
				tag = 0
			}
		case *syn.Unit:
			branchRule = 1
			tag = 0
		case *syn.Integer:
			if cval.Inner.Sign() < 0 {
				return nil, &ScriptError{Code: ErrCodeCaseOnNegativeInt, Message: "case on negative integer"}
			}
			if !cval.Inner.IsInt64() {
				return nil, &ScriptError{Code: ErrCodeCaseIntOutOfRange, Message: "case on integer out of range"}
			}
			ival := cval.Inner.Int64()
			if ival > int64(math.MaxInt) {
				return nil, &ScriptError{Code: ErrCodeCaseIntOutOfRange, Message: "case on integer out of range"}
			}
			tag = int(ival)
		case *syn.ByteString:
			return nil, &ScriptError{Code: ErrCodeCaseOnByteString, Message: "case on bytestring constant not allowed"}
		case *syn.ProtoList:
			branchRule = 2
			if len(cval.List) == 0 {
				tag = 1
			} else {
				tag = 0
				args = m.allocValueElems(2)
				args[0] = m.allocConstant(cval.List[0])
				tail := m.allocProtoListConstant(cval.LTyp, cval.List[1:])
				args[1] = m.allocConstant(tail)
			}
		case *syn.ProtoPair:
			branchRule = 1
			tag = 0
			args = m.allocValueElems(2)
			args[0] = m.allocConstant(cval.First)
			args[1] = m.allocConstant(cval.Second)
		default:
			return nil, &TypeError{Code: ErrCodeNonConstrScrutinized, Message: "NonConstrScrutinized"}
		}

		switch branchRule {
		case 1:
			if len(branches) != 1 {
				return nil, &ScriptError{Code: ErrCodeInvalidBranchCount, Message: "InvalidCaseBranchCount"}
			}
		case 2:
			if len(branches) < 1 || len(branches) > 2 {
				return nil, &ScriptError{Code: ErrCodeInvalidBranchCount, Message: "InvalidCaseBranchCount"}
			}
		}

		if indexExists(branches, tag) {
			comp := m.getCompute()
			if args != nil {
				comp.Ctx = m.transferArgStack(args, context)
			} else {
				comp.Ctx = context
			}
			comp.Env = env
			comp.Term = branches[tag]
			return comp, nil
		}
		return nil, &ScriptError{Code: ErrCodeMissingCaseBranch, Message: "MissingCaseBranch"}
	case *dataListValue[T]:
		if len(v.items) == 0 {
			if len(branches) != 2 && len(branches) != 1 {
				return nil, &ScriptError{Code: ErrCodeInvalidBranchCount, Message: "InvalidCaseBranchCount"}
			}
			if !indexExists(branches, 1) {
				return nil, &ScriptError{Code: ErrCodeMissingCaseBranch, Message: "MissingCaseBranch"}
			}
			comp := m.getCompute()
			comp.Ctx = context
			comp.Env = env
			comp.Term = branches[1]
			return comp, nil
		}

		if len(branches) < 1 || len(branches) > 2 {
			return nil, &ScriptError{Code: ErrCodeInvalidBranchCount, Message: "InvalidCaseBranchCount"}
		}

		args := m.allocValueElems(2)
		args[0] = m.allocDataValue(v.items[0])
		args[1] = m.allocDataListValue(v.items[1:])
		comp := m.getCompute()
		comp.Ctx = m.transferArgStack(args, context)
		comp.Env = env
		comp.Term = branches[0]
		return comp, nil
	case *dataMapValue[T]:
		if len(v.items) == 0 {
			if len(branches) != 2 && len(branches) != 1 {
				return nil, &ScriptError{Code: ErrCodeInvalidBranchCount, Message: "InvalidCaseBranchCount"}
			}
			if !indexExists(branches, 1) {
				return nil, &ScriptError{Code: ErrCodeMissingCaseBranch, Message: "MissingCaseBranch"}
			}
			comp := m.getCompute()
			comp.Ctx = context
			comp.Env = env
			comp.Term = branches[1]
			return comp, nil
		}

		if len(branches) < 1 || len(branches) > 2 {
			return nil, &ScriptError{Code: ErrCodeInvalidBranchCount, Message: "InvalidCaseBranchCount"}
		}

		args := m.allocValueElems(2)
		args[0] = m.allocDataPairValue(v.items[0][0], v.items[0][1])
		args[1] = m.allocDataMapValue(v.items[1:])
		comp := m.getCompute()
		comp.Ctx = m.transferArgStack(args, context)
		comp.Env = env
		comp.Term = branches[0]
		return comp, nil
	case *pairValue[T]:
		if len(branches) != 1 {
			return nil, &ScriptError{Code: ErrCodeInvalidBranchCount, Message: "InvalidCaseBranchCount"}
		}
		args := m.allocValueElems(2)
		args[0] = v.first
		args[1] = v.second
		comp := m.getCompute()
		comp.Ctx = m.transferArgStack(args, context)
		comp.Env = env
		comp.Term = branches[0]
		return comp, nil
	default:
		return nil, &TypeError{Code: ErrCodeNonConstrScrutinized, Message: "NonConstrScrutinized"}
	}
}

// returnCompute handles the Return state of the CEK machine.
// It takes the current evaluation context and a computed value,
// and determines the next state based on the continuation frame.
//
// This method implements the "return" rules of the CEK machine:
// - FrameAwaitArg: apply function to argument (after both are evaluated)
// - FrameAwaitFunTerm: evaluate argument term for function application
// - FrameAwaitFunValue: apply function value to argument value
// - FrameForce: handle forcing of delayed computations
// - FrameConstr: accumulate evaluated constructor fields
// - FrameCases: pattern match on constructor values
// - NoFrame: evaluation complete, discharge value to final term
func (m *Machine[T]) returnCompute(
	context MachineContext[T],
	value Value[T],
) (MachineState[T], error) {
	var state MachineState[T]
	var err error

	switch c := context.(type) {
	case *FrameAwaitArg[T]:
		// Function term evaluated, now apply to argument value
		state, err = m.applyEvaluate(c.Ctx, c.Value, value)
		m.putFrameAwaitArg(c)
		if err != nil {
			return nil, err
		}
	case *FrameAwaitFunTerm[T]:
		// Function evaluated to a value, now evaluate argument term
		argValue, ok, err := m.computeImmediateValue(c.Env, c.Term)
		if err != nil {
			m.putFrameAwaitFunTerm(c)
			return nil, err
		}
		if ok {
			state, err = m.applyEvaluate(c.Ctx, value, argValue)
			m.putFrameAwaitFunTerm(c)
			if err != nil {
				return nil, err
			}
			break
		}
		comp := m.getCompute()
		frame := m.getFrameAwaitArg()
		frame.Ctx = c.Ctx
		frame.Value = value // Function value
		comp.Ctx = frame
		comp.Env = c.Env
		comp.Term = c.Term
		m.putFrameAwaitFunTerm(c)
		state = comp
	case *FrameAwaitFunValue[T]:
		// Argument evaluated to a value, now apply to function value
		state, err = m.applyEvaluate(c.Ctx, value, c.Value)
		m.putFrameAwaitFunValue(c)
		if err != nil {
			return nil, err
		}
	case *FrameForce[T]:
		// Handle forcing of delayed computations or builtin applications
		state, err = m.forceEvaluate(c.Ctx, value)
		m.putFrameForce(c)
		if err != nil {
			return nil, err
		}
	case *FrameConstr[T]:
		// Accumulate evaluated constructor fields
		resolvedFields := append(c.ResolvedFields, value)

		fields := c.Fields

		if len(fields) == 0 {
			// All fields evaluated, create constructor value
			ret := m.getReturn()
			ret.Ctx = c.Ctx
			ret.Value = m.allocConstr(c.Tag, resolvedFields)
			m.putFrameConstr(c)
			state = ret
		} else {
			// More fields to evaluate
			firstField := fields[0]
			rest := fields[1:]
			comp := m.getCompute()
			frame := m.getFrameConstr()
			frame.Ctx = c.Ctx
			frame.Tag = c.Tag
			frame.Fields = rest
			frame.ResolvedFields = resolvedFields
			frame.Env = c.Env
			comp.Ctx = frame
			comp.Env = c.Env
			comp.Term = firstField
			m.putFrameConstr(c)
			state = comp
		}
	case *FrameCases[T]:
		// Pattern match on constructor or constant values
		state, err = m.caseEvaluate(c.Env, c.Branches, c.Ctx, value)
		m.putFrameCases(c)
		if err != nil {
			return nil, err
		}
	case *NoFrame:
		return nil, &InternalError{
			Code:    ErrCodeInternalError,
			Message: "returnCompute reached NoFrame; Run should finalize directly",
		}
	default:
		panic(fmt.Sprintf("unknown context %v", context))
	}

	if state == nil {
		return nil, &InternalError{
			Code:    ErrCodeInternalError,
			Message: "returnCompute: state is nil",
		}
	}

	return state, nil
}

// forceEvaluate handles forcing of delayed computations and builtin applications.
// Force is used to trigger evaluation of suspended computations created by Delay.
//
// For Delay values: resumes evaluation in the captured environment
// For Builtin values: applies forces to builtin functions (for polymorphism)
func (m *Machine[T]) forceEvaluate(
	context MachineContext[T],
	value Value[T],
) (MachineState[T], error) {
	var state MachineState[T]
	var err error

	switch v := value.(type) {
	case *Delay[T]:
		// Force a delayed computation: evaluate body in captured environment
		comp := m.getCompute()
		comp.Ctx = context
		comp.Env = v.Env
		comp.Term = v.AST.Term
		state = comp
	case *Builtin[T]:
		// Force a builtin function application
		if v.NeedsForce() {
			var resolved Value[T]
			nextForces := v.Forces + 1
			if v.Func.ForceCount() == nextForces && v.Func.Arity() == v.ArgCount {
				// Builtin has all arguments, evaluate it
				var err error

				resolved, err = m.evalBuiltinAppReady(
					v.Func,
					nextForces,
					v.ArgCount,
					v.Args,
				)
				if err != nil {
					return nil, err
				}
			} else {
				// Still needs more arguments/forces
				resolved = m.allocBuiltin(v.Func, nextForces, v.ArgCount, v.Args)
			}

			state, err = m.returnValueState(context, resolved)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, &TypeError{Code: ErrCodeBuiltinForceExpected, Message: "BuiltinTermArgumentExpected"}
		}
	default:
		return nil, &TypeError{Code: ErrCodeNonPolymorphic, Message: "NonPolymorphicInstantiation"}
	}

	return state, nil
}

// applyEvaluate handles function application in the CEK machine.
// It takes a function value and an argument value, and applies the function.
//
// For Lambda values: extends the captured environment with the argument
// For Builtin values: applies the argument to the builtin function
func (m *Machine[T]) applyEvaluate(
	context MachineContext[T],
	function Value[T],
	arg Value[T],
) (MachineState[T], error) {
	var state MachineState[T]
	var err error

	switch f := function.(type) {
	case *Lambda[T]:
		// Apply lambda: extend environment and evaluate body
		env := m.extendEnv(f.Env, arg)

		comp := m.getCompute()
		comp.Ctx = context
		comp.Env = env
		comp.Term = f.AST.Body
		state = comp
	case *Builtin[T]:
		// Apply builtin function
		if !f.NeedsForce() && f.IsArrow() {
			var resolved Value[T]
			nextArgCount := f.ArgCount + 1
			if f.Func.Arity() == nextArgCount && f.Func.ForceCount() == f.Forces {
				// Builtin has all arguments, evaluate it
				var err error

				resolved, err = m.evalBuiltinAppWithArg(
					f.Func,
					f.Forces,
					nextArgCount,
					f.Args,
					arg,
				)
				if err != nil {
					return nil, err
				}
			} else {
				// Still needs more arguments
				resolved = m.allocBuiltin(
					f.Func,
					f.Forces,
					nextArgCount,
					m.extendBuiltinArgs(f.Args, arg),
				)
			}

			state, err = m.returnValueState(context, resolved)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, &TypeError{Code: ErrCodeUnexpectedBuiltinArg, Message: "UnexpectedBuiltinTermArgument"}
		}
	default:
		return nil, &TypeError{Code: ErrCodeNonFunctionalApp, Message: "NonFunctionalApplication"}
	}

	return state, nil
}

func (m *Machine[T]) transferArgStack(
	fields []Value[T],
	ctx MachineContext[T],
) MachineContext[T] {
	c := ctx

	for arg := len(fields) - 1; arg >= 0; arg-- {
		frame := m.getFrameAwaitFunValue()
		frame.Ctx = c
		frame.Value = fields[arg]
		c = frame
	}

	return c
}

// dischargeValue converts a runtime Value back to a syntax Term.
// This is the inverse operation of evaluation - it takes computed values
// and reconstructs the equivalent Plutus terms that would produce them.
//
// The process handles different value types:
// - Constants: directly wrap in Constant term
// - Builtins: reconstruct force/apply chains for partial applications
// - Delays: create Delay term with environment-discharged body
// - Lambdas: create Lambda term with environment-discharged body
// - Constructors: recursively discharge all fields
//
// This function is crucial for producing the final result when evaluation
// reaches the Done state, ensuring the output is a valid Plutus term.
func dischargeValue[T syn.Eval](value Value[T]) (syn.Term[T], error) {
	switch v := value.(type) {
	case *Constant:
		return constantTerm[T](v.Constant), nil
	case *dataValue[T]:
		return constantTerm[T](&syn.Data{Inner: v.item}), nil
	case *dataListValue[T]:
		return constantTerm[T](materializeDataListConstant(v.items)), nil
	case *dataMapValue[T]:
		return constantTerm[T](materializeDataMapConstant(v.items)), nil
	case *pairValue[T]:
		constant, ok := materializeConstantValue[T](v)
		if !ok {
			return nil, &InternalError{
				Code:    ErrCodeInternalError,
				Message: fmt.Sprintf("cannot discharge non-constant pair value: %T", v),
			}
		}
		return constantTerm[T](constant), nil
	case *Builtin[T]:
		// Reconstruct the term that represents this builtin application
		var forcedTerm syn.Term[T]

		forcedTerm = &syn.Builtin{
			DefaultFunction: v.Func,
		}

		// Add forces for polymorphic instantiation
		for range uint(v.Forces) {
			forcedTerm = &syn.Force[T]{
				Term: forcedTerm,
			}
		}

		// Add applications for each argument
		for arg := range v.Args.Iter() {
			discharged, err := dischargeValue[T](arg)
			if err != nil {
				return nil, err
			}
			forcedTerm = &syn.Apply[T]{
				Function: forcedTerm,
				Argument: discharged,
			}
		}

		return forcedTerm, nil
	case *Delay[T]:
		// Discharge delayed computation with environment
		body, err := withEnv(0, v.Env, v.AST.Term)
		if err != nil {
			return nil, err
		}
		return &syn.Delay[T]{Term: body}, nil

	case *Lambda[T]:
		// Discharge lambda with environment (lamCnt=1 to account for parameter)
		body, err := withEnv(1, v.Env, v.AST.Body)
		if err != nil {
			return nil, err
		}
		return &syn.Lambda[T]{
			ParameterName: v.AST.ParameterName,
			Body:          body,
		}, nil

	case *Constr[T]:
		// Recursively discharge all constructor fields
		fields := make([]syn.Term[T], len(v.Fields))

		for i, f := range v.Fields {
			discharged, err := dischargeValue[T](f)
			if err != nil {
				return nil, err
			}
			fields[i] = discharged
		}

		return &syn.Constr[T]{
			Tag:    v.Tag,
			Fields: fields,
		}, nil
	}

	return nil, &InternalError{
		Code: ErrCodeInternalError,
		Message: fmt.Sprintf(
			"unsupported value kind in dischargeValue: %T",
			value,
		),
	}
}

// withEnv discharges a term while substituting values from an environment.
// This implements lexical scoping by replacing free variables with their
// bound values from the evaluation environment.
//
// Parameters:
// - lamCnt: number of lambda binders we've traversed (for de Bruijn indexing)
// - env: environment containing variable bindings
// - term: the term to discharge
//
// The function handles variable lookup with proper de Bruijn index adjustment,
// recursively processing complex terms while maintaining environment bindings.
func withEnv[T syn.Eval](
	lamCnt int,
	env *Env[T],
	term syn.Term[T],
) (syn.Term[T], error) {
	switch t := term.(type) {
	case *syn.Var[T]:
		// Variable resolution with de Bruijn index adjustment
		if lamCnt >= t.Name.LookupIndex() {
			// Variable is bound by a lambda we haven't discharged yet
			return t, nil
		}
		value, ok := lookupEnv(env, t.Name.LookupIndex()-lamCnt)
		if ok {
			// Variable found in environment, discharge its value
			return dischargeValue[T](value)
		}
		// Free variable (shouldn't happen in well-formed terms)
		return t, nil

	case *syn.Lambda[T]:
		// Lambda: increase lambda count for body processing
		body, err := withEnv(lamCnt+1, env, t.Body)
		if err != nil {
			return nil, err
		}
		return &syn.Lambda[T]{
			ParameterName: t.ParameterName,
			Body:          body,
		}, nil

	case *syn.Apply[T]:
		// Application: process both function and argument
		fn, err := withEnv(lamCnt, env, t.Function)
		if err != nil {
			return nil, err
		}
		arg, err := withEnv(lamCnt, env, t.Argument)
		if err != nil {
			return nil, err
		}
		return &syn.Apply[T]{
			Function: fn,
			Argument: arg,
		}, nil

	case *syn.Delay[T]:
		// Delay: process delayed term
		inner, err := withEnv(lamCnt, env, t.Term)
		if err != nil {
			return nil, err
		}
		return &syn.Delay[T]{Term: inner}, nil

	case *syn.Force[T]:
		// Force: process term to be forced
		inner, err := withEnv(lamCnt, env, t.Term)
		if err != nil {
			return nil, err
		}
		return &syn.Force[T]{Term: inner}, nil

	case *syn.Constr[T]:
		// Constructor: recursively process all fields
		fields := make([]syn.Term[T], len(t.Fields))
		for i, f := range t.Fields {
			d, err := withEnv(lamCnt, env, f)
			if err != nil {
				return nil, err
			}
			fields[i] = d
		}
		return &syn.Constr[T]{
			Tag:    t.Tag,
			Fields: fields,
		}, nil

	case *syn.Case[T]:
		// Case expression: process scrutinee and all branches
		branches := make([]syn.Term[T], len(t.Branches))
		for i, b := range t.Branches {
			d, err := withEnv(lamCnt, env, b)
			if err != nil {
				return nil, err
			}
			branches[i] = d
		}
		constr, err := withEnv(lamCnt, env, t.Constr)
		if err != nil {
			return nil, err
		}
		return &syn.Case[T]{
			Constr:   constr,
			Branches: branches,
		}, nil

	default:
		// Constants, builtins, errors: no environment processing needed
		return t, nil
	}
}

func (m *Machine[T]) stepAndMaybeSpend(step StepKind) error {
	if m.slippage <= 1 {
		memCost := m.stepCostMem[step]
		cpuCost := m.stepCostCpu[step]
		m.ExBudget.Mem -= memCost
		m.ExBudget.Cpu -= cpuCost
		if m.ExBudget.Mem < 0 || m.ExBudget.Cpu < 0 {
			return &BudgetError{
				Code: ErrCodeBudgetExhausted,
				Requested: ExBudget{
					Cpu: cpuCost,
					Mem: memCost,
				},
				Available: ExBudget{
					Cpu: m.ExBudget.Cpu + cpuCost,
					Mem: m.ExBudget.Mem + memCost,
				},
				Message: "out of budget",
			}
		}
		return nil
	}

	m.unbudgetedSteps[step] += 1
	m.unbudgetedTotal += 1

	if m.unbudgetedTotal >= m.slippage {
		if err := m.spendUnbudgetedSteps(); err != nil {
			return err
		}
	}

	return nil
}

func (m *Machine[T]) spendUnbudgetedSteps() error {
	for i := range uint8(len(m.unbudgetedSteps)) {
		unspentStepBudget := m.stepCosts[StepKind(i)]

		unspentStepBudget.occurrences(m.unbudgetedSteps[i])

		if err := m.spendBudget(unspentStepBudget); err != nil {
			return err
		}

		m.unbudgetedSteps[i] = 0
	}

	m.unbudgetedTotal = 0

	return nil
}

func (m *Machine[T]) spendBudget(exBudget ExBudget) error {
	if DebugBudget {
		log.Printf(
			"[PLUTIGO-BUDGET] Spending mem=%d cpu=%d, before: mem=%d cpu=%d",
			exBudget.Mem,
			exBudget.Cpu,
			m.ExBudget.Mem,
			m.ExBudget.Cpu,
		)
	}

	if exBudget.Mem < 0 || exBudget.Cpu < 0 {
		return m.budgetError(exBudget, "invalid negative budget cost")
	}

	if exBudget.Mem > m.ExBudget.Mem || exBudget.Cpu > m.ExBudget.Cpu {
		return m.budgetError(exBudget, "out of budget")
	}

	m.ExBudget.Mem -= exBudget.Mem
	m.ExBudget.Cpu -= exBudget.Cpu

	return nil
}

func (m *Machine[T]) budgetCostOverflowError(exBudget ExBudget) error {
	return m.budgetError(exBudget, "budget cost overflow")
}

func (m *Machine[T]) budgetError(exBudget ExBudget, message string) error {
	return &BudgetError{
		Code:      ErrCodeBudgetExhausted,
		Requested: exBudget,
		Available: m.ExBudget,
		Message:   message,
	}
}
