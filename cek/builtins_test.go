package cek

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/syn"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

// Helper functions to reduce test code duplication

func newTestMachine() *Machine[syn.DeBruijn] {
	return NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)
}

func newTestBuiltin(fn builtin.DefaultFunction) *Builtin[syn.DeBruijn] {
	return &Builtin[syn.DeBruijn]{
		Func:     fn,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}
}

func evalBuiltin(
	t *testing.T,
	m *Machine[syn.DeBruijn],
	b *Builtin[syn.DeBruijn],
) Value[syn.DeBruijn] {
	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}
	return val
}

func expectConstant(t *testing.T, val Value[syn.DeBruijn]) *Constant {
	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}
	return constVal
}

func expectInteger(t *testing.T, constVal *Constant) *syn.Integer {
	i, ok := constVal.Constant.(*syn.Integer)
	if !ok {
		t.Fatalf("expected Integer constant, got %T", constVal.Constant)
	}
	return i
}

func expectBool(t *testing.T, constVal *Constant) *syn.Bool {
	bl, ok := constVal.Constant.(*syn.Bool)
	if !ok {
		t.Fatalf("expected Bool constant, got %T", constVal.Constant)
	}
	return bl
}

func expectByteString(t *testing.T, constVal *Constant) *syn.ByteString {
	bs, ok := constVal.Constant.(*syn.ByteString)
	if !ok {
		t.Fatalf("expected ByteString constant, got %T", constVal.Constant)
	}
	return bs
}

func expectString(t *testing.T, constVal *Constant) *syn.String {
	str, ok := constVal.Constant.(*syn.String)
	if !ok {
		t.Fatalf("expected String constant, got %T", constVal.Constant)
	}
	return str
}

func expectData(t *testing.T, constVal *Constant) *syn.Data {
	dataConst, ok := constVal.Constant.(*syn.Data)
	if !ok {
		t.Fatalf("expected Data constant, got %T", constVal.Constant)
	}
	return dataConst
}

func expectProtoList(t *testing.T, constVal *Constant) *syn.ProtoList {
	list, ok := constVal.Constant.(*syn.ProtoList)
	if !ok {
		t.Fatalf("expected ProtoList constant, got %T", constVal.Constant)
	}
	return list
}

func expectProtoPair(t *testing.T, constVal *Constant) *syn.ProtoPair {
	pair, ok := constVal.Constant.(*syn.ProtoPair)
	if !ok {
		t.Fatalf("expected ProtoPair constant, got %T", constVal.Constant)
	}
	return pair
}

func expectBlsG1(t *testing.T, constVal *Constant) *syn.Bls12_381G1Element {
	g1, ok := constVal.Constant.(*syn.Bls12_381G1Element)
	if !ok {
		t.Fatalf(
			"expected Bls12_381G1Element constant, got %T",
			constVal.Constant,
		)
	}
	return g1
}

func expectBlsG2(t *testing.T, constVal *Constant) *syn.Bls12_381G2Element {
	g2, ok := constVal.Constant.(*syn.Bls12_381G2Element)
	if !ok {
		t.Fatalf(
			"expected Bls12_381G2Element constant, got %T",
			constVal.Constant,
		)
	}
	return g2
}

func expectBlsMlResult(
	t *testing.T,
	constVal *Constant,
) *syn.Bls12_381MlResult {
	ml, ok := constVal.Constant.(*syn.Bls12_381MlResult)
	if !ok {
		t.Fatalf(
			"expected Bls12_381MlResult constant, got %T",
			constVal.Constant,
		)
	}
	return ml
}

func TestAddIntegerBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.AddInteger)

	v1 := &Constant{&syn.Integer{Inner: big.NewInt(10)}}
	v2 := &Constant{&syn.Integer{Inner: big.NewInt(32)}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	i := expectInteger(t, constVal)

	if i.Inner.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("expected 42, got %v", i.Inner)
	}
}

func TestCostAccounting(t *testing.T) {
	m := newTestMachine()
	orig := m.ExBudget

	// Call CostOne with a function that returns 1
	fn := builtin.IfThenElse
	err := m.CostOne(&fn, func() ExMem { return ExMem(1) })
	if err != nil {
		t.Fatalf("CostOne returned error: %v", err)
	}

	if m.ExBudget.Cpu >= orig.Cpu && m.ExBudget.Mem >= orig.Mem {
		t.Fatalf(
			"expected budget to decrease from %+v to %+v",
			orig,
			m.ExBudget,
		)
	}

	// Exhaust budget
	m.ExBudget = ExBudget{Mem: 0, Cpu: 0}
	fn2 := builtin.IfThenElse
	err = m.CostOne(&fn2, func() ExMem { return ExMem(1000) })
	if err == nil {
		t.Fatalf("expected out of budget error, got nil")
	}
}

func TestSubtractIntegerBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.SubtractInteger)

	v1 := &Constant{&syn.Integer{Inner: big.NewInt(10)}}
	v2 := &Constant{&syn.Integer{Inner: big.NewInt(3)}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	i := expectInteger(t, constVal)

	if i.Inner.Cmp(big.NewInt(7)) != 0 {
		t.Fatalf("expected 7, got %v", i.Inner)
	}
}

func TestMultiplyIntegerBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.MultiplyInteger)

	v1 := &Constant{&syn.Integer{Inner: big.NewInt(6)}}
	v2 := &Constant{&syn.Integer{Inner: big.NewInt(7)}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	i := expectInteger(t, constVal)

	if i.Inner.Cmp(big.NewInt(42)) != 0 {
		t.Fatalf("expected 42, got %v", i.Inner)
	}
}

func TestEqualsIntegerBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		int1     int64
		int2     int64
		expected bool
	}{
		{"equal positive", 5, 5, true},
		{"equal zero", 0, 0, true},
		{"equal negative", -3, -3, true},
		{"unequal positive", 5, 7, false},
		{"unequal with zero", 0, 1, false},
		{"unequal negative", -3, -5, false},
		{"positive vs negative", 3, -3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.EqualsInteger)

			v1 := &Constant{&syn.Integer{Inner: big.NewInt(tt.int1)}}
			v2 := &Constant{&syn.Integer{Inner: big.NewInt(tt.int2)}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			bl := expectBool(t, constVal)

			if bl.Inner != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, bl.Inner)
			}
		})
	}
}

func TestAppendByteStringBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.AppendByteString)

	v1 := &Constant{&syn.ByteString{Inner: []byte("hello")}}
	v2 := &Constant{&syn.ByteString{Inner: []byte("world")}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	bs := expectByteString(t, constVal)

	expected := []byte("helloworld")
	if !bytes.Equal(bs.Inner, expected) {
		t.Fatalf("expected %v, got %v", expected, bs.Inner)
	}
}

func TestEqualsByteStringBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		bs1      []byte
		bs2      []byte
		expected bool
	}{
		{"equal byte strings", []byte("test"), []byte("test"), true},
		{"unequal byte strings", []byte("test"), []byte("different"), false},
		{"empty byte strings", []byte(""), []byte(""), true},
		{"one empty", []byte("test"), []byte(""), false},
		{"different lengths", []byte("abc"), []byte("abcd"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.EqualsByteString)

			v1 := &Constant{&syn.ByteString{Inner: tt.bs1}}
			v2 := &Constant{&syn.ByteString{Inner: tt.bs2}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			bl := expectBool(t, constVal)

			if bl.Inner != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, bl.Inner)
			}
		})
	}
}

func TestLengthOfByteStringBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.LengthOfByteString)

	v1 := &Constant{&syn.ByteString{Inner: []byte("abc")}}

	b = b.ApplyArg(v1)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	i := expectInteger(t, constVal)

	if i.Inner.Int64() != 3 {
		t.Fatalf("expected 3, got %v", i.Inner)
	}
}

func TestEqualsDataBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.EqualsData,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Create two identical integer data
	d1 := &syn.Data{Inner: &data.Integer{Inner: big.NewInt(123)}}
	d2 := &syn.Data{Inner: &data.Integer{Inner: big.NewInt(123)}}

	v1 := &Constant{d1}
	v2 := &Constant{d2}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	bl, ok := constVal.Constant.(*syn.Bool)
	if !ok {
		t.Fatalf("expected Bool constant, got %T", constVal.Constant)
	}

	if !bl.Inner {
		t.Fatalf("expected true, got %v", bl.Inner)
	}
}

func TestUnConstrDataBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.UnConstrData,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Create a constr data
	constr := &data.Constr{
		Tag:    1,
		Fields: []data.PlutusData{&data.Integer{Inner: big.NewInt(99)}},
	}
	d := &syn.Data{Inner: constr}

	v1 := &Constant{d}

	b = b.ApplyArg(v1)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	// UnConstrData returns a ProtoPair constant
	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	pair, ok := constVal.Constant.(*syn.ProtoPair)
	if !ok {
		t.Fatalf("expected ProtoPair constant, got %T", constVal.Constant)
	}

	if pair.FstType == nil || pair.SndType == nil {
		t.Fatal("expected non-nil types in pair")
	}
}

func TestAppendStringBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.AppendString,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	v1 := &Constant{&syn.String{Inner: "hello"}}
	v2 := &Constant{&syn.String{Inner: " world"}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	str, ok := constVal.Constant.(*syn.String)
	if !ok {
		t.Fatalf("expected String constant, got %T", constVal.Constant)
	}

	if str.Inner != "hello world" {
		t.Fatalf("expected 'hello world', got %v", str.Inner)
	}
}

func TestEqualsStringBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		str1     string
		str2     string
		expected bool
	}{
		{"equal strings", "test", "test", true},
		{"unequal strings", "test", "different", false},
		{"empty strings", "", "", true},
		{"one empty", "test", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.EqualsString)

			v1 := &Constant{&syn.String{Inner: tt.str1}}
			v2 := &Constant{&syn.String{Inner: tt.str2}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			bl := expectBool(t, constVal)

			if bl.Inner != tt.expected {
				t.Fatalf("expected %v, got %v", tt.expected, bl.Inner)
			}
		})
	}
}

func TestSha2_256Builtin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Sha2_256,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	input := []byte("hello world")
	v1 := &Constant{&syn.ByteString{Inner: input}}

	b = b.ApplyArg(v1)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	bs, ok := constVal.Constant.(*syn.ByteString)
	if !ok {
		t.Fatalf("expected ByteString constant, got %T", constVal.Constant)
	}

	// SHA256 of "hello world" should be 32 bytes
	if len(bs.Inner) != 32 {
		t.Fatalf("expected 32-byte hash, got %d bytes", len(bs.Inner))
	}

	// Test with empty string
	b2 := &Builtin[syn.DeBruijn]{
		Func:     builtin.Sha2_256,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	v2 := &Constant{&syn.ByteString{Inner: []byte("")}}
	b2 = b2.ApplyArg(v2)

	val2, err := m.evalBuiltinApp(b2)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal2, ok := val2.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val2)
	}

	bs2, ok := constVal2.Constant.(*syn.ByteString)
	if !ok {
		t.Fatalf("expected ByteString constant, got %T", constVal2.Constant)
	}

	if len(bs2.Inner) != 32 {
		t.Fatalf("expected 32-byte hash, got %d bytes", len(bs2.Inner))
	}
}

func TestSha3_256Builtin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Sha3_256,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	input := []byte("hello world")
	v1 := &Constant{&syn.ByteString{Inner: input}}

	b = b.ApplyArg(v1)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	bs, ok := constVal.Constant.(*syn.ByteString)
	if !ok {
		t.Fatalf("expected ByteString constant, got %T", constVal.Constant)
	}

	// SHA3-256 should be 32 bytes
	if len(bs.Inner) != 32 {
		t.Fatalf("expected 32-byte hash, got %d bytes", len(bs.Inner))
	}
}

func TestHeadListBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.HeadList,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Create a list with two integers: [1, 2]
	list := &syn.ProtoList{
		LTyp: &syn.TInteger{},
		List: []syn.IConstant{
			&syn.Integer{Inner: big.NewInt(1)},
			&syn.Integer{Inner: big.NewInt(2)},
		},
	}

	v1 := &Constant{list}
	b = b.ApplyArg(v1)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	intConst, ok := constVal.Constant.(*syn.Integer)
	if !ok {
		t.Fatalf("expected Integer constant, got %T", constVal.Constant)
	}

	if intConst.Inner.Int64() != 1 {
		t.Fatalf("expected 1, got %v", intConst.Inner)
	}
}

func TestTailListBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.TailList,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Create a list with three integers: [1, 2, 3]
	list := &syn.ProtoList{
		LTyp: &syn.TInteger{},
		List: []syn.IConstant{
			&syn.Integer{Inner: big.NewInt(1)},
			&syn.Integer{Inner: big.NewInt(2)},
			&syn.Integer{Inner: big.NewInt(3)},
		},
	}

	v1 := &Constant{list}
	b = b.ApplyArg(v1)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	tailList, ok := constVal.Constant.(*syn.ProtoList)
	if !ok {
		t.Fatalf("expected ProtoList constant, got %T", constVal.Constant)
	}

	if len(tailList.List) != 2 {
		t.Fatalf("expected list with 2 elements, got %d", len(tailList.List))
	}

	elem1, ok := tailList.List[0].(*syn.Integer)
	if !ok {
		t.Fatalf("expected Integer element, got %T", tailList.List[0])
	}
	if elem1.Inner.Int64() != 2 {
		t.Fatalf("expected first element to be 2, got %v", elem1.Inner)
	}

	elem2, ok := tailList.List[1].(*syn.Integer)
	if !ok {
		t.Fatalf("expected Integer element, got %T", tailList.List[1])
	}
	if elem2.Inner.Int64() != 3 {
		t.Fatalf("expected second element to be 3, got %v", elem2.Inner)
	}
}

func TestNullListBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	// Test with non-empty list
	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.NullList,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	list := &syn.ProtoList{
		LTyp: &syn.TInteger{},
		List: []syn.IConstant{
			&syn.Integer{Inner: big.NewInt(1)},
		},
	}

	v1 := &Constant{list}
	b = b.ApplyArg(v1)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	bl, ok := constVal.Constant.(*syn.Bool)
	if !ok {
		t.Fatalf("expected Bool constant, got %T", constVal.Constant)
	}

	if bl.Inner {
		t.Fatalf("expected false for non-empty list, got %v", bl.Inner)
	}

	// Test with empty list
	b2 := &Builtin[syn.DeBruijn]{
		Func:     builtin.NullList,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	emptyList := &syn.ProtoList{
		LTyp: &syn.TInteger{},
		List: []syn.IConstant{},
	}

	v2 := &Constant{emptyList}
	b2 = b2.ApplyArg(v2)

	val2, err := m.evalBuiltinApp(b2)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal2, ok := val2.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val2)
	}

	bl2, ok := constVal2.Constant.(*syn.Bool)
	if !ok {
		t.Fatalf("expected Bool constant, got %T", constVal2.Constant)
	}

	if !bl2.Inner {
		t.Fatalf("expected true for empty list, got %v", bl2.Inner)
	}
}

func TestBlake2b_256Builtin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Blake2b_256,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	input := []byte("hello world")
	v1 := &Constant{&syn.ByteString{Inner: input}}

	b = b.ApplyArg(v1)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	bs, ok := constVal.Constant.(*syn.ByteString)
	if !ok {
		t.Fatalf("expected ByteString constant, got %T", constVal.Constant)
	}

	// BLAKE2b-256 should be 32 bytes
	if len(bs.Inner) != 32 {
		t.Fatalf("expected 32-byte hash, got %d bytes", len(bs.Inner))
	}
}

func TestIfThenElseBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	// Test with true condition - should return "then" branch
	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.IfThenElse,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	cond := &Constant{&syn.Bool{Inner: true}}
	thenVal := &Constant{&syn.String{Inner: "then"}}
	elseVal := &Constant{&syn.String{Inner: "else"}}

	b = b.ApplyArg(cond)
	b = b.ApplyArg(thenVal)
	b = b.ApplyArg(elseVal)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	str, ok := constVal.Constant.(*syn.String)
	if !ok {
		t.Fatalf("expected String constant, got %T", constVal.Constant)
	}

	if str.Inner != "then" {
		t.Fatalf("expected 'then', got %v", str.Inner)
	}

	// Test with false condition - should return "else" branch
	b2 := &Builtin[syn.DeBruijn]{
		Func:     builtin.IfThenElse,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	cond2 := &Constant{&syn.Bool{Inner: false}}
	thenVal2 := &Constant{&syn.String{Inner: "then"}}
	elseVal2 := &Constant{&syn.String{Inner: "else"}}

	b2 = b2.ApplyArg(cond2)
	b2 = b2.ApplyArg(thenVal2)
	b2 = b2.ApplyArg(elseVal2)

	val2, err := m.evalBuiltinApp(b2)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal2, ok := val2.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val2)
	}

	str2, ok := constVal2.Constant.(*syn.String)
	if !ok {
		t.Fatalf("expected String constant, got %T", constVal2.Constant)
	}

	if str2.Inner != "else" {
		t.Fatalf("expected 'else', got %v", str2.Inner)
	}
}

func TestChooseUnitBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.ChooseUnit,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// ChooseUnit takes a unit value and returns the second argument
	unitVal := &Constant{&syn.Unit{}}
	resultVal := &Constant{&syn.String{Inner: "chosen"}}

	b = b.ApplyArg(unitVal)
	b = b.ApplyArg(resultVal)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	str, ok := constVal.Constant.(*syn.String)
	if !ok {
		t.Fatalf("expected String constant, got %T", constVal.Constant)
	}

	if str.Inner != "chosen" {
		t.Fatalf("expected 'chosen', got %v", str.Inner)
	}
}

func TestFstPairBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.FstPair,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Create a pair (1, "second")
	pair := &syn.ProtoPair{
		First:  &syn.Integer{Inner: big.NewInt(1)},
		Second: &syn.String{Inner: "second"},
	}

	v1 := &Constant{pair}
	b = b.ApplyArg(v1)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	intConst, ok := constVal.Constant.(*syn.Integer)
	if !ok {
		t.Fatalf("expected Integer constant, got %T", constVal.Constant)
	}

	if intConst.Inner.Int64() != 1 {
		t.Fatalf("expected 1, got %v", intConst.Inner)
	}
}

func TestSndPairBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.SndPair,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Create a pair (1, "second")
	pair := &syn.ProtoPair{
		First:  &syn.Integer{Inner: big.NewInt(1)},
		Second: &syn.String{Inner: "second"},
	}

	v1 := &Constant{pair}
	b = b.ApplyArg(v1)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	str, ok := constVal.Constant.(*syn.String)
	if !ok {
		t.Fatalf("expected String constant, got %T", constVal.Constant)
	}

	if str.Inner != "second" {
		t.Fatalf("expected 'second', got %v", str.Inner)
	}
}

func TestConstrDataBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.ConstrData,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// ConstrData takes a constructor index and a list of data arguments
	index := &Constant{&syn.Integer{Inner: big.NewInt(0)}}
	dataList := &syn.ProtoList{
		LTyp: &syn.TData{},
		List: []syn.IConstant{
			&syn.Data{Inner: &data.Integer{Inner: big.NewInt(42)}},
		},
	}
	listArg := &Constant{dataList}

	b = b.ApplyArg(index)
	b = b.ApplyArg(listArg)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	dataConst, ok := constVal.Constant.(*syn.Data)
	if !ok {
		t.Fatalf("expected Data constant, got %T", constVal.Constant)
	}

	// Should be a constructor data type
	constrData, ok := dataConst.Inner.(*data.Constr)
	if !ok {
		t.Fatalf("expected Constr data, got %T", dataConst.Inner)
	}

	if constrData.Tag != 0 {
		t.Fatalf("expected constructor 0, got %d", constrData.Tag)
	}
}

func TestIDataBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.IData,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	intVal := &Constant{&syn.Integer{Inner: big.NewInt(123)}}
	b = b.ApplyArg(intVal)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	dataConst, ok := constVal.Constant.(*syn.Data)
	if !ok {
		t.Fatalf("expected Data constant, got %T", constVal.Constant)
	}

	// Should be an integer data type
	intData, ok := dataConst.Inner.(*data.Integer)
	if !ok {
		t.Fatalf("expected Integer data, got %T", dataConst.Inner)
	}

	if intData.Inner.Int64() != 123 {
		t.Fatalf("expected 123, got %v", intData.Inner)
	}
}

func TestBDataBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.BData,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	bsVal := &Constant{&syn.ByteString{Inner: []byte("hello")}}
	b = b.ApplyArg(bsVal)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	dataConst, ok := constVal.Constant.(*syn.Data)
	if !ok {
		t.Fatalf("expected Data constant, got %T", constVal.Constant)
	}

	// Should be a byte string data type
	bsData, ok := dataConst.Inner.(*data.ByteString)
	if !ok {
		t.Fatalf("expected ByteString data, got %T", dataConst.Inner)
	}

	if string(bsData.Inner) != "hello" {
		t.Fatalf("expected 'hello', got %v", string(bsData.Inner))
	}
}

func TestListDataBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.ListData,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Create a list of data values
	dataList := &syn.ProtoList{
		LTyp: &syn.TData{},
		List: []syn.IConstant{
			&syn.Data{Inner: &data.Integer{Inner: big.NewInt(1)}},
			&syn.Data{Inner: &data.Integer{Inner: big.NewInt(2)}},
		},
	}
	listArg := &Constant{dataList}

	b = b.ApplyArg(listArg)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	dataConst, ok := constVal.Constant.(*syn.Data)
	if !ok {
		t.Fatalf("expected Data constant, got %T", constVal.Constant)
	}

	// Should be a list data type
	listData, ok := dataConst.Inner.(*data.List)
	if !ok {
		t.Fatalf("expected List data, got %T", dataConst.Inner)
	}

	if len(listData.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(listData.Items))
	}
}

func TestKeccak_256Builtin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Keccak_256,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	input := []byte("hello world")
	v1 := &Constant{&syn.ByteString{Inner: input}}

	b = b.ApplyArg(v1)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	bs, ok := constVal.Constant.(*syn.ByteString)
	if !ok {
		t.Fatalf("expected ByteString constant, got %T", constVal.Constant)
	}

	// Keccak-256 should be 32 bytes
	if len(bs.Inner) != 32 {
		t.Fatalf("expected 32-byte hash, got %d bytes", len(bs.Inner))
	}
}

func TestMkConsBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.MkCons,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// MkCons takes an element and a list, returns a new list with the element prepended
	elem := &Constant{&syn.Integer{Inner: big.NewInt(0)}}
	list := &syn.ProtoList{
		LTyp: &syn.TInteger{},
		List: []syn.IConstant{
			&syn.Integer{Inner: big.NewInt(1)},
			&syn.Integer{Inner: big.NewInt(2)},
		},
	}
	listArg := &Constant{list}

	b = b.ApplyArg(elem)
	b = b.ApplyArg(listArg)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	resultList, ok := constVal.Constant.(*syn.ProtoList)
	if !ok {
		t.Fatalf("expected ProtoList constant, got %T", constVal.Constant)
	}

	if len(resultList.List) != 3 {
		t.Fatalf("expected list with 3 elements, got %d", len(resultList.List))
	}

	// Check that 0 was prepended
	firstElem, ok := resultList.List[0].(*syn.Integer)
	if !ok {
		t.Fatalf("expected Integer element, got %T", resultList.List[0])
	}
	if firstElem.Inner.Int64() != 0 {
		t.Fatalf("expected first element to be 0, got %v", firstElem.Inner)
	}
}

func TestDivideIntegerByZeroBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.DivideInteger,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	dividend := &Constant{&syn.Integer{Inner: big.NewInt(10)}}
	divisor := &Constant{&syn.Integer{Inner: big.NewInt(0)}}

	b = b.ApplyArg(dividend)
	b = b.ApplyArg(divisor)

	_, err := m.evalBuiltinApp(b)
	if err == nil {
		t.Fatal("expected division by zero error, got nil")
	}
}

func TestQuotientIntegerByZeroBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.QuotientInteger,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	dividend := &Constant{&syn.Integer{Inner: big.NewInt(10)}}
	divisor := &Constant{&syn.Integer{Inner: big.NewInt(0)}}

	b = b.ApplyArg(dividend)
	b = b.ApplyArg(divisor)

	_, err := m.evalBuiltinApp(b)
	if err == nil {
		t.Fatal("expected division by zero error, got nil")
	}
}

func TestRemainderIntegerByZeroBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.RemainderInteger,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	dividend := &Constant{&syn.Integer{Inner: big.NewInt(10)}}
	divisor := &Constant{&syn.Integer{Inner: big.NewInt(0)}}

	b = b.ApplyArg(dividend)
	b = b.ApplyArg(divisor)

	_, err := m.evalBuiltinApp(b)
	if err == nil {
		t.Fatal("expected division by zero error, got nil")
	}
}

func TestModIntegerByZeroBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.ModInteger,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	dividend := &Constant{&syn.Integer{Inner: big.NewInt(10)}}
	divisor := &Constant{&syn.Integer{Inner: big.NewInt(0)}}

	b = b.ApplyArg(dividend)
	b = b.ApplyArg(divisor)

	_, err := m.evalBuiltinApp(b)
	if err == nil {
		t.Fatal("expected division by zero error, got nil")
	}
}

func TestBls12_381_G1_AddBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Bls12_381_G1_Add,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Get generator points
	g1Gen, _, _, _ := bls.Generators()

	// Create two G1 generator points
	g1 := new(bls.G1Jac).Set(&g1Gen)
	g1_2 := new(bls.G1Jac).Set(&g1Gen)

	v1 := &Constant{&syn.Bls12_381G1Element{Inner: g1}}
	v2 := &Constant{&syn.Bls12_381G1Element{Inner: g1_2}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	result, ok := constVal.Constant.(*syn.Bls12_381G1Element)
	if !ok {
		t.Fatalf(
			"expected Bls12_381G1Element constant, got %T",
			constVal.Constant,
		)
	}

	// G1 + G1 should equal 2*G1
	expected := new(bls.G1Jac).Set(&g1Gen)
	expected.Double(expected)

	if !result.Inner.Equal(expected) {
		t.Fatalf("expected G1 + G1 = 2*G1, but got different result")
	}
}

func TestBls12_381_G1_EqualBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Bls12_381_G1_Equal,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Get generator points
	g1Gen, _, _, _ := bls.Generators()

	// Test equal points
	g1 := new(bls.G1Jac).Set(&g1Gen)
	g1_2 := new(bls.G1Jac).Set(&g1Gen)

	v1 := &Constant{&syn.Bls12_381G1Element{Inner: g1}}
	v2 := &Constant{&syn.Bls12_381G1Element{Inner: g1_2}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	result, ok := constVal.Constant.(*syn.Bool)
	if !ok {
		t.Fatalf("expected Bool constant, got %T", constVal.Constant)
	}

	if !result.Inner {
		t.Fatalf("expected equal G1 points to return true, got false")
	}

	// Test unequal points
	b2 := &Builtin[syn.DeBruijn]{
		Func:     builtin.Bls12_381_G1_Equal,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	g1_diff := new(bls.G1Jac).Set(&g1Gen)
	g1_diff.Double(&g1Gen) // 2*G1

	v3 := &Constant{&syn.Bls12_381G1Element{Inner: g1}}
	v4 := &Constant{&syn.Bls12_381G1Element{Inner: g1_diff}}

	b2 = b2.ApplyArg(v3)
	b2 = b2.ApplyArg(v4)

	val2, err := m.evalBuiltinApp(b2)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal2, ok := val2.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val2)
	}

	result2, ok := constVal2.Constant.(*syn.Bool)
	if !ok {
		t.Fatalf("expected Bool constant, got %T", constVal2.Constant)
	}

	if result2.Inner {
		t.Fatalf("expected unequal G1 points to return false, got true")
	}
}

func TestBls12_381_G1_CompressBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Bls12_381_G1_Compress,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Get generator points
	g1Gen, _, _, _ := bls.Generators()

	g1 := new(bls.G1Jac).Set(&g1Gen)
	v1 := &Constant{&syn.Bls12_381G1Element{Inner: g1}}

	b = b.ApplyArg(v1)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	result, ok := constVal.Constant.(*syn.ByteString)
	if !ok {
		t.Fatalf("expected ByteString constant, got %T", constVal.Constant)
	}

	// G1 compression should result in 48 bytes
	if len(result.Inner) != 48 {
		t.Fatalf(
			"expected 48-byte compressed G1 point, got %d bytes",
			len(result.Inner),
		)
	}
}

func TestBls12_381_G1_NegBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Bls12_381_G1_Neg,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Get generator points
	g1Gen, _, _, _ := bls.Generators()

	g1 := new(bls.G1Jac).Set(&g1Gen)
	v1 := &Constant{&syn.Bls12_381G1Element{Inner: g1}}

	b = b.ApplyArg(v1)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	result, ok := constVal.Constant.(*syn.Bls12_381G1Element)
	if !ok {
		t.Fatalf(
			"expected Bls12_381G1Element constant, got %T",
			constVal.Constant,
		)
	}

	// Test that negating twice gives the original point
	negAgain := new(bls.G1Jac).Set(result.Inner)
	negAgain.Neg(negAgain)

	if !g1.Equal(negAgain) {
		t.Fatalf("expected -(-G1) = G1, but got different result")
	}
}

func TestBls12_381_G2_AddBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Bls12_381_G2_Add,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Get generator points
	_, g2Gen, _, _ := bls.Generators()

	// Create two G2 generator points
	g2 := new(bls.G2Jac).Set(&g2Gen)
	g2_2 := new(bls.G2Jac).Set(&g2Gen)

	v1 := &Constant{&syn.Bls12_381G2Element{Inner: g2}}
	v2 := &Constant{&syn.Bls12_381G2Element{Inner: g2_2}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	result, ok := constVal.Constant.(*syn.Bls12_381G2Element)
	if !ok {
		t.Fatalf(
			"expected Bls12_381G2Element constant, got %T",
			constVal.Constant,
		)
	}

	// G2 + G2 should equal 2*G2
	expected := new(bls.G2Jac).Set(&g2Gen)
	expected.Double(&g2Gen)

	if !result.Inner.Equal(expected) {
		t.Fatalf("expected G2 + G2 = 2*G2, but got different result")
	}
}

func TestBls12_381_G2_EqualBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Bls12_381_G2_Equal,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Get generator points
	_, g2Gen, _, _ := bls.Generators()

	// Test equal points
	g2 := new(bls.G2Jac).Set(&g2Gen)
	g2_2 := new(bls.G2Jac).Set(&g2Gen)

	v1 := &Constant{&syn.Bls12_381G2Element{Inner: g2}}
	v2 := &Constant{&syn.Bls12_381G2Element{Inner: g2_2}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	result, ok := constVal.Constant.(*syn.Bool)
	if !ok {
		t.Fatalf("expected Bool constant, got %T", constVal.Constant)
	}

	if !result.Inner {
		t.Fatalf("expected equal G2 points to return true, got false")
	}

	// Test unequal points
	b2 := &Builtin[syn.DeBruijn]{
		Func:     builtin.Bls12_381_G2_Equal,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	g2_diff := new(bls.G2Jac).Set(&g2Gen)
	g2_diff.Double(&g2Gen) // 2*G2

	v3 := &Constant{&syn.Bls12_381G2Element{Inner: g2}}
	v4 := &Constant{&syn.Bls12_381G2Element{Inner: g2_diff}}

	b2 = b2.ApplyArg(v3)
	b2 = b2.ApplyArg(v4)

	val2, err := m.evalBuiltinApp(b2)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal2, ok := val2.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val2)
	}

	result2, ok := constVal2.Constant.(*syn.Bool)
	if !ok {
		t.Fatalf("expected Bool constant, got %T", constVal2.Constant)
	}

	if result2.Inner {
		t.Fatalf("expected unequal G2 points to return false, got true")
	}
}

func TestBls12_381_G2_CompressBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Bls12_381_G2_Compress,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Get generator points
	_, g2Gen, _, _ := bls.Generators()

	g2 := new(bls.G2Jac).Set(&g2Gen)
	v1 := &Constant{&syn.Bls12_381G2Element{Inner: g2}}

	b = b.ApplyArg(v1)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	result, ok := constVal.Constant.(*syn.ByteString)
	if !ok {
		t.Fatalf("expected ByteString constant, got %T", constVal.Constant)
	}

	// G2 compression should result in 96 bytes
	if len(result.Inner) != 96 {
		t.Fatalf(
			"expected 96-byte compressed G2 point, got %d bytes",
			len(result.Inner),
		)
	}
}

func TestBls12_381_G2_NegBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Bls12_381_G2_Neg,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Get generator points
	_, g2Gen, _, _ := bls.Generators()

	g2 := new(bls.G2Jac).Set(&g2Gen)
	v1 := &Constant{&syn.Bls12_381G2Element{Inner: g2}}

	b = b.ApplyArg(v1)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	result, ok := constVal.Constant.(*syn.Bls12_381G2Element)
	if !ok {
		t.Fatalf(
			"expected Bls12_381G2Element constant, got %T",
			constVal.Constant,
		)
	}

	// Test that negating twice gives the original point
	negAgain := new(bls.G2Jac).Set(result.Inner)
	negAgain.Neg(negAgain)

	if !g2.Equal(negAgain) {
		t.Fatalf("expected -(-G2) = G2, but got different result")
	}
}

func TestBls12_381_MillerLoopBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Bls12_381_MillerLoop,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Get generator points
	g1Gen, g2Gen, _, _ := bls.Generators()

	v1 := &Constant{&syn.Bls12_381G1Element{Inner: &g1Gen}}
	v2 := &Constant{&syn.Bls12_381G2Element{Inner: &g2Gen}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	result, ok := constVal.Constant.(*syn.Bls12_381MlResult)
	if !ok {
		t.Fatalf(
			"expected Bls12_381MlResult constant, got %T",
			constVal.Constant,
		)
	}

	// The result should be a valid Miller loop result (GT element)
	if result.Inner == nil {
		t.Fatalf("expected non-nil Miller loop result")
	}
}

func TestBls12_381_MulMlResultBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Bls12_381_MulMlResult,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// First create two Miller loop results
	// Create first Miller loop result
	ml1, err := bls.MillerLoop([]bls.G1Affine{{}}, []bls.G2Affine{{}})
	if err != nil {
		t.Fatalf("failed to create Miller loop result: %v", err)
	}

	// Create second Miller loop result (identity)
	ml2 := new(bls.GT).SetOne()

	v1 := &Constant{&syn.Bls12_381MlResult{Inner: &ml1}}
	v2 := &Constant{&syn.Bls12_381MlResult{Inner: ml2}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	result, ok := constVal.Constant.(*syn.Bls12_381MlResult)
	if !ok {
		t.Fatalf(
			"expected Bls12_381MlResult constant, got %T",
			constVal.Constant,
		)
	}

	// The result should be a valid GT element
	if result.Inner == nil {
		t.Fatalf("expected non-nil MulMlResult")
	}
}

func TestBls12_381_FinalVerifyBuiltin(t *testing.T) {
	m := NewMachineWithVersionCosts[syn.DeBruijn]([3]uint32{1, 2, 0}, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Bls12_381_FinalVerify,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// Create two Miller loop results for verification
	// First result: identity element
	gt1 := new(bls.GT).SetOne()

	// Second result: also identity element (so verification should pass)
	gt2 := new(bls.GT).SetOne()

	v1 := &Constant{&syn.Bls12_381MlResult{Inner: gt1}}
	v2 := &Constant{&syn.Bls12_381MlResult{Inner: gt2}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val, err := m.evalBuiltinApp(b)
	if err != nil {
		t.Fatalf("evalBuiltinApp returned error: %v", err)
	}

	constVal, ok := val.(*Constant)
	if !ok {
		t.Fatalf("expected Constant result, got %T", val)
	}

	result, ok := constVal.Constant.(*syn.Bool)
	if !ok {
		t.Fatalf("expected Bool constant, got %T", constVal.Constant)
	}

	// Final verification of two identity elements should return true
	if !result.Inner {
		t.Fatalf(
			"expected final verification of identity elements to return true, got false",
		)
	}
}
