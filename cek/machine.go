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
		New: func() any {
			return &Compute[syn.DeBruijn]{}
		},
	}
	returnPool = sync.Pool{
		New: func() any {
			return &Return[syn.DeBruijn]{}
		},
	}
	donePool = sync.Pool{
		New: func() any {
			return &Done[syn.DeBruijn]{}
		},
	}
)

// getCompute returns a Compute state from the pool
func getCompute[T syn.Eval]() *Compute[T] {
	c := computePool.Get().(*Compute[syn.DeBruijn])
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
	return (*Return[T])(unsafe.Pointer(r))
}

// putReturn returns a Return state to the pool
func putReturn[T syn.Eval](r *Return[T]) {
	// Reset the state
	r.Ctx = nil
	r.Value = nil
	returnPool.Put((*Return[syn.DeBruijn])(unsafe.Pointer(r)))
}

// getDone returns a Done state from the pool
func getDone[T syn.Eval]() *Done[T] {
	d := donePool.Get().(*Done[syn.DeBruijn])
	return (*Done[T])(unsafe.Pointer(d))
}

// putDone returns a Done state to the pool
func putDone[T syn.Eval](d *Done[T]) {
	// Reset the state
	d.term = nil
	donePool.Put((*Done[syn.DeBruijn])(unsafe.Pointer(d)))
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

// Run executes a Plutus term using the CEK (Control, Environment, Kontinuation) abstract machine.
// The CEK machine is a small-step operational semantics for evaluating functional programs.
// It maintains three components:
// - Control (C): the current term being evaluated
// - Environment (E): mapping from variables to values
// - Kontinuation (K): represents the evaluation context/stack
//
// The algorithm proceeds by repeatedly transitioning between three states:
// - Compute: evaluate the current term in the current environment and context
// - Return: handle a computed value in the current context
// - Done: evaluation complete, return the final result
//
// This implementation uses object pooling for performance optimization, reusing
// Compute/Return/Done state objects to minimize garbage collection pressure.
func (m *Machine[T]) Run(term syn.Term[T]) (syn.Term[T], error) {
	// Spend initial startup budget for machine initialization
	startupBudget := m.costs.machineCosts.startup
	if err := m.spendBudget(startupBudget); err != nil {
		return nil, err
	}

	var state MachineState[T]

	// Initialize with a Compute state: evaluate the input term with empty environment
	// and no continuation context (NoFrame)
	comp := getCompute[T]()
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
				putCompute(v) // Return object to pool
				return nil, err
			}
			if newState == nil {
				putCompute(v) // Return object to pool
				return nil, errors.New("compute returned nil state")
			}
			putCompute(v) // Return object to pool
			state = newState
		case *Return[T]:
			// Return state: handle a computed value in current context
			newState, err := m.returnCompute(v.Ctx, v.Value)
			if err != nil {
				putReturn(v) // Return object to pool
				return nil, err
			}
			if newState == nil {
				putReturn(v) // Return object to pool
				return nil, errors.New("returnCompute returned nil state")
			}
			putReturn(v) // Return object to pool
			state = newState
		case *Done[T]:
			// Done state: evaluation complete, extract final result
			term := v.term
			putDone(v) // Return object to pool
			return term, nil
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

		value, exists := env.Lookup(t.Name.LookupIndex())

		if !exists {
			return nil, errors.New("open term evaluated")
		}

		// Transition to Return state with the looked-up value
		ret := getReturn[T]()
		ret.Ctx = context
		ret.Value = value
		state = ret
	case *syn.Delay[T]:
		// Delay creates a suspended computation that can be forced later
		if err := m.stepAndMaybeSpend(ExDelay); err != nil {
			return nil, err
		}

		value := &Delay[T]{
			Body: t.Term,
			Env:  env, // Capture current environment for later evaluation
		}

		ret := getReturn[T]()
		ret.Ctx = context
		ret.Value = value
		state = ret
	case *syn.Lambda[T]:
		// Lambda creates a closure capturing the current environment
		if err := m.stepAndMaybeSpend(ExLambda); err != nil {
			return nil, err
		}

		value := &Lambda[T]{
			ParameterName: t.ParameterName,
			Body:          t.Body,
			Env:           env, // Capture environment for closure
		}

		ret := getReturn[T]()
		ret.Ctx = context
		ret.Value = value
		state = ret
	case *syn.Apply[T]:
		// Application: evaluate function term first, then argument
		// Uses FrameAwaitFunTerm to remember argument for later evaluation
		if err := m.stepAndMaybeSpend(ExApply); err != nil {
			return nil, err
		}

		frame := &FrameAwaitFunTerm[T]{
			Env:  env,
			Term: t.Argument, // Remember argument to evaluate later
			Ctx:  context,
		}

		comp := getCompute[T]()
		comp.Ctx = frame
		comp.Env = env
		comp.Term = t.Function // Evaluate function first
		state = comp
	case *syn.Constant:
		// Constants are already evaluated values
		if err := m.stepAndMaybeSpend(ExConstant); err != nil {
			return nil, err
		}

		ret := getReturn[T]()
		ret.Ctx = context
		ret.Value = &Constant{
			Constant: t.Con,
		}
		state = ret
	case *syn.Force[T]:
		// Force triggers evaluation of a delayed computation
		// Uses FrameForce to handle the result
		if err := m.stepAndMaybeSpend(ExForce); err != nil {
			return nil, err
		}

		frame := &FrameForce[T]{
			Ctx: context,
		}

		comp := getCompute[T]()
		comp.Ctx = frame
		comp.Env = env
		comp.Term = t.Term
		state = comp
	case *syn.Error:
		// Explicit error term - evaluation fails
		return nil, errors.New("error explicitly called")

	case *syn.Builtin:
		// Builtin functions are treated as values
		if err := m.stepAndMaybeSpend(ExBuiltin); err != nil {
			return nil, err
		}

		ret := getReturn[T]()
		ret.Ctx = context
		ret.Value = &Builtin[T]{
			Func:   t.DefaultFunction,
			Args:   nil,
			Forces: 0,
		}
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
			ret := getReturn[T]()
			ret.Ctx = context
			ret.Value = &Constr[T]{
				Tag:    t.Tag,
				Fields: []Value[T]{},
			}
			state = ret
		} else {
			// Evaluate fields sequentially using FrameConstr
			first_field := fields[0]

			rest := fields[1:]

			frame := &FrameConstr[T]{
				Ctx:            context,
				Tag:            t.Tag,
				Fields:         rest,         // Remaining fields to evaluate
				ResolvedFields: []Value[T]{}, // Accumulate evaluated fields
				Env:            env,
			}

			comp := getCompute[T]()
			comp.Ctx = frame
			comp.Env = env
			comp.Term = first_field // Evaluate first field
			state = comp
		}
	case *syn.Case[T]:
		// Case expression: evaluate scrutinee, then match against branches
		// Uses FrameCases to handle branching logic
		if err := m.stepAndMaybeSpend(ExCase); err != nil {
			return nil, err
		}

		frame := &FrameCases[T]{
			Env:      env,
			Ctx:      context,
			Branches: t.Branches,
		}

		comp := getCompute[T]()
		comp.Ctx = frame
		comp.Env = env
		comp.Term = t.Constr // Evaluate scrutinee
		state = comp
	default:
		panic(fmt.Sprintf("unknown term: %T: %v", term, term))
	}

	if state == nil {
		return nil, errors.New("compute: state is nil")
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
		if err != nil {
			return nil, err
		}
	case *FrameAwaitFunTerm[T]:
		// Function evaluated to a value, now evaluate argument term
		comp := getCompute[T]()
		comp.Ctx = &FrameAwaitArg[T]{
			Ctx:   c.Ctx,
			Value: value, // Function value
		}
		comp.Env = c.Env
		comp.Term = c.Term // Argument term to evaluate
		state = comp
	case *FrameAwaitFunValue[T]:
		// Argument evaluated to a value, now apply to function value
		state, err = m.applyEvaluate(c.Ctx, value, c.Value)
		if err != nil {
			return nil, err
		}
	case *FrameForce[T]:
		// Handle forcing of delayed computations or builtin applications
		state, err = m.forceEvaluate(c.Ctx, value)
		if err != nil {
			return nil, err
		}
	case *FrameConstr[T]:
		// Accumulate evaluated constructor fields
		resolvedFields := append(c.ResolvedFields, value)

		fields := c.Fields

		if len(fields) == 0 {
			// All fields evaluated, create constructor value
			ret := getReturn[T]()
			ret.Ctx = c.Ctx
			ret.Value = &Constr[T]{
				Tag:    c.Tag,
				Fields: resolvedFields,
			}
			state = ret
		} else {
			// More fields to evaluate
			first_field := fields[0]
			rest := fields[1:]

			frame := &FrameConstr[T]{
				Ctx:            c.Ctx,
				Tag:            c.Tag,
				Fields:         rest,
				ResolvedFields: resolvedFields,
				Env:            c.Env,
			}

			comp := getCompute[T]()
			comp.Ctx = frame
			comp.Env = c.Env
			comp.Term = first_field
			state = comp
		}
	case *FrameCases[T]:
		// Pattern match on constructor value
		switch v := value.(type) {
		case *Constr[T]:
			if v.Tag > math.MaxInt {
				return nil, errors.New("MaxIntExceeded")
			}
			if indexExists(c.Branches, int(v.Tag)) {
				// Matching branch found, evaluate it with arguments on stack
				comp := getCompute[T]()
				comp.Ctx = transferArgStack(v.Fields, c.Ctx)
				comp.Env = c.Env
				comp.Term = c.Branches[v.Tag]
				state = comp
			} else {
				return nil, errors.New("MissingCaseBranch")
			}
		default:
			return nil, errors.New("NonConstrScrutinized")
		}
	case *NoFrame:
		// No more continuations - evaluation complete
		// Spend any remaining unbudgeted steps before finishing
		if m.unbudgetedSteps[9] > 0 {
			if err := m.spendUnbudgetedSteps(); err != nil {
				return nil, err
			}
		}

		done := getDone[T]()
		done.term = dischargeValue[T](value)
		state = done
	default:
		panic(fmt.Sprintf("unknown context %v", context))
	}

	if state == nil {
		return nil, errors.New("returnCompute: state is nil")
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
		comp := getCompute[T]()
		comp.Ctx = context
		comp.Env = v.Env // Use environment captured at delay creation
		comp.Term = v.Body
		state = comp
	case *Builtin[T]:
		// Force a builtin function application
		if v.NeedsForce() {
			var resolved Value[T]

			b := v.ConsumeForce() // Consume one force

			if b.IsReady() {
				// Builtin has all arguments, evaluate it
				var err error

				resolved, err = m.evalBuiltinApp(b)
				if err != nil {
					return nil, err
				}
			} else {
				// Still needs more arguments/forces
				resolved = b
			}

			ret := getReturn[T]()
			ret.Ctx = context
			ret.Value = resolved
			state = ret
		} else {
			return nil, errors.New("BuiltinTermArgumentExpected")
		}
	default:
		return nil, errors.New("NonPolymorphicInstantiation")
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
		env := f.Env.Extend(arg)

		comp := getCompute[T]()
		comp.Ctx = context
		comp.Env = env // Extended environment with argument bound
		comp.Term = f.Body
		state = comp
	case *Builtin[T]:
		// Apply builtin function
		if !f.NeedsForce() && f.IsArrow() {
			var resolved Value[T]

			b := f.ApplyArg(arg) // Apply argument to builtin

			if b.IsReady() {
				// Builtin has all arguments, evaluate it
				var err error

				resolved, err = m.evalBuiltinApp(b)
				if err != nil {
					return nil, err
				}
			} else {
				// Still needs more arguments
				resolved = b
			}

			ret := getReturn[T]()
			ret.Ctx = context
			ret.Value = resolved
			state = ret
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
		dischargedTerm = &syn.Constant{
			Con: v.Constant,
		}
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
		} else if val, exists := env.Lookup(t.Name.LookupIndex() - lamCnt); exists {
			// Variable found in environment, discharge its value
			dischargedTerm = dischargeValue[T](val)
		} else {
			// Free variable (shouldn't happen in well-formed terms)
			dischargedTerm = t
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
		fields := []syn.Term[T]{}

		for _, f := range t.Fields {
			fields = append(fields, withEnv(lamCnt, env, f))
		}

		dischargedTerm = &syn.Constr[T]{
			Tag:    t.Tag,
			Fields: fields,
		}
	case *syn.Case[T]:
		// Case expression: process scrutinee and all branches
		branches := []syn.Term[T]{}

		for _, b := range t.Branches {
			branches = append(branches, withEnv(lamCnt, env, b))
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
	for i := range uint8(len(m.unbudgetedSteps) - 1) {
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
