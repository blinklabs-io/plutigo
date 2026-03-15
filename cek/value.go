package cek

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/syn"
)

type Value[T syn.Eval] interface {
	fmt.Stringer
	toExMem() ExMem
	isValue()
}

type Constant struct {
	Constant syn.IConstant
}

const (
	cachedIntMin = -256
	cachedIntMax = 65535
)

var (
	cachedBoolFalse = &Constant{
		Constant: &syn.Bool{Inner: false},
	}
	cachedBoolTrue = &Constant{
		Constant: &syn.Bool{Inner: true},
	}
	cachedUnitConstant = &Constant{
		Constant: &syn.Unit{},
	}
	cachedIntConstants = buildCachedIntConstants()
)

func boolConstant(v bool) *Constant {
	if v {
		return cachedBoolTrue
	}
	return cachedBoolFalse
}

// int64Constant reuses immutable small constants to avoid repeated big.Int and
// syn.Integer allocations on hot integer builtin paths.
func int64Constant(v int64) *Constant {
	if v >= cachedIntMin && v <= cachedIntMax {
		return cachedIntConstants[v-cachedIntMin]
	}
	return &Constant{
		Constant: &syn.Integer{Inner: big.NewInt(v)},
	}
}

func (m *Machine[T]) int64Constant(v int64) *Constant {
	if v >= cachedIntMin && v <= cachedIntMax {
		return cachedIntConstants[v-cachedIntMin]
	}
	if cached := m.dynamicIntConstants[v]; cached != nil {
		return cached
	}

	constant := &Constant{
		Constant: &syn.Integer{Inner: big.NewInt(v)},
	}
	if len(m.dynamicIntConstants) < int64ConstantCacheCap {
		m.dynamicIntConstants[v] = constant
	}
	return constant
}

func machineConstantValue[T syn.Eval](m *Machine[T], constant syn.IConstant) *Constant {
	switch c := constant.(type) {
	case *syn.Bool:
		return boolConstant(c.Inner)
	case *syn.Unit:
		return cachedUnitConstant
	case *syn.Integer:
		if c.Inner.IsInt64() {
			return m.int64Constant(c.Inner.Int64())
		}
	}
	return m.allocConstant(constant)
}

func constantTerm[T syn.Eval](constant syn.IConstant) syn.Term[T] {
	return &syn.Constant{Con: cloneConstant(constant)}
}

func (c Constant) String() string {
	return fmt.Sprintf("%v", c.Constant)
}

func (Constant) isValue() {}

func (c Constant) toExMem() ExMem {
	return iconstantExMem(c.Constant)()
}

func buildCachedIntConstants() []*Constant {
	ret := make([]*Constant, cachedIntMax-cachedIntMin+1)
	for i := int64(cachedIntMin); i <= cachedIntMax; i++ {
		ret[i-cachedIntMin] = &Constant{
			Constant: &syn.Integer{Inner: big.NewInt(i)},
		}
	}
	return ret
}

func cloneConstant(constant syn.IConstant) syn.IConstant {
	switch c := constant.(type) {
	case *syn.Integer:
		return &syn.Integer{Inner: new(big.Int).Set(c.Inner)}
	case *syn.ByteString:
		return &syn.ByteString{Inner: bytes.Clone(c.Inner)}
	case *syn.String:
		return &syn.String{Inner: c.Inner}
	case *syn.Unit:
		return &syn.Unit{}
	case *syn.Bool:
		return &syn.Bool{Inner: c.Inner}
	case *syn.ProtoList:
		items := make([]syn.IConstant, len(c.List))
		for i := range c.List {
			items[i] = cloneConstant(c.List[i])
		}
		return &syn.ProtoList{
			LTyp: c.LTyp,
			List: items,
		}
	case *syn.ProtoPair:
		return &syn.ProtoPair{
			FstType: c.FstType,
			SndType: c.SndType,
			First:   cloneConstant(c.First),
			Second:  cloneConstant(c.Second),
		}
	case *syn.Data:
		return &syn.Data{Inner: c.Inner.Clone()}
	case *syn.Bls12_381G1Element:
		if c.Inner == nil {
			return &syn.Bls12_381G1Element{}
		}
		tmp := *c.Inner
		return &syn.Bls12_381G1Element{Inner: &tmp}
	case *syn.Bls12_381G2Element:
		if c.Inner == nil {
			return &syn.Bls12_381G2Element{}
		}
		tmp := *c.Inner
		return &syn.Bls12_381G2Element{Inner: &tmp}
	case *syn.Bls12_381MlResult:
		if c.Inner == nil {
			return &syn.Bls12_381MlResult{}
		}
		tmp := *c.Inner
		return &syn.Bls12_381MlResult{Inner: &tmp}
	default:
		panic(fmt.Sprintf("unsupported constant type: %T", constant))
	}
}

type Delay[T syn.Eval] struct {
	Body syn.Term[T]
	Env  *Env[T]
}

func (d Delay[T]) String() string {
	return fmt.Sprintf("Delay[%T]", d.Body)
}

func (Delay[T]) isValue() {}

func (Delay[T]) toExMem() ExMem {
	return ExMem(1)
}

type Lambda[T syn.Eval] struct {
	ParameterName T
	Body          syn.Term[T]
	Env           *Env[T]
}

func (l Lambda[T]) String() string {
	return fmt.Sprintf("Lambda[%v]", l.ParameterName)
}

func (l Lambda[T]) isValue() {}

func (Lambda[T]) toExMem() ExMem {
	return ExMem(1)
}

type Builtin[T syn.Eval] struct {
	Func     builtin.DefaultFunction
	Forces   uint
	ArgCount uint
	Args     *BuiltinArgs[T]
}

func (b Builtin[T]) String() string {
	return fmt.Sprintf("Builtin[%d args, %d forces]", b.ArgCount, b.Forces)
}

func (b Builtin[T]) isValue() {}

func (b Builtin[T]) toExMem() ExMem {
	return ExMem(1)
}

func (b Builtin[T]) NeedsForce() bool {
	return b.Func.ForceCount() > b.Forces
}

func (b *Builtin[T]) ConsumeForce() *Builtin[T] {
	return &Builtin[T]{
		Func:     b.Func,
		Forces:   b.Forces + 1,
		ArgCount: b.ArgCount,
		Args:     b.Args,
	}
}

func (b *Builtin[T]) ApplyArg(arg Value[T]) *Builtin[T] {
	return &Builtin[T]{
		Func:     b.Func,
		Forces:   b.Forces,
		ArgCount: b.ArgCount + 1,
		Args:     b.Args.Extend(arg),
	}
}

func (b *Builtin[T]) IsReady() bool {
	return b.Func.Arity() == b.ArgCount && b.Func.ForceCount() == b.Forces
}

func (b *Builtin[T]) IsArrow() bool {
	return b.Func.Arity() > b.ArgCount
}

type Constr[T syn.Eval] struct {
	Tag    uint
	Fields []Value[T]
}

func (c Constr[T]) String() string {
	return fmt.Sprintf("Constr[tag=%d, fields=%d]", c.Tag, len(c.Fields))
}

func (c Constr[T]) isValue() {}

func (c Constr[T]) toExMem() ExMem {
	return ExMem(1)
}
