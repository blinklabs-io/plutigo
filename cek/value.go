package cek

import (
	"bytes"
	"fmt"
	"math/big"
	"sync"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
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
	cachedIntMin = -64
	cachedIntMax = 255
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
	sharedDynamicIntMu sync.RWMutex
	sharedDynamicInts  map[int64]*Constant
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
	return newDynamicIntConstant(v)
}

func newDynamicIntConstant(v int64) *Constant {
	integer := &syn.Integer{}
	integer.SetInner(big.NewInt(v))
	return &Constant{
		Constant: integer,
	}
}

func loadSharedDynamicIntConstant(v int64) *Constant {
	sharedDynamicIntMu.RLock()
	cached := sharedDynamicInts[v]
	sharedDynamicIntMu.RUnlock()
	return cached
}

func storeSharedDynamicIntConstant(v int64, constant *Constant) *Constant {
	sharedDynamicIntMu.Lock()
	defer sharedDynamicIntMu.Unlock()
	if cached := sharedDynamicInts[v]; cached != nil {
		return cached
	}
	if sharedDynamicInts == nil || len(sharedDynamicInts) >= int64ConstantCacheCap {
		sharedDynamicInts = make(map[int64]*Constant, 64)
	}
	sharedDynamicInts[v] = constant
	return constant
}

func (m *Machine[T]) int64Constant(v int64) *Constant {
	if v >= cachedIntMin && v <= cachedIntMax {
		return cachedIntConstants[v-cachedIntMin]
	}
	if cached := loadSharedDynamicIntConstant(v); cached != nil {
		return cached
	}
	if m.dynamicIntConstants != nil {
		if cached := m.dynamicIntConstants[v]; cached != nil {
			return cached
		}
	}

	constant := newDynamicIntConstant(v)
	if shared := storeSharedDynamicIntConstant(v, constant); shared != nil {
		return shared
	}
	if m.dynamicIntConstants == nil {
		m.dynamicIntConstants = make(map[int64]*Constant, 64)
	}
	if len(m.dynamicIntConstants) < int64ConstantCacheCap {
		m.dynamicIntConstants[v] = constant
	}
	return constant
}

func machineConstantValue[T syn.Eval](m *Machine[T], constant syn.IConstant) Value[T] {
	switch c := constant.(type) {
	case *syn.Bool:
		return boolConstant(c.Inner)
	case *syn.Unit:
		return cachedUnitConstant
	case *syn.Integer:
		if v, ok := c.CachedInt64(); ok {
			return m.int64Constant(v)
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
	if integer, ok := c.Constant.(*syn.Integer); ok {
		return ExMem(integer.ExMemWords())
	}
	return iconstantExMem(c.Constant)()
}

type dataListValue[T syn.Eval] struct {
	items []data.PlutusData
}

func (d dataListValue[T]) String() string {
	return fmt.Sprintf("DataList[%d]", len(d.items))
}

func (dataListValue[T]) isValue() {}

func (d dataListValue[T]) toExMem() ExMem {
	acc := ExMem(NilCost)
	for _, item := range d.items {
		acc += dataExMem(item)() + ConsCost
	}
	return acc
}

type dataValue[T syn.Eval] struct {
	item data.PlutusData
}

func (d dataValue[T]) String() string {
	return fmt.Sprintf("%v", d.item)
}

func (dataValue[T]) isValue() {}

func (d dataValue[T]) toExMem() ExMem {
	return dataExMem(d.item)()
}

type dataMapValue[T syn.Eval] struct {
	items [][2]data.PlutusData
}

func (d dataMapValue[T]) String() string {
	return fmt.Sprintf("DataMapList[%d]", len(d.items))
}

func (dataMapValue[T]) isValue() {}

func (d dataMapValue[T]) toExMem() ExMem {
	acc := ExMem(NilCost)
	for _, item := range d.items {
		acc += ExMem(PairCost) + dataExMem(item[0])() + dataExMem(item[1])() + ConsCost
	}
	return acc
}

type pairValue[T syn.Eval] struct {
	first  Value[T]
	second Value[T]
}

func (p pairValue[T]) String() string {
	return fmt.Sprintf("Pair[%T,%T]", p.first, p.second)
}

func (pairValue[T]) isValue() {}

func (p pairValue[T]) toExMem() ExMem {
	return ExMem(PairCost) + p.first.toExMem() + p.second.toExMem()
}

func buildCachedIntConstants() []*Constant {
	ret := make([]*Constant, cachedIntMax-cachedIntMin+1)
	for i := int64(cachedIntMin); i <= cachedIntMax; i++ {
		integer := &syn.Integer{}
		integer.SetInner(big.NewInt(i))
		ret[i-cachedIntMin] = &Constant{
			Constant: integer,
		}
	}
	return ret
}

func materializeDataListConstant(items []data.PlutusData) *syn.ProtoList {
	list := make([]syn.IConstant, len(items))
	for i, item := range items {
		list[i] = &syn.Data{Inner: item}
	}
	return &syn.ProtoList{
		LTyp: sharedDataType,
		List: list,
	}
}

func materializeDataMapConstant(items [][2]data.PlutusData) *syn.ProtoList {
	list := make([]syn.IConstant, len(items))
	for i, item := range items {
		list[i] = &syn.ProtoPair{
			FstType: sharedDataType,
			SndType: sharedDataType,
			First:   &syn.Data{Inner: item[0]},
			Second:  &syn.Data{Inner: item[1]},
		}
	}
	return &syn.ProtoList{
		LTyp: sharedPairDataType,
		List: list,
	}
}

func materializeConstantValue[T syn.Eval](
	value Value[T],
) (syn.IConstant, bool, error) {
	return materializeConstantValueDepth[T](
		value,
		0,
		maxDischargeDepth,
	)
}

func materializeConstantValueDepth[T syn.Eval](
	value Value[T],
	depth int,
	maxDepth int,
) (syn.IConstant, bool, error) {
	if depth > maxDepth {
		return nil, false, dischargeDepthLimitError()
	}

	switch v := value.(type) {
	case *Constant:
		return v.Constant, true, nil
	case *dataValue[T]:
		return &syn.Data{Inner: v.item}, true, nil
	case *dataListValue[T]:
		return materializeDataListConstant(v.items), true, nil
	case *dataMapValue[T]:
		return materializeDataMapConstant(v.items), true, nil
	case *pairValue[T]:
		first, ok, err := materializeConstantValueDepth[T](
			v.first,
			depth+1,
			maxDepth,
		)
		if err != nil {
			return nil, false, err
		}
		if !ok || first == nil {
			return nil, false, nil
		}
		second, ok, err := materializeConstantValueDepth[T](
			v.second,
			depth+1,
			maxDepth,
		)
		if err != nil {
			return nil, false, err
		}
		if !ok || second == nil {
			return nil, false, nil
		}
		return &syn.ProtoPair{
			FstType: first.Typ(),
			SndType: second.Typ(),
			First:   first,
			Second:  second,
		}, true, nil
	default:
		return nil, false, nil
	}
}

func cloneConstant(constant syn.IConstant) syn.IConstant {
	switch c := constant.(type) {
	case *syn.Integer:
		ret := &syn.Integer{}
		if c.Inner == nil {
			ret.SetInner(nil)
			return ret
		}
		ret.SetInner(new(big.Int).Set(c.Inner))
		return ret
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
	AST *syn.Delay[T]
	Env *Env[T]
}

func (d Delay[T]) String() string {
	if d.AST == nil || d.AST.Term == nil {
		return "Delay[<nil>]"
	}
	return fmt.Sprintf("Delay[%T]", d.AST.Term)
}

func (Delay[T]) isValue() {}

func (Delay[T]) toExMem() ExMem {
	return ExMem(1)
}

type Lambda[T syn.Eval] struct {
	AST *syn.Lambda[T]
	Env *Env[T]
}

func (l Lambda[T]) String() string {
	if l.AST == nil {
		return "Lambda[<nil>]"
	}
	return fmt.Sprintf("Lambda[%v]", l.AST.ParameterName)
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
