package cek

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/syn"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

// Helper functions to reduce test code duplication

func newTestMachine() *Machine[syn.DeBruijn] {
	return NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)
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

func evalBuiltinWithError(
	t *testing.T,
	m *Machine[syn.DeBruijn],
	b *Builtin[syn.DeBruijn],
) (Value[syn.DeBruijn], error) {
	t.Helper()
	return m.evalBuiltinApp(b)
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

func TestDivideIntegerBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.DivideInteger)

	v1 := &Constant{&syn.Integer{Inner: big.NewInt(10)}}
	v2 := &Constant{&syn.Integer{Inner: big.NewInt(2)}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	i := expectInteger(t, constVal)

	if i.Inner.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("expected 5, got %v", i.Inner)
	}
}

func TestDivideIntegerByZero(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.DivideInteger)

	v1 := &Constant{&syn.Integer{Inner: big.NewInt(10)}}
	v2 := &Constant{&syn.Integer{Inner: big.NewInt(0)}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	_, err := evalBuiltinWithError(t, m, b)
	if err == nil {
		t.Fatalf("expected division by zero error, got nil")
	}
	if err.Error() != "division by zero" {
		t.Fatalf("expected 'division by zero', got %v", err)
	}
}

func TestQuotientIntegerBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.QuotientInteger)

	v1 := &Constant{&syn.Integer{Inner: big.NewInt(10)}}
	v2 := &Constant{&syn.Integer{Inner: big.NewInt(3)}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	i := expectInteger(t, constVal)

	if i.Inner.Cmp(big.NewInt(3)) != 0 {
		t.Fatalf("expected 3, got %v", i.Inner)
	}
}

func TestRemainderIntegerBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.RemainderInteger)

	v1 := &Constant{&syn.Integer{Inner: big.NewInt(10)}}
	v2 := &Constant{&syn.Integer{Inner: big.NewInt(3)}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	i := expectInteger(t, constVal)

	if i.Inner.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("expected 1, got %v", i.Inner)
	}
}

func TestModIntegerBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.ModInteger)

	v1 := &Constant{&syn.Integer{Inner: big.NewInt(10)}}
	v2 := &Constant{&syn.Integer{Inner: big.NewInt(3)}}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	i := expectInteger(t, constVal)

	if i.Inner.Cmp(big.NewInt(1)) != 0 {
		t.Fatalf("expected 1, got %v", i.Inner)
	}
}

func TestChooseDataBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.ChooseData)

	// Create a Constr data
	constrData := data.NewConstr(1, data.NewInteger(big.NewInt(42)))
	dataVal := &Constant{&syn.Data{Inner: constrData}}

	// Create 6 branch values
	constrBranch := &Constant{&syn.String{Inner: "constr"}}
	mapBranch := &Constant{&syn.String{Inner: "map"}}
	listBranch := &Constant{&syn.String{Inner: "list"}}
	integerBranch := &Constant{&syn.String{Inner: "integer"}}
	bytesBranch := &Constant{&syn.String{Inner: "bytes"}}

	b = b.ApplyArg(dataVal)
	b = b.ApplyArg(constrBranch)
	b = b.ApplyArg(mapBranch)
	b = b.ApplyArg(listBranch)
	b = b.ApplyArg(integerBranch)
	b = b.ApplyArg(bytesBranch)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	str := expectString(t, constVal)

	if str.Inner != "constr" {
		t.Fatalf("expected 'constr', got %v", str.Inner)
	}
}

func TestLengthOfArrayBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.LengthOfArray)

	// Create a ProtoList with 3 elements
	list := []syn.IConstant{
		&syn.Integer{Inner: big.NewInt(1)},
		&syn.Integer{Inner: big.NewInt(2)},
		&syn.Integer{Inner: big.NewInt(3)},
	}
	protoList := &syn.ProtoList{List: list}
	arrayVal := &Constant{protoList}

	b = b.ApplyArg(arrayVal)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	i := expectInteger(t, constVal)

	if i.Inner.Cmp(big.NewInt(3)) != 0 {
		t.Fatalf("expected 3, got %v", i.Inner)
	}
}

func TestExpModIntegerBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.ExpModInteger)

	// Test case: 2^3 mod 5 = 8 mod 5 = 3
	base := &Constant{&syn.Integer{Inner: big.NewInt(2)}}
	exponent := &Constant{&syn.Integer{Inner: big.NewInt(3)}}
	modulus := &Constant{&syn.Integer{Inner: big.NewInt(5)}}

	b = b.ApplyArg(base)
	b = b.ApplyArg(exponent)
	b = b.ApplyArg(modulus)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	i := expectInteger(t, constVal)

	if i.Inner.Cmp(big.NewInt(3)) != 0 {
		t.Fatalf("expected 3, got %v", i.Inner)
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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

	b := &Builtin[syn.DeBruijn]{
		Func:     builtin.Bls12_381_MulMlResult,
		Args:     nil,
		ArgCount: 0,
		Forces:   0,
	}

	// First create two Miller loop results
	// Create first Miller loop result using generators
	g1Gen, g2Gen, _, _ := bls.Generators()
	var g1Affine bls.G1Affine
	g1Affine.FromJacobian(&g1Gen)
	var g2Affine bls.G2Affine
	g2Affine.FromJacobian(&g2Gen)
	ml1, err := bls.MillerLoop(
		[]bls.G1Affine{g1Affine},
		[]bls.G2Affine{g2Affine},
	)
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
	m := NewMachineWithVersionCosts[syn.DeBruijn](LanguageVersionV3, 0)

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

func TestLessThanIntegerBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		arg1     int64
		arg2     int64
		expected bool
	}{
		{"less than true", 5, 10, true},
		{"less than false", 10, 5, false},
		{"equal", 5, 5, false},
		{"negative numbers", -10, -5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.LessThanInteger)

			v1 := &Constant{&syn.Integer{Inner: big.NewInt(tt.arg1)}}
			v2 := &Constant{&syn.Integer{Inner: big.NewInt(tt.arg2)}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			boolVal := expectBool(t, constVal)

			if boolVal.Inner != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, boolVal.Inner)
			}
		})
	}
}

func TestLessThanEqualsIntegerBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		arg1     int64
		arg2     int64
		expected bool
	}{
		{"less than true", 5, 10, true},
		{"less than false", 10, 5, false},
		{"equal", 5, 5, true},
		{"negative numbers", -10, -5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.LessThanEqualsInteger)

			v1 := &Constant{&syn.Integer{Inner: big.NewInt(tt.arg1)}}
			v2 := &Constant{&syn.Integer{Inner: big.NewInt(tt.arg2)}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			boolVal := expectBool(t, constVal)

			if boolVal.Inner != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, boolVal.Inner)
			}
		})
	}
}

func TestConsByteStringBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		byte     uint8
		bs       []byte
		expected []byte
	}{
		{"prepend to empty", 0x42, []byte{}, []byte{0x42}},
		{
			"prepend to non-empty",
			0x01,
			[]byte{0x02, 0x03},
			[]byte{0x01, 0x02, 0x03},
		},
		{"prepend zero", 0x00, []byte{0x01}, []byte{0x00, 0x01}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.ConsByteString)

			v1 := &Constant{&syn.Integer{Inner: big.NewInt(int64(tt.byte))}}
			v2 := &Constant{&syn.ByteString{Inner: tt.bs}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			bsVal := expectByteString(t, constVal)

			if !bytes.Equal(bsVal.Inner, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, bsVal.Inner)
			}
		})
	}
}

func TestSliceByteStringBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		skip     int64
		take     int64
		bs       []byte
		expected []byte
	}{
		{
			"full slice",
			0,
			3,
			[]byte{0x01, 0x02, 0x03},
			[]byte{0x01, 0x02, 0x03},
		},
		{"skip 1 take 1", 1, 1, []byte{0x01, 0x02, 0x03}, []byte{0x02}},
		{"take 0", 1, 0, []byte{0x01, 0x02, 0x03}, []byte{}},
		{"out of bounds", 0, 10, []byte{0x01, 0x02}, []byte{0x01, 0x02}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.SliceByteString)

			v1 := &Constant{&syn.Integer{Inner: big.NewInt(tt.skip)}}
			v2 := &Constant{&syn.Integer{Inner: big.NewInt(tt.take)}}
			v3 := &Constant{&syn.ByteString{Inner: tt.bs}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)
			b = b.ApplyArg(v3)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			bsVal := expectByteString(t, constVal)

			if !bytes.Equal(bsVal.Inner, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, bsVal.Inner)
			}
		})
	}
}

func TestIndexByteStringBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		bs       []byte
		index    int64
		expected uint8
	}{
		{"first byte", []byte{0x01, 0x02, 0x03}, 0, 0x01},
		{"middle byte", []byte{0x01, 0x02, 0x03}, 1, 0x02},
		{"last byte", []byte{0x01, 0x02, 0x03}, 2, 0x03},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.IndexByteString)

			v1 := &Constant{&syn.ByteString{Inner: tt.bs}}
			v2 := &Constant{&syn.Integer{Inner: big.NewInt(tt.index)}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			intVal := expectInteger(t, constVal)

			if intVal.Inner.Uint64() != uint64(tt.expected) {
				t.Errorf(
					"expected %v, got %v",
					tt.expected,
					intVal.Inner.Uint64(),
				)
			}
		})
	}
}

func TestLessThanEqualsByteStringBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		arg1     []byte
		arg2     []byte
		expected bool
	}{
		{"less than true", []byte{1, 2}, []byte{1, 3}, true},
		{"less than false", []byte{1, 3}, []byte{1, 2}, false},
		{"equal", []byte{1, 2}, []byte{1, 2}, true},
		{"different lengths", []byte{1}, []byte{1, 2}, true},
		{"empty vs non-empty", []byte{}, []byte{1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.LessThanEqualsByteString)

			v1 := &Constant{&syn.ByteString{Inner: tt.arg1}}
			v2 := &Constant{&syn.ByteString{Inner: tt.arg2}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			boolVal := expectBool(t, constVal)

			if boolVal.Inner != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, boolVal.Inner)
			}
		})
	}
}

func TestVerifyEd25519SignatureBuiltin(t *testing.T) {
	// Generate a key pair
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}

	message := []byte("test message")
	signature := ed25519.Sign(privateKey, message)

	// Test valid signature
	t.Run("valid signature", func(t *testing.T) {
		m := newTestMachine()
		b := newTestBuiltin(builtin.VerifyEd25519Signature)

		v1 := &Constant{&syn.ByteString{Inner: publicKey}}
		v2 := &Constant{&syn.ByteString{Inner: message}}
		v3 := &Constant{&syn.ByteString{Inner: signature}}

		b = b.ApplyArg(v1)
		b = b.ApplyArg(v2)
		b = b.ApplyArg(v3)

		val := evalBuiltin(t, m, b)
		constVal := expectConstant(t, val)
		boolVal := expectBool(t, constVal)

		if !boolVal.Inner {
			t.Errorf("expected true for valid signature, got false")
		}
	})

	// Test invalid signature
	t.Run("invalid signature", func(t *testing.T) {
		m := newTestMachine()
		b := newTestBuiltin(builtin.VerifyEd25519Signature)

		invalidSig := make([]byte, ed25519.SignatureSize)
		copy(invalidSig, signature)
		invalidSig[0] ^= 1 // Flip a bit

		v1 := &Constant{&syn.ByteString{Inner: publicKey}}
		v2 := &Constant{&syn.ByteString{Inner: message}}
		v3 := &Constant{&syn.ByteString{Inner: invalidSig}}

		b = b.ApplyArg(v1)
		b = b.ApplyArg(v2)
		b = b.ApplyArg(v3)

		val := evalBuiltin(t, m, b)
		constVal := expectConstant(t, val)
		boolVal := expectBool(t, constVal)

		if boolVal.Inner {
			t.Errorf("expected false for invalid signature, got true")
		}
	})

	// Test wrong public key
	t.Run("wrong public key", func(t *testing.T) {
		m := newTestMachine()
		b := newTestBuiltin(builtin.VerifyEd25519Signature)

		wrongPubKey := make([]byte, ed25519.PublicKeySize)
		copy(wrongPubKey, publicKey)
		wrongPubKey[0] ^= 1

		v1 := &Constant{&syn.ByteString{Inner: wrongPubKey}}
		v2 := &Constant{&syn.ByteString{Inner: message}}
		v3 := &Constant{&syn.ByteString{Inner: signature}}

		b = b.ApplyArg(v1)
		b = b.ApplyArg(v2)
		b = b.ApplyArg(v3)

		val := evalBuiltin(t, m, b)
		constVal := expectConstant(t, val)
		boolVal := expectBool(t, constVal)

		if boolVal.Inner {
			t.Errorf("expected false for wrong public key, got true")
		}
	})
}

func TestBlake2B224Builtin(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectedLen int
	}{
		{"empty", []byte{}, 28},
		{"hello", []byte("hello"), 28},
		{"long input", []byte("this is a longer input for hashing"), 28},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.Blake2b_224)

			v1 := &Constant{&syn.ByteString{Inner: tt.input}}

			b = b.ApplyArg(v1)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			bs := expectByteString(t, constVal)

			if len(bs.Inner) != tt.expectedLen {
				t.Errorf(
					"expected hash length %d, got %d",
					tt.expectedLen,
					len(bs.Inner),
				)
			}
		})
	}
}

func TestEncodeUtf8Builtin(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []byte
	}{
		{"empty", "", []byte{}},
		{"ascii", "hello", []byte("hello")},
		{"unicode", "héllo", []byte("héllo")},
		{"emoji", "🚀", []byte("🚀")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.EncodeUtf8)

			v1 := &Constant{&syn.String{Inner: tt.input}}

			b = b.ApplyArg(v1)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			bs := expectByteString(t, constVal)

			if !bytes.Equal(bs.Inner, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, bs.Inner)
			}
		})
	}
}

func TestDecodeUtf8Builtin(t *testing.T) {
	tests := []struct {
		name       string
		input      []byte
		expected   string
		shouldFail bool
	}{
		{"empty", []byte{}, "", false},
		{"ascii", []byte("hello"), "hello", false},
		{"unicode", []byte("héllo"), "héllo", false},
		{"emoji", []byte("🚀"), "🚀", false},
		{"invalid utf8", []byte{0xff, 0xfe, 0xfd}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.DecodeUtf8)

			v1 := &Constant{&syn.ByteString{Inner: tt.input}}

			b = b.ApplyArg(v1)

			val, err := m.evalBuiltinApp(b)
			if tt.shouldFail {
				if err == nil {
					t.Errorf("expected error for invalid UTF-8, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("evalBuiltinApp returned error: %v", err)
			}

			constVal := expectConstant(t, val)
			str := expectString(t, constVal)

			if str.Inner != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, str.Inner)
			}
		})
	}
}

func TestTraceBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.Trace)

	message := "debug message"
	returnValue := &Constant{&syn.Integer{Inner: big.NewInt(42)}}

	v1 := &Constant{&syn.String{Inner: message}}
	v2 := returnValue

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	i := expectInteger(t, constVal)

	if i.Inner.Cmp(big.NewInt(42)) != 0 {
		t.Errorf("expected 42, got %v", i.Inner)
	}

	if len(m.Logs) != 1 || m.Logs[0] != message {
		t.Errorf("expected log %q, got %v", message, m.Logs)
	}
}

func TestChooseListBuiltin(t *testing.T) {
	emptyBranch := &Constant{&syn.String{Inner: "empty"}}
	otherwiseBranch := &Constant{&syn.String{Inner: "not empty"}}

	t.Run("empty list", func(t *testing.T) {
		m := newTestMachine()
		b := newTestBuiltin(builtin.ChooseList)

		emptyList := &Constant{&syn.ProtoList{List: []syn.IConstant{}}}
		v1 := emptyList
		v2 := emptyBranch
		v3 := otherwiseBranch

		b = b.ApplyArg(v1)
		b = b.ApplyArg(v2)
		b = b.ApplyArg(v3)

		val := evalBuiltin(t, m, b)
		if val != emptyBranch {
			t.Errorf("expected empty branch, got %v", val)
		}
	})

	t.Run("non-empty list", func(t *testing.T) {
		m := newTestMachine()
		b := newTestBuiltin(builtin.ChooseList)

		nonEmptyList := &Constant{
			&syn.ProtoList{
				List: []syn.IConstant{&syn.Integer{Inner: big.NewInt(1)}},
			},
		}
		v1 := nonEmptyList
		v2 := emptyBranch
		v3 := otherwiseBranch

		b = b.ApplyArg(v1)
		b = b.ApplyArg(v2)
		b = b.ApplyArg(v3)

		val := evalBuiltin(t, m, b)
		if val != otherwiseBranch {
			t.Errorf("expected otherwise branch, got %v", val)
		}
	})
}

func TestRipemd160Builtin(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectedLen int
	}{
		{"empty", []byte{}, 20},
		{"hello", []byte("hello"), 20},
		{"long input", []byte("this is a longer input for hashing"), 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.Ripemd_160)

			v1 := &Constant{&syn.ByteString{Inner: tt.input}}

			b = b.ApplyArg(v1)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			bs := expectByteString(t, constVal)

			if len(bs.Inner) != tt.expectedLen {
				t.Errorf(
					"expected hash length %d, got %d",
					tt.expectedLen,
					len(bs.Inner),
				)
			}
		})
	}
}

func TestMkNilDataBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.MkNilData)

	unit := &Constant{&syn.Unit{}}

	b = b.ApplyArg(unit)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	list := expectProtoList(t, constVal)

	if len(list.List) != 0 {
		t.Errorf("expected empty list, got %d elements", len(list.List))
	}

	// Check that it's a data list
	if _, ok := list.LTyp.(*syn.TData); !ok {
		t.Errorf("expected data type, got %T", list.LTyp)
	}
}

func TestMkNilPairDataBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.MkNilPairData)

	unit := &Constant{&syn.Unit{}}

	b = b.ApplyArg(unit)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	list := expectProtoList(t, constVal)

	if len(list.List) != 0 {
		t.Errorf("expected empty list, got %d elements", len(list.List))
	}

	// Check that it's a pair[data,data] list
	if pairType, ok := list.LTyp.(*syn.TPair); ok {
		if _, ok1 := pairType.First.(*syn.TData); !ok1 {
			t.Errorf("expected first type data, got %T", pairType.First)
		}
		if _, ok2 := pairType.Second.(*syn.TData); !ok2 {
			t.Errorf("expected second type data, got %T", pairType.Second)
		}
	} else {
		t.Errorf("expected pair type, got %T", list.LTyp)
	}
}

func TestMkPairDataBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.MkPairData)

	// Create two data values
	data1 := &syn.Data{Inner: &data.Integer{Inner: big.NewInt(42)}}
	data2 := &syn.Data{Inner: &data.ByteString{Inner: []byte("hello")}}

	v1 := &Constant{data1}
	v2 := &Constant{data2}

	b = b.ApplyArg(v1)
	b = b.ApplyArg(v2)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	pair := expectProtoPair(t, constVal)

	// Check the values
	fstData, ok := pair.First.(*syn.Data)
	if !ok {
		t.Fatalf("expected *syn.Data for first, got %T", pair.First)
	}
	sndData, ok := pair.Second.(*syn.Data)
	if !ok {
		t.Fatalf("expected *syn.Data for second, got %T", pair.Second)
	}

	fstInt, ok := fstData.Inner.(*data.Integer)
	if !ok {
		t.Fatalf(
			"expected *data.Integer for first inner, got %T",
			fstData.Inner,
		)
	}
	if fstInt.Inner.Cmp(big.NewInt(42)) != 0 {
		t.Errorf("expected first value 42, got %v", fstInt.Inner)
	}

	sndBS, ok := sndData.Inner.(*data.ByteString)
	if !ok {
		t.Fatalf(
			"expected *data.ByteString for second inner, got %T",
			sndData.Inner,
		)
	}
	if string(sndBS.Inner) != "hello" {
		t.Errorf("expected second value 'hello', got %s", string(sndBS.Inner))
	}
}

func TestSerialiseDataBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.SerialiseData)

	// Create a data value
	dataVal := &syn.Data{Inner: &data.Integer{Inner: big.NewInt(42)}}

	v1 := &Constant{dataVal}

	b = b.ApplyArg(v1)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	bs := expectByteString(t, constVal)

	// The result should be valid CBOR encoding
	if len(bs.Inner) == 0 {
		t.Errorf("expected non-empty byte string")
	}

	// We can verify by decoding it back
	decoded, err := data.Decode(bs.Inner)
	if err != nil {
		t.Errorf("failed to decode serialized data: %v", err)
	}

	if intData, ok := decoded.(*data.Integer); ok {
		if intData.Inner.Cmp(big.NewInt(42)) != 0 {
			t.Errorf("expected 42, got %v", intData.Inner)
		}
	} else {
		t.Errorf("expected Integer, got %T", decoded)
	}
}

func TestUnIDataBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.UnIData)

	// Create a data value containing an integer
	dataVal := &syn.Data{Inner: &data.Integer{Inner: big.NewInt(123)}}

	v1 := &Constant{dataVal}

	b = b.ApplyArg(v1)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	integer := expectInteger(t, constVal)

	if integer.Inner.Cmp(big.NewInt(123)) != 0 {
		t.Errorf("expected 123, got %v", integer.Inner)
	}
}

func TestUnBDataBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.UnBData)

	// Create a data value containing a byte string
	dataVal := &syn.Data{Inner: &data.ByteString{Inner: []byte("hello")}}

	v1 := &Constant{dataVal}

	b = b.ApplyArg(v1)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	bs := expectByteString(t, constVal)

	if string(bs.Inner) != "hello" {
		t.Errorf("expected 'hello', got %s", string(bs.Inner))
	}
}

func TestUnListDataBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.UnListData)

	// Create a data value containing a list
	list := &data.List{
		Items: []data.PlutusData{
			&data.Integer{Inner: big.NewInt(1)},
			&data.Integer{Inner: big.NewInt(2)},
			&data.Integer{Inner: big.NewInt(3)},
		},
	}
	dataVal := &syn.Data{Inner: list}

	v1 := &Constant{dataVal}

	b = b.ApplyArg(v1)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	protoList := expectProtoList(t, constVal)

	if len(protoList.List) != 3 {
		t.Errorf("expected list of length 3, got %d", len(protoList.List))
	}

	// Check first element
	if data1, ok := protoList.List[0].(*syn.Data); ok {
		if intData, ok := data1.Inner.(*data.Integer); ok {
			if intData.Inner.Cmp(big.NewInt(1)) != 0 {
				t.Errorf("expected first element 1, got %v", intData.Inner)
			}
		} else {
			t.Errorf("expected first element to be Integer, got %T", data1.Inner)
		}
	} else {
		t.Errorf("expected first element to be Data, got %T", protoList.List[0])
	}
}

func TestUnMapDataBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.UnMapData)

	// Create a data value containing a map
	dataMap := &data.Map{
		Pairs: [][2]data.PlutusData{
			{
				&data.Integer{Inner: big.NewInt(1)},
				&data.ByteString{Inner: []byte("one")},
			},
			{
				&data.Integer{Inner: big.NewInt(2)},
				&data.ByteString{Inner: []byte("two")},
			},
		},
	}
	dataVal := &syn.Data{Inner: dataMap}

	v1 := &Constant{dataVal}

	b = b.ApplyArg(v1)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	protoList := expectProtoList(t, constVal)

	if len(protoList.List) != 2 {
		t.Errorf("expected list of length 2, got %d", len(protoList.List))
	}

	// Check first pair
	if pair, ok := protoList.List[0].(*syn.ProtoPair); ok {
		if keyData, ok := pair.First.(*syn.Data); ok {
			if keyInt, ok := keyData.Inner.(*data.Integer); ok {
				if keyInt.Inner.Cmp(big.NewInt(1)) != 0 {
					t.Errorf("expected key 1, got %v", keyInt.Inner)
				}
			} else {
				t.Errorf("expected key to be Integer, got %T", keyData.Inner)
			}
		} else {
			t.Errorf("expected first to be Data, got %T", pair.First)
		}

		if valData, ok := pair.Second.(*syn.Data); ok {
			if valBS, ok := valData.Inner.(*data.ByteString); ok {
				if string(valBS.Inner) != "one" {
					t.Errorf(
						"expected value 'one', got %s",
						string(valBS.Inner),
					)
				}
			} else {
				t.Errorf("expected value to be ByteString, got %T", valData.Inner)
			}
		} else {
			t.Errorf("expected second to be Data, got %T", pair.Second)
		}
	} else {
		t.Errorf("expected first element to be ProtoPair, got %T", protoList.List[0])
	}
}

func TestMapDataBuiltin(t *testing.T) {
	m := newTestMachine()
	b := newTestBuiltin(builtin.MapData)

	// Create a list of pairs (each pair contains two Data values)
	pair1 := &syn.ProtoPair{
		FstType: &syn.TData{},
		SndType: &syn.TData{},
		First:   &syn.Data{Inner: &data.Integer{Inner: big.NewInt(1)}},
		Second:  &syn.Data{Inner: &data.ByteString{Inner: []byte("one")}},
	}
	pair2 := &syn.ProtoPair{
		FstType: &syn.TData{},
		SndType: &syn.TData{},
		First:   &syn.Data{Inner: &data.Integer{Inner: big.NewInt(2)}},
		Second:  &syn.Data{Inner: &data.ByteString{Inner: []byte("two")}},
	}

	protoList := &syn.ProtoList{
		LTyp: &syn.TPair{
			First:  &syn.TData{},
			Second: &syn.TData{},
		},
		List: []syn.IConstant{pair1, pair2},
	}

	v1 := &Constant{protoList}

	b = b.ApplyArg(v1)

	val := evalBuiltin(t, m, b)
	constVal := expectConstant(t, val)
	resultData := expectData(t, constVal)
	resultMap, ok := resultData.Inner.(*data.Map)
	if !ok {
		t.Fatalf("expected Map, got %T", resultData.Inner)
	}

	if len(resultMap.Pairs) != 2 {
		t.Errorf("expected map with 2 pairs, got %d", len(resultMap.Pairs))
	}

	// Check first pair
	keyInt, ok := resultMap.Pairs[0][0].(*data.Integer)
	if !ok {
		t.Fatalf(
			"expected *data.Integer for key, got %T",
			resultMap.Pairs[0][0],
		)
	}
	if keyInt.Inner.Cmp(big.NewInt(1)) != 0 {
		t.Errorf(
			"expected key 1, got %v",
			keyInt.Inner,
		)
	}
	valBS, ok := resultMap.Pairs[0][1].(*data.ByteString)
	if !ok {
		t.Fatalf(
			"expected *data.ByteString for value, got %T",
			resultMap.Pairs[0][1],
		)
	}
	if string(valBS.Inner) != "one" {
		t.Errorf(
			"expected value 'one', got %s",
			string(valBS.Inner),
		)
	}
}

func TestAndByteStringBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		pad      bool
		arg1     []byte
		arg2     []byte
		expected []byte
	}{
		{
			name:     "no padding, equal length",
			pad:      false,
			arg1:     []byte{0xFF, 0x0F},
			arg2:     []byte{0xF0, 0xFF},
			expected: []byte{0xF0, 0x0F},
		},
		{
			name:     "no padding, different length",
			pad:      false,
			arg1:     []byte{0xFF, 0x0F, 0xAA},
			arg2:     []byte{0xF0, 0xFF},
			expected: []byte{0xF0, 0x0F},
		},
		{
			name:     "with padding, equal length",
			pad:      true,
			arg1:     []byte{0xFF, 0x0F},
			arg2:     []byte{0xF0, 0xFF},
			expected: []byte{0xF0, 0x0F},
		},
		{
			name:     "with padding, arg1 longer",
			pad:      true,
			arg1:     []byte{0xFF, 0x0F, 0xAA},
			arg2:     []byte{0xF0, 0xFF},
			expected: []byte{0xF0, 0x0F, 0xAA},
		},
		{
			name:     "with padding, arg2 longer",
			pad:      true,
			arg1:     []byte{0xFF, 0x0F},
			arg2:     []byte{0xF0, 0xFF, 0xBB},
			expected: []byte{0xF0, 0x0F, 0xBB},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.AndByteString)

			v1 := &Constant{&syn.Bool{Inner: tt.pad}}
			v2 := &Constant{&syn.ByteString{Inner: tt.arg1}}
			v3 := &Constant{&syn.ByteString{Inner: tt.arg2}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)
			b = b.ApplyArg(v3)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			result := expectByteString(t, constVal)

			if !bytes.Equal(result.Inner, tt.expected) {
				t.Errorf("expected %x, got %x", tt.expected, result.Inner)
			}
		})
	}
}

func TestOrByteStringBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		pad      bool
		arg1     []byte
		arg2     []byte
		expected []byte
	}{
		{
			name:     "no padding, equal length",
			pad:      false,
			arg1:     []byte{0xF0, 0x0F},
			arg2:     []byte{0x0F, 0xF0},
			expected: []byte{0xFF, 0xFF},
		},
		{
			name:     "with padding, arg1 longer",
			pad:      true,
			arg1:     []byte{0xF0, 0x0F, 0xAA},
			arg2:     []byte{0x0F, 0xF0},
			expected: []byte{0xFF, 0xFF, 0xAA},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.OrByteString)

			v1 := &Constant{&syn.Bool{Inner: tt.pad}}
			v2 := &Constant{&syn.ByteString{Inner: tt.arg1}}
			v3 := &Constant{&syn.ByteString{Inner: tt.arg2}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)
			b = b.ApplyArg(v3)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			result := expectByteString(t, constVal)

			if !bytes.Equal(result.Inner, tt.expected) {
				t.Errorf("expected %x, got %x", tt.expected, result.Inner)
			}
		})
	}
}

func TestXorByteStringBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		pad      bool
		arg1     []byte
		arg2     []byte
		expected []byte
	}{
		{
			name:     "no padding, equal length",
			pad:      false,
			arg1:     []byte{0xFF, 0x0F},
			arg2:     []byte{0xF0, 0xFF},
			expected: []byte{0x0F, 0xF0},
		},
		{
			name:     "with padding, arg1 longer",
			pad:      true,
			arg1:     []byte{0xFF, 0x0F, 0xAA},
			arg2:     []byte{0xF0, 0xFF},
			expected: []byte{0x0F, 0xF0, 0xAA},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.XorByteString)

			v1 := &Constant{&syn.Bool{Inner: tt.pad}}
			v2 := &Constant{&syn.ByteString{Inner: tt.arg1}}
			v3 := &Constant{&syn.ByteString{Inner: tt.arg2}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)
			b = b.ApplyArg(v3)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			result := expectByteString(t, constVal)

			if !bytes.Equal(result.Inner, tt.expected) {
				t.Errorf("expected %x, got %x", tt.expected, result.Inner)
			}
		})
	}
}

func TestIntegerToByteStringBuiltin(t *testing.T) {
	tests := []struct {
		name      string
		endian    bool
		size      int64
		input     int64
		expected  []byte
		shouldErr bool
	}{
		{
			name:      "big-endian, exact size",
			endian:    true,
			size:      2,
			input:     0xABCD,
			expected:  []byte{0xAB, 0xCD},
			shouldErr: false,
		},
		{
			name:      "little-endian, exact size",
			endian:    false,
			size:      2,
			input:     0xABCD,
			expected:  []byte{0xCD, 0xAB},
			shouldErr: false,
		},
		{
			name:      "big-endian, padding",
			endian:    true,
			size:      4,
			input:     0xABCD,
			expected:  []byte{0x00, 0x00, 0xAB, 0xCD},
			shouldErr: false,
		},
		{
			name:      "zero input",
			endian:    true,
			size:      2,
			input:     0,
			expected:  []byte{0x00, 0x00},
			shouldErr: false,
		},
		{
			name:      "negative size",
			endian:    true,
			size:      -1,
			input:     0xABCD,
			expected:  nil,
			shouldErr: true,
		},
		{
			name:      "negative input",
			endian:    true,
			size:      2,
			input:     -1,
			expected:  nil,
			shouldErr: true,
		},
		{
			name:      "input too large for size",
			endian:    true,
			size:      1,
			input:     0xABCD,
			expected:  nil,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.IntegerToByteString)

			v1 := &Constant{&syn.Bool{Inner: tt.endian}}
			v2 := &Constant{&syn.Integer{Inner: big.NewInt(tt.size)}}
			v3 := &Constant{&syn.Integer{Inner: big.NewInt(tt.input)}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)
			b = b.ApplyArg(v3)

			val, err := evalBuiltinWithError(t, m, b)
			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			constVal := expectConstant(t, val)
			result := expectByteString(t, constVal)

			if !bytes.Equal(result.Inner, tt.expected) {
				t.Errorf("expected %x, got %x", tt.expected, result.Inner)
			}
		})
	}
}

func TestByteStringToIntegerBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		endian   bool
		input    []byte
		expected int64
	}{
		{
			name:     "big-endian",
			endian:   true,
			input:    []byte{0xAB, 0xCD},
			expected: 0xABCD,
		},
		{
			name:     "little-endian",
			endian:   false,
			input:    []byte{0xCD, 0xAB},
			expected: 0xABCD,
		},
		{
			name:     "zero bytes",
			endian:   true,
			input:    []byte{},
			expected: 0,
		},
		{
			name:     "single byte",
			endian:   true,
			input:    []byte{0x42},
			expected: 0x42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.ByteStringToInteger)

			v1 := &Constant{&syn.Bool{Inner: tt.endian}}
			v2 := &Constant{&syn.ByteString{Inner: tt.input}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			result := expectInteger(t, constVal)

			if result.Inner.Cmp(big.NewInt(tt.expected)) != 0 {
				t.Errorf("expected %d, got %v", tt.expected, result.Inner)
			}
		})
	}
}

func TestCountSetBitsBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected int64
	}{
		{
			name:     "empty bytes",
			input:    []byte{},
			expected: 0,
		},
		{
			name:     "single byte with 4 bits set",
			input:    []byte{0b10101010}, // 170 = 0xAA
			expected: 4,
		},
		{
			name:     "multiple bytes",
			input:    []byte{0xFF, 0x00, 0x0F}, // 8 + 0 + 4 = 12
			expected: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.CountSetBits)

			v1 := &Constant{&syn.ByteString{Inner: tt.input}}

			b = b.ApplyArg(v1)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			result := expectInteger(t, constVal)

			if result.Inner.Cmp(big.NewInt(tt.expected)) != 0 {
				t.Errorf("expected %d, got %v", tt.expected, result.Inner)
			}
		})
	}
}

func TestFindFirstSetBitBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected int64
	}{
		{
			name:     "empty bytes",
			input:    []byte{},
			expected: -1,
		},
		{
			name:     "first bit set in first byte",
			input:    []byte{0b00000001}, // bit 0 set
			expected: 0,
		},
		{
			name:     "bit 6 set in first byte",
			input:    []byte{0b01000000}, // bit 6 set
			expected: 6,
		},
		{
			name: "last bit set in first byte",
			input: []byte{
				0b10000000,
			}, // bit 7 set
			expected: 7,
		},
		{
			name:     "bit set in second byte",
			input:    []byte{0x00, 0b00000001}, // bit 0 in second byte = bit 0
			expected: 0,
		},
		{
			name:     "all zeros",
			input:    []byte{0x00, 0x00},
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.FindFirstSetBit)

			v1 := &Constant{&syn.ByteString{Inner: tt.input}}

			b = b.ApplyArg(v1)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			result := expectInteger(t, constVal)

			if result.Inner.Cmp(big.NewInt(tt.expected)) != 0 {
				t.Errorf("expected %d, got %v", tt.expected, result.Inner)
			}
		})
	}
}

func TestComplementByteStringBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "empty bytes",
			input:    []byte{},
			expected: []byte{},
		},
		{
			name:     "single byte",
			input:    []byte{0xAA}, // 10101010
			expected: []byte{0x55}, // 01010101
		},
		{
			name:     "multiple bytes",
			input:    []byte{0x00, 0xFF, 0x0F},
			expected: []byte{0xFF, 0x00, 0xF0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.ComplementByteString)

			v1 := &Constant{&syn.ByteString{Inner: tt.input}}

			b = b.ApplyArg(v1)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			result := expectByteString(t, constVal)

			if !bytes.Equal(result.Inner, tt.expected) {
				t.Errorf("expected %x, got %x", tt.expected, result.Inner)
			}
		})
	}
}

func TestReadBitBuiltin(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		bitIndex  int64
		expected  bool
		shouldErr bool
	}{
		{
			name:      "read bit 0 from single byte",
			input:     []byte{0b00000001}, // bit 0 set
			bitIndex:  0,
			expected:  true,
			shouldErr: false,
		},
		{
			name:      "read bit 7 from single byte",
			input:     []byte{0b10000000}, // bit 7 set
			bitIndex:  7,
			expected:  true,
			shouldErr: false,
		},
		{
			name: "read bit 0 from second byte",
			input: []byte{
				0b00000001,
				0x00,
			}, // bit 8 set (bit 0 of first byte in little-endian)
			bitIndex:  8,
			expected:  true,
			shouldErr: false,
		},
		{
			name:      "read unset bit",
			input:     []byte{0b11111110}, // all bits except 0 set
			bitIndex:  0,
			expected:  false,
			shouldErr: false,
		},
		{
			name:      "bit index out of bounds",
			input:     []byte{0xFF},
			bitIndex:  8,
			expected:  false,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.ReadBit)

			v1 := &Constant{&syn.ByteString{Inner: tt.input}}
			v2 := &Constant{&syn.Integer{Inner: big.NewInt(tt.bitIndex)}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)

			val, err := evalBuiltinWithError(t, m, b)
			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			constVal := expectConstant(t, val)
			result := expectBool(t, constVal)

			if result.Inner != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result.Inner)
			}
		})
	}
}

func TestReplicateByteBuiltin(t *testing.T) {
	tests := []struct {
		name      string
		size      int64
		byteVal   int64
		expected  []byte
		shouldErr bool
	}{
		{
			name:      "replicate 0xFF three times",
			size:      3,
			byteVal:   0xFF,
			expected:  []byte{0xFF, 0xFF, 0xFF},
			shouldErr: false,
		},
		{
			name:      "replicate 0x00 five times",
			size:      5,
			byteVal:   0x00,
			expected:  []byte{0x00, 0x00, 0x00, 0x00, 0x00},
			shouldErr: false,
		},
		{
			name:      "size zero",
			size:      0,
			byteVal:   0x42,
			expected:  []byte{},
			shouldErr: false,
		},
		{
			name:      "byte value out of bounds",
			size:      2,
			byteVal:   256,
			expected:  nil,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.ReplicateByte)

			v1 := &Constant{&syn.Integer{Inner: big.NewInt(tt.size)}}
			v2 := &Constant{&syn.Integer{Inner: big.NewInt(tt.byteVal)}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)

			val, err := evalBuiltinWithError(t, m, b)
			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			constVal := expectConstant(t, val)
			result := expectByteString(t, constVal)

			if !bytes.Equal(result.Inner, tt.expected) {
				t.Errorf("expected %x, got %x", tt.expected, result.Inner)
			}
		})
	}
}

func TestLessThanByteStringBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		arg1     []byte
		arg2     []byte
		expected bool
	}{
		{"less than true", []byte{1, 2}, []byte{1, 3}, true},
		{"less than false", []byte{1, 3}, []byte{1, 2}, false},
		{"equal", []byte{1, 2}, []byte{1, 2}, false},
		{"different lengths", []byte{1}, []byte{1, 2}, true},
		{"empty vs non-empty", []byte{}, []byte{1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.LessThanByteString)

			v1 := &Constant{&syn.ByteString{Inner: tt.arg1}}
			v2 := &Constant{&syn.ByteString{Inner: tt.arg2}}

			b = b.ApplyArg(v1)
			b = b.ApplyArg(v2)

			val := evalBuiltin(t, m, b)
			constVal := expectConstant(t, val)
			boolVal := expectBool(t, constVal)

			if boolVal.Inner != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, boolVal.Inner)
			}
		})
	}
}

func TestMultiIndexArrayBuiltin(t *testing.T) {
	tests := []struct {
		name     string
		indices  []int64
		array    []syn.IConstant
		expected []syn.IConstant
		hasError bool
	}{
		{
			name:    "empty indices",
			indices: []int64{},
			array: []syn.IConstant{
				&syn.Integer{Inner: big.NewInt(1)},
				&syn.Integer{Inner: big.NewInt(2)},
			},
			expected: []syn.IConstant{},
			hasError: false,
		},
		{
			name:    "single index",
			indices: []int64{1},
			array: []syn.IConstant{
				&syn.Integer{Inner: big.NewInt(10)},
				&syn.Integer{Inner: big.NewInt(20)},
				&syn.Integer{Inner: big.NewInt(30)},
			},
			expected: []syn.IConstant{&syn.Integer{Inner: big.NewInt(20)}},
			hasError: false,
		},
		{
			name:    "multiple indices",
			indices: []int64{0, 2, 1},
			array: []syn.IConstant{
				&syn.Integer{Inner: big.NewInt(100)},
				&syn.Integer{Inner: big.NewInt(200)},
				&syn.Integer{Inner: big.NewInt(300)},
			},
			expected: []syn.IConstant{
				&syn.Integer{Inner: big.NewInt(100)},
				&syn.Integer{Inner: big.NewInt(300)},
				&syn.Integer{Inner: big.NewInt(200)},
			},
			hasError: false,
		},
		{
			name:    "duplicate indices",
			indices: []int64{1, 1, 0},
			array: []syn.IConstant{
				&syn.Integer{Inner: big.NewInt(5)},
				&syn.Integer{Inner: big.NewInt(15)},
			},
			expected: []syn.IConstant{
				&syn.Integer{Inner: big.NewInt(15)},
				&syn.Integer{Inner: big.NewInt(15)},
				&syn.Integer{Inner: big.NewInt(5)},
			},
			hasError: false,
		},
		{
			name:     "out of bounds negative",
			indices:  []int64{-1},
			array:    []syn.IConstant{&syn.Integer{Inner: big.NewInt(1)}},
			expected: nil,
			hasError: true,
		},
		{
			name:    "out of bounds too large",
			indices: []int64{5},
			array: []syn.IConstant{
				&syn.Integer{Inner: big.NewInt(1)},
				&syn.Integer{Inner: big.NewInt(2)},
			},
			expected: nil,
			hasError: true,
		},
		{
			name:    "non-integer index element",
			indices: []int64{},
			array: []syn.IConstant{
				&syn.Integer{Inner: big.NewInt(1)},
				&syn.Integer{Inner: big.NewInt(2)},
			},
			expected: nil,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMachine()
			b := newTestBuiltin(builtin.MultiIndexArray)

			// Create indices list
			var indicesList *Constant
			if tt.name == "non-integer index element" {
				// Create a list that contains a non-integer element (ByteString) to hit the error branch
				indices := []syn.IConstant{&syn.ByteString{Inner: []byte{0x01}}}
				indicesList = &Constant{
					&syn.ProtoList{LTyp: &syn.TByteString{}, List: indices},
				}
			} else {
				indices := make([]syn.IConstant, len(tt.indices))
				for i, idx := range tt.indices {
					indices[i] = &syn.Integer{Inner: big.NewInt(idx)}
				}
				indicesList = &Constant{&syn.ProtoList{LTyp: &syn.TInteger{}, List: indices}}
			}

			// Create array
			array := &Constant{&syn.ProtoList{List: tt.array}}

			b = b.ApplyArg(indicesList)
			b = b.ApplyArg(array)

			val, err := evalBuiltinWithError(t, m, b)
			if tt.hasError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			constVal := expectConstant(t, val)
			resultList := expectProtoList(t, constVal)

			if len(resultList.List) != len(tt.expected) {
				t.Errorf(
					"expected list length %d, got %d",
					len(tt.expected),
					len(resultList.List),
				)
			}

			for i, expected := range tt.expected {
				if i >= len(resultList.List) {
					t.Errorf("result list too short")
					break
				}
				actual := resultList.List[i]
				expectedInt, ok := expected.(*syn.Integer)
				if !ok {
					t.Errorf("expected element %d is not an integer", i)
					continue
				}
				actualInt, ok := actual.(*syn.Integer)
				if !ok {
					t.Errorf("actual element %d is not an integer", i)
					continue
				}
				if expectedInt.Inner.Cmp(actualInt.Inner) != 0 {
					t.Errorf(
						"at index %d, expected %v, got %v",
						i,
						expectedInt.Inner,
						actualInt.Inner,
					)
				}
			}
		})
	}
}

func TestBuiltinArgsIter(t *testing.T) {
	// Create some test values
	val1 := Constant{Constant: &syn.Integer{Inner: big.NewInt(1)}}
	val2 := Constant{Constant: &syn.Integer{Inner: big.NewInt(2)}}
	val3 := Constant{Constant: &syn.Integer{Inner: big.NewInt(3)}}

	// Create a chain: val3 -> val2 -> val1
	args := &BuiltinArgs[syn.DeBruijn]{data: val1, next: nil}
	args = &BuiltinArgs[syn.DeBruijn]{data: val2, next: args}
	args = &BuiltinArgs[syn.DeBruijn]{data: val3, next: args}

	// Collect values using Iter
	var collected []Value[syn.DeBruijn]
	for v := range args.Iter() {
		collected = append(collected, v)
	}

	// Iter reverses the order, so should be val1, val2, val3
	expected := []Value[syn.DeBruijn]{val1, val2, val3}
	if len(collected) != len(expected) {
		t.Fatalf("expected %d values, got %d", len(expected), len(collected))
	}
	for i, v := range collected {
		collectedInt := v.(Constant).Constant.(*syn.Integer)
		expectedInt := expected[i].(Constant).Constant.(*syn.Integer)
		if collectedInt.Inner.Cmp(expectedInt.Inner) != 0 {
			t.Errorf(
				"at index %d, expected %v, got %v",
				i,
				expectedInt.Inner,
				collectedInt.Inner,
			)
		}
	}
}
