package cek

import (
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/lang"
	"github.com/blinklabs-io/plutigo/syn"
)

func callMultiIndexArray(t *testing.T, array, indices []syn.IConstant) (Value[syn.DeBruijn], error) {
	t.Helper()
	b := newTestBuiltin(builtin.Builtins["multiIndexArray"])
	b = b.ApplyArg(&Constant{&syn.ProtoArray{ATyp: &syn.TInteger{}, Array: array}})
	b = b.ApplyArg(&Constant{&syn.ProtoList{LTyp: &syn.TInteger{}, List: indices}})
	return multiIndexArray(newTestMachineV4(), b)
}

func TestMultiIndexArrayParses(t *testing.T) {
	if _, err := syn.Parse("(program 1.3.0 (force (builtin multiIndexArray)))"); err != nil {
		t.Fatalf("multiIndexArray did not parse in V4: %v", err)
	}
}

func TestMultiIndexArrayReferenceVectors(t *testing.T) {
	fn, ok := builtin.Builtins["multiIndexArray"]
	if !ok {
		t.Fatal("multiIndexArray is not registered")
	}

	array := &Constant{&syn.ProtoArray{
		ATyp: &syn.TInteger{},
		Array: []syn.IConstant{
			&syn.Integer{Inner: big.NewInt(10)},
			&syn.Integer{Inner: big.NewInt(20)},
			&syn.Integer{Inner: big.NewInt(30)},
		},
	}}
	indices := &Constant{&syn.ProtoList{
		LTyp: &syn.TInteger{},
		List: []syn.IConstant{
			&syn.Integer{Inner: big.NewInt(2)},
			&syn.Integer{Inner: big.NewInt(0)},
			&syn.Integer{Inner: big.NewInt(0)},
			&syn.Integer{Inner: big.NewInt(1)},
		},
	}}

	b := newTestBuiltin(fn)
	b = b.ApplyArg(array)
	b = b.ApplyArg(indices)
	value, err := multiIndexArray(newTestMachineV4(), b)
	if err != nil {
		t.Fatalf("multiIndexArray returned error: %v", err)
	}

	result, ok := expectConstant(t, value).Constant.(*syn.ProtoList)
	if !ok {
		t.Fatalf("result type = %T, want *syn.ProtoList", expectConstant(t, value).Constant)
	}
	want := []int64{30, 10, 10, 20}
	if len(result.List) != len(want) {
		t.Fatalf("result length = %d, want %d", len(result.List), len(want))
	}
	for i, got := range result.List {
		gotInt, ok := got.(*syn.Integer)
		if !ok {
			t.Fatalf("result[%d] has type %T, want integer", i, got)
		}
		if gotInt.Inner.Int64() != want[i] {
			t.Errorf("result[%d] = %d, want %d", i, gotInt.Inner.Int64(), want[i])
		}
	}
}

func TestMultiIndexArrayRejectsMoreThanMaximumIndices(t *testing.T) {
	fn, ok := builtin.Builtins["multiIndexArray"]
	if !ok {
		t.Fatal("multiIndexArray is not registered")
	}

	indices := make([]syn.IConstant, 1025)
	for i := range indices {
		indices[i] = &syn.Integer{Inner: big.NewInt(0)}
	}
	array := &Constant{&syn.ProtoArray{
		ATyp:  &syn.TInteger{},
		Array: []syn.IConstant{&syn.Integer{Inner: big.NewInt(42)}},
	}}
	b := newTestBuiltin(fn)
	b = b.ApplyArg(array)
	b = b.ApplyArg(&Constant{&syn.ProtoList{
		LTyp: &syn.TInteger{},
		List: indices,
	}})
	_, err := multiIndexArray(newTestMachineV4(), b)
	if err == nil {
		t.Fatal("multiIndexArray accepted more than 1024 indices")
	}
	builtinErr, ok := err.(*BuiltinError)
	if !ok {
		t.Fatalf("error type = %T, want *BuiltinError", err)
	}
	if builtinErr.Code != ErrCodeOutOfBounds {
		t.Fatalf("error code = %v, want %v", builtinErr.Code, ErrCodeOutOfBounds)
	}
	if builtinErr.Message != "too many indices (maximum is 1024)" {
		t.Fatalf("error message = %q, want maximum-index error", builtinErr.Message)
	}
}

func TestMultiIndexArrayAcceptsMaximumIndices(t *testing.T) {
	fn := builtin.Builtins["multiIndexArray"]
	indices := make([]syn.IConstant, multiIndexArrayMaximumIndexCount)
	for i := range indices {
		indices[i] = &syn.Integer{Inner: big.NewInt(0)}
	}

	b := newTestBuiltin(fn)
	b = b.ApplyArg(&Constant{&syn.ProtoArray{
		ATyp:  &syn.TInteger{},
		Array: []syn.IConstant{&syn.Integer{Inner: big.NewInt(42)}},
	}})
	b = b.ApplyArg(&Constant{&syn.ProtoList{
		LTyp: &syn.TInteger{},
		List: indices,
	}})
	value, err := multiIndexArray(newTestMachineV4(), b)
	if err != nil {
		t.Fatalf("multiIndexArray rejected the maximum index count: %v", err)
	}
	result, ok := expectConstant(t, value).Constant.(*syn.ProtoList)
	if !ok {
		t.Fatalf("result type = %T, want *syn.ProtoList", expectConstant(t, value).Constant)
	}
	if len(result.List) != multiIndexArrayMaximumIndexCount {
		t.Fatalf("result length = %d, want %d", len(result.List), multiIndexArrayMaximumIndexCount)
	}
}

func TestMultiIndexArrayRejectsInvalidIndices(t *testing.T) {
	twoTo64 := new(big.Int).Lsh(big.NewInt(1), 64)
	tests := []struct {
		name    string
		index   syn.IConstant
		code    ErrorCode
		message string
	}{
		{"negative", &syn.Integer{Inner: big.NewInt(-1)}, ErrCodeOutOfBounds, "negative index"},
		{"overflow", &syn.Integer{Inner: twoTo64}, ErrCodeOverflow, "index too large"},
		{"out of bounds", &syn.Integer{Inner: big.NewInt(1)}, ErrCodeOutOfBounds, "index 1 out of bounds for array of length 1"},
		{"wrong type", &syn.ByteString{}, ErrCodeTypeMismatch, "type mismatch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := callMultiIndexArray(t,
				[]syn.IConstant{&syn.Integer{Inner: big.NewInt(42)}},
				[]syn.IConstant{tt.index},
			)
			if err == nil {
				t.Fatal("multiIndexArray accepted invalid index")
			}
			if tt.code == ErrCodeTypeMismatch {
				typeErr, ok := err.(*TypeError)
				if !ok {
					t.Fatalf("error type = %T, want *TypeError", err)
				}
				if typeErr.Code != tt.code || typeErr.Message != tt.message {
					t.Fatalf("error = (%v, %q), want (%v, %q)", typeErr.Code, typeErr.Message, tt.code, tt.message)
				}
				return
			}
			builtinErr, ok := err.(*BuiltinError)
			if !ok {
				t.Fatalf("error type = %T, want *BuiltinError", err)
			}
			if builtinErr.Code != tt.code || builtinErr.Message != tt.message {
				t.Fatalf("error = (%v, %q), want (%v, %q)", builtinErr.Code, builtinErr.Message, tt.code, tt.message)
			}
		})
	}
}

func TestV4CostModelParameters(t *testing.T) {
	names := lang.GetParamNamesForVersion(lang.LanguageVersionV4)
	wantNames := []string{
		"multiIndexArray-cpu-arguments-c0",
		"multiIndexArray-cpu-arguments-c1",
		"multiIndexArray-cpu-arguments-c2",
		"multiIndexArray-memory-arguments-intercept",
		"multiIndexArray-memory-arguments-slope",
		"assetCount-cpu-arguments",
		"assetCount-memory-arguments",
	}
	for _, want := range wantNames {
		found := false
		for _, name := range names {
			if name == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("V4 parameter list is missing %q", want)
		}
	}

	params := make(map[string]int64, len(wantNames))
	params["multiIndexArray-cpu-arguments-c0"] = 326163
	params["multiIndexArray-cpu-arguments-c1"] = 12304
	params["multiIndexArray-cpu-arguments-c2"] = 2
	params["multiIndexArray-memory-arguments-intercept"] = 4
	params["multiIndexArray-memory-arguments-slope"] = 3
	_, err := costModelFromMap(lang.LanguageVersionV4, SemanticsVariantE, params)
	if err != nil {
		t.Fatalf("V4 cost model parameters were rejected: %v", err)
	}
}

func TestV4CostModelLoadsCompleteParameterList(t *testing.T) {
	params := make([]int64, len(lang.CostModelParamNamesV4))
	for i, name := range lang.CostModelParamNamesV4 {
		switch name {
		case "assetCount-cpu-arguments":
			params[i] = 17
		case "assetCount-memory-arguments":
			params[i] = 23
		}
	}
	context, err := NewEvalContext(
		lang.LanguageVersionV4,
		ProtoVersion{Major: 12},
		params,
	)
	if err != nil {
		t.Fatalf("full V4 cost model was rejected: %v", err)
	}
	assetCountCosts := context.CostModel.builtinCosts[builtin.AssetCount]
	if got := assetCountCosts.cpu.(*ConstantCost).c; got != 17 {
		t.Fatalf("assetCount CPU cost = %d, want 17", got)
	}
	if got := assetCountCosts.mem.(*ConstantCost).c; got != 23 {
		t.Fatalf("assetCount memory cost = %d, want 23", got)
	}
}

func TestMultiIndexArrayIsUnavailableWithoutProtocolVersion(t *testing.T) {
	machine := NewMachine[syn.DeBruijn](lang.LanguageVersionV4, 0, nil)
	if machine.builtins[builtin.MultiIndexArray] != nil {
		t.Fatal("multiIndexArray is available without an activating protocol version")
	}
}

func testValueEntry(policy string, tokenNames ...string) syn.IConstant {
	tokens := make([]syn.IConstant, 0, len(tokenNames))
	for _, tokenName := range tokenNames {
		tokens = append(tokens, &syn.ProtoPair{
			FstType: &syn.TByteString{},
			SndType: &syn.TInteger{},
			First:   &syn.ByteString{Inner: []byte(tokenName)},
			Second:  &syn.Integer{Inner: big.NewInt(1)},
		})
	}
	return &syn.ProtoPair{
		FstType: &syn.TByteString{},
		SndType: &syn.TList{Typ: &syn.TPair{
			First:  &syn.TByteString{},
			Second: &syn.TInteger{},
		}},
		First: &syn.ByteString{Inner: []byte(policy)},
		Second: &syn.ProtoList{
			LTyp: &syn.TPair{First: &syn.TByteString{}, Second: &syn.TInteger{}},
			List: tokens,
		},
	}
}

func TestPoliciesAndAssetCountMatchValueSemantics(t *testing.T) {
	value := &Constant{&syn.Value{Entries: []syn.IConstant{
		testValueEntry("alpha", "a", "b"),
		testValueEntry("zeta", "c"),
	}}}

	policyMachine := newTestMachineV4()
	policyMachine.ExBudget = ExBudget{Cpu: 100000000001, Mem: 100000000001}
	policyBuiltin := newTestBuiltin(builtin.Policies).ApplyArg(value)
	policyValue, err := policies(policyMachine, policyBuiltin)
	if err != nil {
		t.Fatalf("policies returned error: %v", err)
	}
	policyList, ok := expectConstant(t, policyValue).Constant.(*syn.ProtoList)
	if !ok {
		t.Fatalf("policies result type = %T, want *syn.ProtoList", expectConstant(t, policyValue).Constant)
	}
	if len(policyList.List) != 2 {
		t.Fatalf("policies result length = %d, want 2", len(policyList.List))
	}
	for i, want := range []string{"alpha", "zeta"} {
		got, ok := policyList.List[i].(*syn.ByteString)
		if !ok {
			t.Fatalf("policies result[%d] type = %T, want *syn.ByteString", i, policyList.List[i])
		}
		if string(got.Inner) != want {
			t.Fatalf("policies result[%d] = %q, want %q", i, string(got.Inner), want)
		}
	}

	assetBuiltin := newTestBuiltin(builtin.AssetCount).ApplyArg(value)
	assetValue, err := assetCount(newTestMachineV4(), assetBuiltin)
	if err != nil {
		t.Fatalf("assetCount returned error: %v", err)
	}
	if got := expectInteger(t, expectConstant(t, assetValue)).Inner.Int64(); got != 3 {
		t.Fatalf("assetCount = %d, want 3", got)
	}
}
