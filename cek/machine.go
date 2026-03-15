package cek

import (
	"fmt"
	"log"
	"math"
	"math/big"
	"reflect"
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
	sharedBuiltinTable      = newBuiltins[syn.DeBruijn]()
	sharedBuiltinValueTable = newBuiltinValueTable[syn.DeBruijn]()
	deBruijnEvalType        = reflect.TypeFor[syn.DeBruijn]()
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
	costs         CostModel
	stepCosts     [9]ExBudget
	builtins      *Builtins[T]
	builtinValues *[builtin.TotalBuiltinCount]*Builtin[T]
	twoArgCosts   [builtin.TotalBuiltinCount]twoArgCost
	available     *[builtin.TotalBuiltinCount]bool
	slippage      uint32
	version       lang.LanguageVersion
	semantics     SemanticsVariant
	protoMajor    uint
	ExBudget      ExBudget
	Logs          []string

	argHolder       argHolder[T]
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
	constantElemChunks     [][]syn.IConstant
	constantElemChunkPos   int
	valueElemChunks        [][]Value[T]
	valueElemChunkPos      int
	builtinChunks          [][]Builtin[T]
	builtinChunkPos        int
	builtinArgChunks       [][]BuiltinArgs[T]
	builtinArgChunkPos     int
	envChunks              [][]Env[T]
	envChunkPos            int
	budgetTemplate         ExBudget
	lastRunRemaining       ExBudget
	hasRun                 bool
}

const (
	envChunkSize          = 8192
	envRetainChunkCap     = 8
	valueChunkSize        = 4096
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

func allocArenaSlot[S any](chunks *[][]S, pos *int) *S {
	remaining := *pos
	for i := range *chunks {
		chunk := (*chunks)[i]
		if chunk == nil {
			continue
		}
		if remaining < len(chunk) {
			slot := &chunk[remaining]
			*pos += 1
			return slot
		}
		remaining -= len(chunk)
	}

	chunk := make([]S, valueChunkSize)
	*chunks = append(*chunks, chunk)
	slot := &chunk[0]
	*pos += 1
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

func (m *Machine[T]) allocDelay(body syn.Term[T], env *Env[T]) *Delay[T] {
	delay := allocArenaSlot(&m.delayChunks, &m.delayChunkPos)
	delay.Body = body
	delay.Env = env
	return delay
}

func (m *Machine[T]) allocConstr(tag uint, fields []Value[T]) *Constr[T] {
	constr := allocArenaSlot(&m.constrChunks, &m.constrChunkPos)
	constr.Tag = tag
	constr.Fields = fields
	return constr
}

func (m *Machine[T]) allocLambda(
	parameterName T,
	body syn.Term[T],
	env *Env[T],
) *Lambda[T] {
	lambda := allocArenaSlot(&m.lambdaChunks, &m.lambdaChunkPos)
	lambda.ParameterName = parameterName
	lambda.Body = body
	lambda.Env = env
	return lambda
}

func (m *Machine[T]) allocConstant(con syn.IConstant) *Constant {
	constant := allocArenaSlot(&m.constantChunks, &m.constantChunkPos)
	constant.Constant = con
	return constant
}

func (m *Machine[T]) allocDataConstant(inner data.PlutusData) *syn.Data {
	dataConstant := allocArenaSlot(&m.dataChunks, &m.dataChunkPos)
	dataConstant.Inner = inner
	return dataConstant
}

func (m *Machine[T]) allocIntegerConstant(inner *big.Int) *syn.Integer {
	integerConstant := allocArenaSlot(&m.integerChunks, &m.integerChunkPos)
	integerConstant.Inner = inner
	return integerConstant
}

func (m *Machine[T]) allocByteStringConstant(inner []byte) *syn.ByteString {
	byteStringConstant := allocArenaSlot(&m.byteStringChunks, &m.byteStringChunkPos)
	byteStringConstant.Inner = inner
	return byteStringConstant
}

func (m *Machine[T]) allocProtoListConstant(typ syn.Typ, list []syn.IConstant) *syn.ProtoList {
	listConstant := allocArenaSlot(&m.protoListChunks, &m.protoListChunkPos)
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
	pairConstant := allocArenaSlot(&m.protoPairChunks, &m.protoPairChunkPos)
	pairConstant.FstType = firstType
	pairConstant.SndType = secondType
	pairConstant.First = first
	pairConstant.Second = second
	return pairConstant
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
		valueChunkSize,
	)
}

func (m *Machine[T]) extendBuiltinArgs(
	next *BuiltinArgs[T],
	data Value[T],
) *BuiltinArgs[T] {
	args := allocArenaSlot(&m.builtinArgChunks, &m.builtinArgChunkPos)
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
	builtinValue := allocArenaSlot(&m.builtinChunks, &m.builtinChunkPos)
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
	if reflect.TypeFor[T]() != deBruijnEvalType {
		panic(
			fmt.Sprintf(
				"cek.NewMachine requires T == syn.DeBruijn, got %v",
				reflect.TypeFor[T](),
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
	return &Machine[T]{
		costs: evalContext.CostModel,
		stepCosts: [9]ExBudget{
			evalContext.CostModel.machineCosts.get(ExConstant),
			evalContext.CostModel.machineCosts.get(ExVar),
			evalContext.CostModel.machineCosts.get(ExLambda),
			evalContext.CostModel.machineCosts.get(ExApply),
			evalContext.CostModel.machineCosts.get(ExDelay),
			evalContext.CostModel.machineCosts.get(ExForce),
			evalContext.CostModel.machineCosts.get(ExBuiltin),
			evalContext.CostModel.machineCosts.get(ExConstr),
			evalContext.CostModel.machineCosts.get(ExCase),
		},
		builtins:      chooseBuiltins[T](version, evalContext.ProtoMajor),
		builtinValues: getSharedBuiltinValues[T](),
		twoArgCosts:   newTwoArgCostCache(evalContext.CostModel.builtinCosts),
		available:     chooseAvailableBuiltins(version, evalContext.ProtoMajor),
		slippage:      slippage,
		version:       version,
		semantics:     evalContext.SemanticsVariant,
		protoMajor:    evalContext.ProtoMajor,
		ExBudget:      DefaultExBudget,
		Logs:          make([]string, 0),

		argHolder:       newArgHolder[T](),
		unbudgetedSteps: [9]uint32{0, 0, 0, 0, 0, 0, 0, 0, 0},
		unbudgetedTotal: 0,

		freeCompute:            make([]*Compute[T], 0, 32),
		freeReturn:             make([]*Return[T], 0, 32),
		freeDone:               make([]*Done[T], 0, 4),
		freeFrameAwaitArg:      make([]*FrameAwaitArg[T], 0, 32),
		freeFrameAwaitFunTerm:  make([]*FrameAwaitFunTerm[T], 0, 32),
		freeFrameAwaitFunValue: make([]*FrameAwaitFunValue[T], 0, 32),
		freeFrameForce:         make([]*FrameForce[T], 0, 16),
		freeFrameConstr:        make([]*FrameConstr[T], 0, 16),
		freeFrameCases:         make([]*FrameCases[T], 0, 16),
		dynamicIntConstants:    make(map[int64]*Constant, 512),
		constrChunks:           make([][]Constr[T], 0, 8),
		constrChunkPos:         0,
		delayChunks:            make([][]Delay[T], 0, 8),
		delayChunkPos:          0,
		lambdaChunks:           make([][]Lambda[T], 0, 8),
		lambdaChunkPos:         0,
		constantChunks:         make([][]Constant, 0, 8),
		constantChunkPos:       0,
		dataChunks:             make([][]syn.Data, 0, 8),
		dataChunkPos:           0,
		integerChunks:          make([][]syn.Integer, 0, 4),
		integerChunkPos:        0,
		byteStringChunks:       make([][]syn.ByteString, 0, 4),
		byteStringChunkPos:     0,
		protoListChunks:        make([][]syn.ProtoList, 0, 8),
		protoListChunkPos:      0,
		protoPairChunks:        make([][]syn.ProtoPair, 0, 8),
		protoPairChunkPos:      0,
		constantElemChunks:     make([][]syn.IConstant, 0, 8),
		constantElemChunkPos:   0,
		valueElemChunks:        make([][]Value[T], 0, 8),
		valueElemChunkPos:      0,
		builtinChunks:          make([][]Builtin[T], 0, 8),
		builtinChunkPos:        0,
		builtinArgChunks:       make([][]BuiltinArgs[T], 0, 8),
		builtinArgChunkPos:     0,
		envChunks:              make([][]Env[T], 0, 8),
		envChunkPos:            0,
		budgetTemplate:         DefaultExBudget,
		lastRunRemaining:       DefaultExBudget,
		hasRun:                 false,
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
	remaining := m.envChunkPos
	for i := range m.envChunks {
		chunk := m.envChunks[i]
		if chunk == nil {
			continue
		}
		if remaining < len(chunk) {
			env := &chunk[remaining]
			m.envChunkPos += 1
			env.data = data
			env.next = parent
			return env
		}
		remaining -= len(chunk)
	}

	chunk := make([]Env[T], envChunkSize)
	m.envChunks = append(m.envChunks, chunk)
	env := &chunk[0]
	m.envChunkPos += 1
	env.data = data
	env.next = parent
	return env
}

func (m *Machine[T]) resetEnvArena() {
	clearArenaChunks(m.envChunks, m.envChunkPos)
	if len(m.envChunks) > envRetainChunkCap {
		for i := envRetainChunkCap; i < len(m.envChunks); i++ {
			m.envChunks[i] = nil
		}
		m.envChunks = m.envChunks[:envRetainChunkCap]
	}
	m.envChunkPos = 0
}

func (m *Machine[T]) resetValueArenas() {
	clearArenaChunks(m.constrChunks, m.constrChunkPos)
	m.constrChunkPos = 0
	clearArenaChunks(m.delayChunks, m.delayChunkPos)
	m.delayChunkPos = 0
	clearArenaChunks(m.lambdaChunks, m.lambdaChunkPos)
	m.lambdaChunkPos = 0
	clearArenaChunks(m.constantChunks, m.constantChunkPos)
	m.constantChunkPos = 0
	clearArenaChunks(m.dataChunks, m.dataChunkPos)
	m.dataChunkPos = 0
	clearArenaChunks(m.integerChunks, m.integerChunkPos)
	m.integerChunkPos = 0
	clearArenaChunks(m.byteStringChunks, m.byteStringChunkPos)
	m.byteStringChunkPos = 0
	clearArenaChunks(m.protoListChunks, m.protoListChunkPos)
	m.protoListChunkPos = 0
	clearArenaChunks(m.protoPairChunks, m.protoPairChunkPos)
	m.protoPairChunkPos = 0
	clearArenaChunks(m.constantElemChunks, m.constantElemChunkPos)
	m.constantElemChunkPos = 0
	clearArenaChunks(m.valueElemChunks, m.valueElemChunkPos)
	m.valueElemChunkPos = 0
	clearArenaChunks(m.builtinChunks, m.builtinChunkPos)
	m.builtinChunkPos = 0
	clearArenaChunks(m.builtinArgChunks, m.builtinArgChunkPos)
	m.builtinArgChunkPos = 0
}

func (m *Machine[T]) finishReturn(ret *Return[T]) (syn.Term[T], error) {
	if m.unbudgetedTotal > 0 {
		if err := m.spendUnbudgetedSteps(); err != nil {
			m.putReturn(ret)
			return nil, err
		}
	}
	term := dischargeValue[T](ret.Value)
	m.putReturn(ret)
	return term, nil
}

// Run executes a Plutus term using the CEK (Control, Environment, Kontinuation) abstract machine.
// The CEK machine is a small-step operational semantics for evaluating functional programs.
// It maintains three components:
// - Control (C): the current term being evaluated
// - Environment (E): mapping from variables to values
// - Kontinuation (K): represents the evaluation context/stack
//
// The algorithm proceeds by repeatedly transitioning between machine states:
// - Compute: evaluate the current term in the current environment and context
// - Return: handle a computed value in the current context
//
// This implementation uses object pooling for performance optimization, reusing
// Compute/Return state objects to minimize garbage collection pressure.
func (m *Machine[T]) Run(term syn.Term[T]) (syn.Term[T], error) {
	if m.hasRun {
		if m.ExBudget != m.lastRunRemaining {
			m.budgetTemplate = m.ExBudget
		} else {
			m.ExBudget = m.budgetTemplate
		}
	} else {
		m.budgetTemplate = m.ExBudget
	}
	m.Logs = m.Logs[:0]
	clear(m.unbudgetedSteps[:])
	m.unbudgetedTotal = 0
	defer func() {
		m.lastRunRemaining = m.ExBudget
		m.hasRun = true
		m.resetValueArenas()
		m.resetEnvArena()
	}()

	// Spend initial startup budget for machine initialization
	startupBudget := m.costs.machineCosts.startup
	if err := m.spendBudget(startupBudget); err != nil {
		return nil, err
	}

	var state MachineState[T]

	// Initialize with a Compute state: evaluate the input term with empty environment
	// and no continuation context (NoFrame)
	comp := m.getCompute()
	comp.Ctx = &NoFrame{}
	comp.Env = nil
	comp.Term = term
	state = comp

	// Main CEK evaluation loop: continue until we reach Done state
	for {
		switch v := state.(type) {
		case *Compute[T]:
			// Compute state: evaluate the current term
			newState, err := m.compute(v.Ctx, v.Env, v.Term)
			if err != nil {
				m.putCompute(v)
				return nil, err
			}
			if newState == nil {
				m.putCompute(v)
				return nil, &InternalError{Code: ErrCodeInternalError, Message: "compute returned nil state"}
			}
			m.putCompute(v)
			state = newState
		case *Return[T]:
			if _, ok := v.Ctx.(*NoFrame); ok {
				return m.finishReturn(v)
			}
			// Return state: handle a computed value in current context
			newState, err := m.returnCompute(v.Ctx, v.Value)
			if err != nil {
				m.putReturn(v)
				return nil, err
			}
			if newState == nil {
				m.putReturn(v)
				return nil, &InternalError{Code: ErrCodeInternalError, Message: "returnCompute returned nil state"}
			}
			m.putReturn(v)
			state = newState
		default:
			panic(fmt.Sprintf("unknown machine state: %T", state))
		}
	}
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

		// Transition to Return state with the looked-up value
		ret := m.getReturn()
		ret.Ctx = context
		ret.Value = value
		state = ret
	case *syn.Delay[T]:
		// Delay creates a suspended computation that can be forced later
		if err := m.stepAndMaybeSpend(ExDelay); err != nil {
			return nil, err
		}

		value := m.allocDelay(t.Term, env)

		ret := m.getReturn()
		ret.Ctx = context
		ret.Value = value
		state = ret
	case *syn.Lambda[T]:
		// Lambda creates a closure capturing the current environment
		if err := m.stepAndMaybeSpend(ExLambda); err != nil {
			return nil, err
		}

		value := m.allocLambda(t.ParameterName, t.Body, env)

		ret := m.getReturn()
		ret.Ctx = context
		ret.Value = value
		state = ret
	case *syn.Apply[T]:
		// Application: evaluate function term first, then argument
		// Uses FrameAwaitFunTerm to remember argument for later evaluation
		if err := m.stepAndMaybeSpend(ExApply); err != nil {
			return nil, err
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

		ret := m.getReturn()
		ret.Ctx = context
		ret.Value = machineConstantValue(m, t.Con)
		state = ret
	case *syn.Force[T]:
		// Force triggers evaluation of a delayed computation
		// Uses FrameForce to handle the result
		if err := m.stepAndMaybeSpend(ExForce); err != nil {
			return nil, err
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

		ret := m.getReturn()
		ret.Ctx = context
		ret.Value = m.builtinValues[t.DefaultFunction]
		state = ret
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
			ret := m.getReturn()
			ret.Ctx = context
			ret.Value = m.allocConstr(t.Tag, nil)
			state = ret
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
		switch v := value.(type) {
		case *Constr[T]:
			if v.Tag > math.MaxInt {
				m.putFrameCases(c)
				return nil, &ScriptError{Code: ErrCodeMaxIntExceeded, Message: "MaxIntExceeded"}
			}
			if indexExists(c.Branches, int(v.Tag)) {
				// Matching branch found, evaluate it with arguments on stack
				comp := m.getCompute()
				comp.Ctx = m.transferArgStack(v.Fields, c.Ctx)
				comp.Env = c.Env
				comp.Term = c.Branches[v.Tag]
				m.putFrameCases(c)
				state = comp
			} else {
				m.putFrameCases(c)
				return nil, &ScriptError{Code: ErrCodeMissingCaseBranch, Message: "MissingCaseBranch"}
			}
		case *Constant:
			// Handle case on constants: Bool, Unit, Integer, List, Pair
			var tag int
			var args []Value[T] // Only allocate when needed (ProtoList, ProtoPair)
			// branchRule semantics:
			// 0: allow any number of branches (Integer)
			// 1: require exactly 1 branch (Unit/Pair)
			// 2: allow up to 2 branches (1 or 2) but disallow >2 (Bool/List)
			branchRule := 0

			switch cval := v.Constant.(type) {
			case *syn.Bool:
				// Bool: allow 1 or 2 branches; >2 invalid
				branchRule = 2
				// False=0, True=1
				if cval.Inner {
					tag = 1
				} else {
					tag = 0
				}
			case *syn.Unit:
				// Unit constants must have exactly 1 branch
				branchRule = 1
				// ()=0
				tag = 0
			case *syn.Integer:
				// Integer: use value as branch index
				if cval.Inner.Sign() < 0 {
					m.putFrameCases(c)
					return nil, &ScriptError{Code: ErrCodeCaseOnNegativeInt, Message: "case on negative integer"}
				}
				if !cval.Inner.IsInt64() {
					m.putFrameCases(c)
					return nil, &ScriptError{Code: ErrCodeCaseIntOutOfRange, Message: "case on integer out of range"}
				}
				ival := cval.Inner.Int64()
				if ival > int64(math.MaxInt) {
					m.putFrameCases(c)
					return nil, &ScriptError{Code: ErrCodeCaseIntOutOfRange, Message: "case on integer out of range"}
				}
				tag = int(ival)
			case *syn.ByteString:
				// ByteString constant not valid in case according to conformance
				m.putFrameCases(c)
				return nil, &ScriptError{Code: ErrCodeCaseOnByteString, Message: "case on bytestring constant not allowed"}
			case *syn.ProtoList:
				// List: allow 1 or 2 branches; >2 invalid
				branchRule = 2
				// cons=0 (with [head, tail] args), nil=1 (no args)
				if len(cval.List) == 0 {
					tag = 1
				} else {
					tag = 0
					// head and tail
					args = m.allocValueElems(2)
					args[0] = m.allocConstant(cval.List[0])
					tail := m.allocProtoListConstant(cval.LTyp, cval.List[1:])
					args[1] = m.allocConstant(tail)
				}
			case *syn.ProtoPair:
				// Pair constants must have exactly 1 branch
				branchRule = 1
				// Pass both fields as args
				tag = 0
				args = m.allocValueElems(2)
				args[0] = m.allocConstant(cval.First)
				args[1] = m.allocConstant(cval.Second)
			default:
				m.putFrameCases(c)
				return nil, &TypeError{Code: ErrCodeNonConstrScrutinized, Message: "NonConstrScrutinized"}
			}

			// Enforce branch count rules for constant cases
			switch branchRule {
			case 1:
				// exact 1
				if len(c.Branches) != 1 {
					m.putFrameCases(c)
					return nil, &ScriptError{Code: ErrCodeInvalidBranchCount, Message: "InvalidCaseBranchCount"}
				}
			case 2:
				// 1 or 2
				if len(c.Branches) < 1 || len(c.Branches) > 2 {
					m.putFrameCases(c)
					return nil, &ScriptError{Code: ErrCodeInvalidBranchCount, Message: "InvalidCaseBranchCount"}
				}
			}

			if indexExists(c.Branches, tag) {
				comp := m.getCompute()
				if args != nil {
					comp.Ctx = m.transferArgStack(args, c.Ctx)
				} else {
					comp.Ctx = c.Ctx
				}
				comp.Env = c.Env
				comp.Term = c.Branches[tag]
				m.putFrameCases(c)
				state = comp
			} else {
				m.putFrameCases(c)
				return nil, &ScriptError{Code: ErrCodeMissingCaseBranch, Message: "MissingCaseBranch"}
			}
		default:
			m.putFrameCases(c)
			return nil, &TypeError{Code: ErrCodeNonConstrScrutinized, Message: "NonConstrScrutinized"}
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

	switch v := value.(type) {
	case *Delay[T]:
		// Force a delayed computation: evaluate body in captured environment
		comp := m.getCompute()
		comp.Ctx = context
		comp.Env = v.Env
		comp.Term = v.Body
		state = comp
	case *Builtin[T]:
		// Force a builtin function application
		if v.NeedsForce() {
			var resolved Value[T]
			nextForces := v.Forces + 1
			if v.Func.ForceCount() == nextForces && v.Func.Arity() == v.ArgCount {
				// Builtin has all arguments, evaluate it
				var err error

				resolved, err = m.evalBuiltinApp(
					m.allocBuiltin(v.Func, nextForces, v.ArgCount, v.Args),
				)
				if err != nil {
					return nil, err
				}
			} else {
				// Still needs more arguments/forces
				resolved = m.allocBuiltin(v.Func, nextForces, v.ArgCount, v.Args)
			}

			ret := m.getReturn()
			ret.Ctx = context
			ret.Value = resolved
			state = ret
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

	switch f := function.(type) {
	case *Lambda[T]:
		// Apply lambda: extend environment and evaluate body
		env := m.extendEnv(f.Env, arg)

		comp := m.getCompute()
		comp.Ctx = context
		comp.Env = env
		comp.Term = f.Body
		state = comp
	case *Builtin[T]:
		// Apply builtin function
		if !f.NeedsForce() && f.IsArrow() {
			var resolved Value[T]
			nextArgCount := f.ArgCount + 1
			if f.Func.Arity() == nextArgCount && f.Func.ForceCount() == f.Forces {
				// Builtin has all arguments, evaluate it
				var err error

				resolved, err = m.evalBuiltinApp(
					m.allocBuiltin(
						f.Func,
						f.Forces,
						nextArgCount,
						m.extendBuiltinArgs(f.Args, arg),
					),
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

			ret := m.getReturn()
			ret.Ctx = context
			ret.Value = resolved
			state = ret
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
func dischargeValue[T syn.Eval](value Value[T]) syn.Term[T] {
	var dischargedTerm syn.Term[T]

	switch v := value.(type) {
	case *Constant:
		dischargedTerm = constantTerm[T](v.Constant)
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
			forcedTerm = &syn.Apply[T]{
				Function: forcedTerm,
				Argument: dischargeValue[T](arg),
			}
		}

		dischargedTerm = forcedTerm
	case *Delay[T]:
		// Discharge delayed computation with environment
		dischargedTerm = &syn.Delay[T]{
			Term: withEnv(0, v.Env, v.Body),
		}

	case *Lambda[T]:
		// Discharge lambda with environment (lamCnt=1 to account for parameter)
		dischargedTerm = &syn.Lambda[T]{
			ParameterName: v.ParameterName,
			Body:          withEnv(1, v.Env, v.Body),
		}

	case *Constr[T]:
		// Recursively discharge all constructor fields
		fields := make([]syn.Term[T], len(v.Fields))

		for i, f := range v.Fields {
			fields[i] = dischargeValue[T](f)
		}

		dischargedTerm = &syn.Constr[T]{
			Tag:    v.Tag,
			Fields: fields,
		}
	}

	return dischargedTerm
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
) syn.Term[T] {
	var dischargedTerm syn.Term[T]

	switch t := term.(type) {
	case *syn.Var[T]:
		// Variable resolution with de Bruijn index adjustment
		if lamCnt >= t.Name.LookupIndex() {
			// Variable is bound by a lambda we haven't discharged yet
			dischargedTerm = t
		} else {
			value, ok := lookupEnv(env, t.Name.LookupIndex()-lamCnt)
			if ok {
				// Variable found in environment, discharge its value
				dischargedTerm = dischargeValue[T](value)
			} else {
				// Free variable (shouldn't happen in well-formed terms)
				dischargedTerm = t
			}
		}

	case *syn.Lambda[T]:
		// Lambda: increase lambda count for body processing
		dischargedTerm = &syn.Lambda[T]{
			ParameterName: t.ParameterName,
			Body:          withEnv(lamCnt+1, env, t.Body),
		}

	case *syn.Apply[T]:
		// Application: process both function and argument
		dischargedTerm = &syn.Apply[T]{
			Function: withEnv(lamCnt, env, t.Function),
			Argument: withEnv(lamCnt, env, t.Argument),
		}
	case *syn.Delay[T]:
		// Delay: process delayed term
		dischargedTerm = &syn.Delay[T]{
			Term: withEnv(lamCnt, env, t.Term),
		}

	case *syn.Force[T]:
		// Force: process term to be forced
		dischargedTerm = &syn.Force[T]{
			Term: withEnv(lamCnt, env, t.Term),
		}

	case *syn.Constr[T]:
		// Constructor: recursively process all fields
		fields := make([]syn.Term[T], len(t.Fields))

		for i, f := range t.Fields {
			fields[i] = withEnv(lamCnt, env, f)
		}

		dischargedTerm = &syn.Constr[T]{
			Tag:    t.Tag,
			Fields: fields,
		}
	case *syn.Case[T]:
		// Case expression: process scrutinee and all branches
		branches := make([]syn.Term[T], len(t.Branches))

		for i, b := range t.Branches {
			branches[i] = withEnv(lamCnt, env, b)
		}

		dischargedTerm = &syn.Case[T]{
			Constr:   withEnv(lamCnt, env, t.Constr),
			Branches: branches,
		}
	default:
		// Constants, builtins, errors: no environment processing needed
		dischargedTerm = t
	}

	return dischargedTerm
}

func (m *Machine[T]) stepAndMaybeSpend(step StepKind) error {
	if m.slippage <= 1 {
		exBudget := m.stepCosts[step]
		m.ExBudget.Mem -= exBudget.Mem
		m.ExBudget.Cpu -= exBudget.Cpu
		if m.ExBudget.Mem < 0 || m.ExBudget.Cpu < 0 {
			return &BudgetError{
				Code:      ErrCodeBudgetExhausted,
				Requested: exBudget,
				Available: ExBudget{
					Cpu: m.ExBudget.Cpu + exBudget.Cpu,
					Mem: m.ExBudget.Mem + exBudget.Mem,
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

	m.ExBudget.Mem -= exBudget.Mem
	m.ExBudget.Cpu -= exBudget.Cpu

	if m.ExBudget.Mem < 0 || m.ExBudget.Cpu < 0 {
		return &BudgetError{
			Code:      ErrCodeBudgetExhausted,
			Requested: exBudget,
			Available: ExBudget{
				Cpu: m.ExBudget.Cpu + exBudget.Cpu,
				Mem: m.ExBudget.Mem + exBudget.Mem,
			},
			Message: "out of budget",
		}
	}

	return nil
}
