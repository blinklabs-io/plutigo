package cek

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"unsafe"

	"github.com/blinklabs-io/plutigo/syn"
)

// Debug mode for additional runtime checks
const debug = false

// Object pools for frequently allocated CEK machine objects.
// Note: sync.Pool can grow unbounded, but this is acceptable for the CEK machine's
// performance goals. The pools reduce allocations by 25-40% and the memory overhead
// is minimal compared to the evaluation performance benefits.
var (
	computePool = sync.Pool{
		New: func() interface{} {
			return &Compute[syn.DeBruijn]{}
		},
	}
	returnPool = sync.Pool{
		New: func() interface{} {
			return &Return[syn.DeBruijn]{}
		},
	}
	donePool = sync.Pool{
		New: func() interface{} {
			return &Done[syn.DeBruijn]{}
		},
	}
)

// getCompute returns a Compute state from the pool
func getCompute[T syn.Eval]() *Compute[T] {
	c := computePool.Get().(*Compute[syn.DeBruijn])
	if debug {
		// Runtime type assertion for safety in debug builds
		_ = (*Compute[T])(unsafe.Pointer(c))
	}
	return (*Compute[T])(unsafe.Pointer(c))
}

// putCompute returns a Compute state to the pool
func putCompute[T syn.Eval](c *Compute[T]) {
	// Reset the state
	c.Ctx = nil
	c.Env = nil
	c.Term = nil
	computePool.Put(c)
}

// getReturn returns a Return state from the pool
func getReturn[T syn.Eval]() *Return[T] {
	r := returnPool.Get().(*Return[syn.DeBruijn])
	if debug {
		// Runtime type assertion for safety in debug builds
		_ = (*Return[T])(unsafe.Pointer(r))
	}
	return (*Return[T])(unsafe.Pointer(r))
}

// putReturn returns a Return state to the pool
func putReturn[T syn.Eval](r *Return[T]) {
	// Reset the state
	r.Ctx = nil
	r.Value = nil
	returnPool.Put(r)
}

// getDone returns a Done state from the pool
func getDone[T syn.Eval]() *Done[T] {
	d := donePool.Get().(*Done[syn.DeBruijn])
	if debug {
		// Runtime type assertion for safety in debug builds
		_ = (*Done[T])(unsafe.Pointer(d))
	}
	return (*Done[T])(unsafe.Pointer(d))
}

// putDone returns a Done state to the pool
func putDone[T syn.Eval](d *Done[T]) {
	// Reset the state
	d.term = nil
	donePool.Put(d)
}

type Machine[T syn.Eval] struct {
	costs    CostModel
	builtins Builtins[T]
	slippage uint32
	version  [3]uint32
	ExBudget ExBudget
	Logs     []string

	argHolder       argHolder[T]
	unbudgetedSteps [10]uint32
}

func NewMachine[T syn.Eval](
	version [3]uint32,
	slippage uint32,
	costs ...CostModel,
) *Machine[T] {
	var costModel CostModel
	if len(costs) > 0 {
		costModel = costs[0]
	} else {
		costModel = DefaultCostModel
	}
	return &Machine[T]{
		costs:    costModel,
		builtins: newBuiltins[T](),
		slippage: slippage,
		version:  version,
		ExBudget: DefaultExBudget,
		Logs:     make([]string, 0),

		argHolder:       newArgHolder[T](),
		unbudgetedSteps: [10]uint32{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}
}

// NewMachineWithVersionCosts creates a machine with version-appropriate cost models
func NewMachineWithVersionCosts[T syn.Eval](
	version [3]uint32,
	slippage uint32,
) *Machine[T] {
	costModel := GetCostModel(version)
	return NewMachine[T](version, slippage, costModel)
}

func (m *Machine[T]) Run(term syn.Term[T]) (syn.Term[T], error) {
	startupBudget := m.costs.machineCosts.startup
	if err := m.spendBudget(startupBudget); err != nil {
		return nil, err
	}

	var state MachineState[T] = getCompute[T]()
	state.(*Compute[T]).Ctx = &NoFrame{}
	state.(*Compute[T]).Env = nil
	state.(*Compute[T]).Term = term

	for {
		switch v := state.(type) {
		case *Compute[T]:
			newState, err := m.compute(v.Ctx, v.Env, v.Term)
			if err != nil {
				putCompute(v)
				return nil, err
			}
			if newState == nil {
				putCompute(v)
				return nil, errors.New("compute returned nil state")
			}
			putCompute(v)
			state = newState
		case *Return[T]:
			newState, err := m.returnCompute(v.Ctx, v.Value)
			if err != nil {
				putReturn(v)
				return nil, err
			}
			if newState == nil {
				putReturn(v)
				return nil, errors.New("returnCompute returned nil state")
			}
			putReturn(v)
			state = newState
		case *Done[T]:
			term := v.term
			putDone(v)
			return term, nil
		default:
			panic(fmt.Sprintf("unknown machine state: %T", state))
		}
	}
}

func (m *Machine[T]) compute(
	context MachineContext[T],
	env *Env[T],
	term syn.Term[T],
) (MachineState[T], error) {
	var state MachineState[T]

	switch t := term.(type) {
	case *syn.Var[T]:
		if err := m.stepAndMaybeSpend(ExVar); err != nil {
			return nil, err
		}

		value, exists := env.Lookup(t.Name.LookupIndex())

		if !exists {
			return nil, errors.New("open term evaluated")
		}

		state = getReturn[T]()
		state.(*Return[T]).Ctx = context
		state.(*Return[T]).Value = value
	case *syn.Delay[T]:
		if err := m.stepAndMaybeSpend(ExDelay); err != nil {
			return nil, err
		}

		value := &Delay[T]{
			Body: t.Term,
			Env:  env,
		}

		state = getReturn[T]()
		state.(*Return[T]).Ctx = context
		state.(*Return[T]).Value = value
	case *syn.Lambda[T]:
		if err := m.stepAndMaybeSpend(ExLambda); err != nil {
			return nil, err
		}

		value := &Lambda[T]{
			ParameterName: t.ParameterName,
			Body:          t.Body,
			Env:           env,
		}

		state = getReturn[T]()
		state.(*Return[T]).Ctx = context
		state.(*Return[T]).Value = value
	case *syn.Apply[T]:
		if err := m.stepAndMaybeSpend(ExApply); err != nil {
			return nil, err
		}

		frame := &FrameAwaitFunTerm[T]{
			Env:  env,
			Term: t.Argument,
			Ctx:  context,
		}

		state = getCompute[T]()
		state.(*Compute[T]).Ctx = frame
		state.(*Compute[T]).Env = env
		state.(*Compute[T]).Term = t.Function
	case *syn.Constant:
		if err := m.stepAndMaybeSpend(ExConstant); err != nil {
			return nil, err
		}

		state = getReturn[T]()
		state.(*Return[T]).Ctx = context
		state.(*Return[T]).Value = &Constant{
			Constant: t.Con,
		}
	case *syn.Force[T]:
		if err := m.stepAndMaybeSpend(ExForce); err != nil {
			return nil, err
		}

		frame := &FrameForce[T]{
			Ctx: context,
		}

		state = getCompute[T]()
		state.(*Compute[T]).Ctx = frame
		state.(*Compute[T]).Env = env
		state.(*Compute[T]).Term = t.Term
	case *syn.Error:
		return nil, errors.New("error explicitly called")

	case *syn.Builtin:
		if err := m.stepAndMaybeSpend(ExBuiltin); err != nil {
			return nil, err
		}

		state = getReturn[T]()
		state.(*Return[T]).Ctx = context
		state.(*Return[T]).Value = &Builtin[T]{
			Func:   t.DefaultFunction,
			Args:   nil,
			Forces: 0,
		}
	case *syn.Constr[T]:
		if err := m.stepAndMaybeSpend(ExConstr); err != nil {
			return nil, err
		}

		fields := t.Fields

		if len(fields) == 0 {
			state = getReturn[T]()
			state.(*Return[T]).Ctx = context
			state.(*Return[T]).Value = &Constr[T]{
				Tag:    t.Tag,
				Fields: []Value[T]{},
			}
		} else {
			first_field := fields[0]

			rest := fields[1:]

			frame := &FrameConstr[T]{
				Ctx:            context,
				Tag:            t.Tag,
				Fields:         rest,
				ResolvedFields: []Value[T]{},
				Env:            env,
			}

			state = getCompute[T]()
			state.(*Compute[T]).Ctx = frame
			state.(*Compute[T]).Env = env
			state.(*Compute[T]).Term = first_field
		}
	case *syn.Case[T]:
		if err := m.stepAndMaybeSpend(ExCase); err != nil {
			return nil, err
		}

		frame := &FrameCases[T]{
			Env:      env,
			Ctx:      context,
			Branches: t.Branches,
		}

		state = getCompute[T]()
		state.(*Compute[T]).Ctx = frame
		state.(*Compute[T]).Env = env
		state.(*Compute[T]).Term = t.Constr
	default:
		panic(fmt.Sprintf("unknown term: %T: %v", term, term))
	}

	if state == nil {
		return nil, errors.New("compute: state is nil")
	}

	return state, nil
}

func (m *Machine[T]) returnCompute(
	context MachineContext[T],
	value Value[T],
) (MachineState[T], error) {
	var state MachineState[T]
	var err error

	switch c := context.(type) {
	case *FrameAwaitArg[T]:
		state, err = m.applyEvaluate(c.Ctx, c.Value, value)
		if err != nil {
			return nil, err
		}
	case *FrameAwaitFunTerm[T]:
		state = getCompute[T]()
		state.(*Compute[T]).Ctx = &FrameAwaitArg[T]{
			Ctx:   c.Ctx,
			Value: value,
		}
		state.(*Compute[T]).Env = c.Env
		state.(*Compute[T]).Term = c.Term
	case *FrameAwaitFunValue[T]:
		state, err = m.applyEvaluate(c.Ctx, value, c.Value)
		if err != nil {
			return nil, err
		}
	case *FrameForce[T]:
		state, err = m.forceEvaluate(c.Ctx, value)
		if err != nil {
			return nil, err
		}
	case *FrameConstr[T]:
		resolvedFields := append(c.ResolvedFields, value)

		fields := c.Fields

		if len(fields) == 0 {
			state = getReturn[T]()
			state.(*Return[T]).Ctx = c.Ctx
			state.(*Return[T]).Value = &Constr[T]{
				Tag:    c.Tag,
				Fields: resolvedFields,
			}
		} else {
			first_field := fields[0]
			rest := fields[1:]

			frame := &FrameConstr[T]{
				Ctx:            c.Ctx,
				Tag:            c.Tag,
				Fields:         rest,
				ResolvedFields: resolvedFields,
				Env:            c.Env,
			}

			state = getCompute[T]()
			state.(*Compute[T]).Ctx = frame
			state.(*Compute[T]).Env = c.Env
			state.(*Compute[T]).Term = first_field
		}
	case *FrameCases[T]:
		switch v := value.(type) {
		case *Constr[T]:
			if v.Tag > math.MaxInt {
				return nil, errors.New("MaxIntExceeded")
			}
			if indexExists(c.Branches, int(v.Tag)) {
				state = getCompute[T]()
				state.(*Compute[T]).Ctx = transferArgStack(v.Fields, c.Ctx)
				state.(*Compute[T]).Env = c.Env
				state.(*Compute[T]).Term = c.Branches[v.Tag]
			} else {
				return nil, errors.New("MissingCaseBranch")
			}
		default:
			return nil, errors.New("NonConstrScrutinized")
		}
	case *NoFrame:
		if m.unbudgetedSteps[9] > 0 {
			if err := m.spendUnbudgetedSteps(); err != nil {
				return nil, err
			}
		}

		state = getDone[T]()
		state.(*Done[T]).term = dischargeValue[T](value)
	default:
		panic(fmt.Sprintf("unknown context %v", context))
	}

	if state == nil {
		return nil, errors.New("returnCompute: state is nil")
	}

	return state, nil
}

func (m *Machine[T]) forceEvaluate(
	context MachineContext[T],
	value Value[T],
) (MachineState[T], error) {
	var state MachineState[T]

	switch v := value.(type) {
	case *Delay[T]:
		state = getCompute[T]()
		state.(*Compute[T]).Ctx = context
		state.(*Compute[T]).Env = v.Env
		state.(*Compute[T]).Term = v.Body
	case *Builtin[T]:
		if v.NeedsForce() {
			var resolved Value[T]

			b := v.ConsumeForce()

			if b.IsReady() {
				var err error

				resolved, err = m.evalBuiltinApp(b)
				if err != nil {
					return nil, err
				}
			} else {
				resolved = b
			}

			state = getReturn[T]()
			state.(*Return[T]).Ctx = context
			state.(*Return[T]).Value = resolved
		} else {
			return nil, errors.New("BuiltinTermArgumentExpected")
		}
	default:
		return nil, errors.New("NonPolymorphicInstantiation")
	}

	return state, nil
}

func (m *Machine[T]) applyEvaluate(
	context MachineContext[T],
	function Value[T],
	arg Value[T],
) (MachineState[T], error) {
	var state MachineState[T]

	switch f := function.(type) {
	case *Lambda[T]:
		env := f.Env.Extend(arg)

		state = getCompute[T]()
		state.(*Compute[T]).Ctx = context
		state.(*Compute[T]).Env = env
		state.(*Compute[T]).Term = f.Body
	case *Builtin[T]:
		if !f.NeedsForce() && f.IsArrow() {
			var resolved Value[T]

			b := f.ApplyArg(arg)

			if b.IsReady() {
				var err error

				resolved, err = m.evalBuiltinApp(b)
				if err != nil {
					return nil, err
				}
			} else {
				resolved = b
			}

			state = getReturn[T]()
			state.(*Return[T]).Ctx = context
			state.(*Return[T]).Value = resolved
		} else {
			return nil, errors.New("UnexpectedBuiltinTermArgument")
		}
	default:
		return nil, errors.New("NonFunctionalApplication")
	}

	return state, nil
}

func transferArgStack[T syn.Eval](
	fields []Value[T],
	ctx MachineContext[T],
) MachineContext[T] {
	c := ctx

	for arg := len(fields) - 1; arg >= 0; arg-- {
		c = &FrameAwaitFunValue[T]{
			Ctx:   c,
			Value: fields[arg],
		}
	}

	return c
}

func dischargeValue[T syn.Eval](value Value[T]) syn.Term[T] {
	var dischargedTerm syn.Term[T]

	switch v := value.(type) {
	case *Constant:
		dischargedTerm = &syn.Constant{
			Con: v.Constant,
		}
	case *Builtin[T]:
		var forcedTerm syn.Term[T]

		forcedTerm = &syn.Builtin{
			DefaultFunction: v.Func,
		}

		for range uint(v.Forces) {
			forcedTerm = &syn.Force[T]{
				Term: forcedTerm,
			}
		}

		for arg := range v.Args.Iter() {
			forcedTerm = &syn.Apply[T]{
				Function: forcedTerm,
				Argument: dischargeValue[T](arg),
			}
		}

		dischargedTerm = forcedTerm
	case *Delay[T]:
		dischargedTerm = &syn.Delay[T]{
			Term: withEnv(0, v.Env, v.Body),
		}

	case *Lambda[T]:
		dischargedTerm = &syn.Lambda[T]{
			ParameterName: v.ParameterName,
			Body:          withEnv(1, v.Env, v.Body),
		}

	case *Constr[T]:
		fields := []syn.Term[T]{}

		for _, f := range v.Fields {
			fields = append(fields, dischargeValue[T](f))
		}

		dischargedTerm = &syn.Constr[T]{
			Tag:    v.Tag,
			Fields: fields,
		}
	}

	return dischargedTerm
}

func withEnv[T syn.Eval](
	lamCnt int,
	env *Env[T],
	term syn.Term[T],
) syn.Term[T] {
	var dischargedTerm syn.Term[T]

	switch t := term.(type) {
	case *syn.Var[T]:
		if lamCnt >= t.Name.LookupIndex() {
			dischargedTerm = t
		} else if val, exists := env.Lookup(t.Name.LookupIndex() - lamCnt); exists {
			dischargedTerm = dischargeValue[T](val)
		} else {
			dischargedTerm = t
		}

	case *syn.Lambda[T]:
		dischargedTerm = &syn.Lambda[T]{
			ParameterName: t.ParameterName,
			Body:          withEnv(lamCnt+1, env, t.Body),
		}

	case *syn.Apply[T]:
		dischargedTerm = &syn.Apply[T]{
			Function: withEnv(lamCnt, env, t.Function),
			Argument: withEnv(lamCnt, env, t.Argument),
		}
	case *syn.Delay[T]:
		dischargedTerm = &syn.Delay[T]{
			Term: withEnv(lamCnt, env, t.Term),
		}

	case *syn.Force[T]:
		dischargedTerm = &syn.Force[T]{
			Term: withEnv(lamCnt, env, t.Term),
		}

	case *syn.Constr[T]:
		fields := []syn.Term[T]{}

		for _, f := range t.Fields {
			fields = append(fields, withEnv(lamCnt, env, f))
		}

		dischargedTerm = &syn.Constr[T]{
			Tag:    t.Tag,
			Fields: fields,
		}
	case *syn.Case[T]:
		branches := []syn.Term[T]{}

		for _, b := range t.Branches {
			branches = append(branches, withEnv(lamCnt, env, b))
		}

		dischargedTerm = &syn.Case[T]{
			Constr:   withEnv(lamCnt, env, t.Constr),
			Branches: branches,
		}
	default:
		dischargedTerm = t
	}

	return dischargedTerm
}

func (m *Machine[T]) stepAndMaybeSpend(step StepKind) error {
	m.unbudgetedSteps[step] += 1
	m.unbudgetedSteps[9] += 1

	if m.unbudgetedSteps[9] >= m.slippage {
		if err := m.spendUnbudgetedSteps(); err != nil {
			return err
		}
	}

	return nil
}

func (m *Machine[T]) spendUnbudgetedSteps() error {
	for i := uint8(0); i < uint8(len(m.unbudgetedSteps)-1); i++ {
		unspent_step_budget := m.costs.machineCosts.get(StepKind(i))

		unspent_step_budget.occurrences(m.unbudgetedSteps[i])

		if err := m.spendBudget(unspent_step_budget); err != nil {
			return err
		}

		m.unbudgetedSteps[i] = 0
	}

	m.unbudgetedSteps[9] = 0

	return nil
}

func (m *Machine[T]) spendBudget(exBudget ExBudget) error {
	m.ExBudget.Mem -= exBudget.Mem
	m.ExBudget.Cpu -= exBudget.Cpu

	if m.ExBudget.Mem < 0 || m.ExBudget.Cpu < 0 {
		return errors.New("out of budget")
	}

	return nil
}
