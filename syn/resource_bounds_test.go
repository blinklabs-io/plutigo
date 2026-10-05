package syn

import (
	"github.com/blinklabs-io/plutigo/data"
	"math/big"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/lang"
)

func errorFields(n int) []Term[DeBruijn] {
	fields := make([]Term[DeBruijn], n)
	for i := range fields {
		fields[i] = &Error{}
	}
	return fields
}

func encodeV110(t *testing.T, term Term[DeBruijn]) []byte {
	t.Helper()
	encoded, err := Encode(&Program[DeBruijn]{
		Version: lang.LanguageVersion{1, 1, 0},
		Term:    term,
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return encoded
}

// decodeAll runs every FLAT decode entry point over input.
func decodeAll(input []byte) map[string]error {
	out := map[string]error{}
	_, out["DecodeDeBruijn"] = DecodeDeBruijn(input)
	_, out["Decode[DeBruijn]"] = Decode[DeBruijn](input)
	_, out["Decode[NamedDeBruijn]"] = Decode[NamedDeBruijn](input)
	return out
}

func requireErrContains(t *testing.T, path string, err error, want string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected error containing %q, got nil", path, want)
		return
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("%s: expected error containing %q, got: %v", path, want, err)
	}
}

func TestFlatDecodeRejectsOversizedInput(t *testing.T) {
	t.Parallel()
	for path, err := range decodeAll(make([]byte, maxInputBytes+1)) {
		requireErrContains(t, path, err, "input too large")
	}
}

func TestFlatDecodeNodeLimit(t *testing.T) {
	t.Parallel()
	groups := maxProgramNodes/maxCollectionWidth + 1
	outer := make([]Term[DeBruijn], groups)
	for i := range outer {
		outer[i] = &Constr[DeBruijn]{Fields: errorFields(maxCollectionWidth)}
	}
	encoded := encodeV110(t, &Constr[DeBruijn]{Fields: outer})
	for path, err := range decodeAll(encoded) {
		requireErrContains(t, path, err, "too many nodes")
	}
}

func TestFlatDecodeCollectionWidthLimit(t *testing.T) {
	t.Parallel()
	tooWide := maxCollectionWidth + 1
	ints := make([]IConstant, tooWide)
	for i := range ints {
		ints[i] = &Integer{Inner: big.NewInt(0)}
	}
	tests := map[string]Term[DeBruijn]{
		"constr fields": &Constr[DeBruijn]{Fields: errorFields(tooWide)},
		"case branches": &Case[DeBruijn]{
			Constr:   &Error{},
			Branches: errorFields(tooWide),
		},
		"constant list": &Constant{Con: &ProtoList{
			LTyp: &TInteger{}, List: ints,
		}},
	}
	for name, term := range tests {
		encoded := encodeV110(t, term)
		for path, err := range decodeAll(encoded) {
			requireErrContains(t, name+"/"+path, err, "too many")
		}
	}

	// The limit is inclusive: a collection of exactly the maximum width decodes.
	encoded := encodeV110(
		t,
		&Constr[DeBruijn]{Fields: errorFields(maxCollectionWidth)},
	)
	for path, err := range decodeAll(encoded) {
		if err != nil {
			t.Errorf("%s: width at the limit should decode, got: %v", path, err)
		}
	}
}

// TestFlatDecodeProtocolConstrBoundBeforeAllocation truncates the input in the
// middle of an over-wide field list. A decoder that checks the protocol bound
// only after reading the whole list fails on the truncation instead.
func TestFlatDecodeProtocolConstrBoundBeforeAllocation(t *testing.T) {
	t.Parallel()
	encoded := encodeV110(
		t,
		&Constr[DeBruijn]{Fields: errorFields(maxConstrFieldsPV11 + 100)},
	)
	truncated := encoded[:len(encoded)-20]
	context := ProgramContext{
		LedgerLanguage: lang.LanguageVersionV3,
		ProtocolMajor:  builtin.VanRossemProtoVersion,
	}
	_, err := DecodeWithContext[DeBruijn](truncated, context)
	requireErrContains(t, "DecodeWithContext[DeBruijn]", err, "too many")
	_, err = DecodeWithContext[NamedDeBruijn](truncated, context)
	requireErrContains(t, "DecodeWithContext[NamedDeBruijn]", err, "too many")

	// Before protocol version 11 the same fields are only bounded by the
	// default width, so the truncation is what fails.
	context.ProtocolMajor = builtin.VanRossemProtoVersion - 1
	_, err = DecodeWithContext[DeBruijn](truncated, context)
	if err == nil || strings.Contains(err.Error(), "too many") {
		t.Errorf("pre-PV11 decode should not apply the PV11 bound, got: %v", err)
	}
}

func TestParseRejectsOversizedInput(t *testing.T) {
	t.Parallel()
	_, err := Parse(strings.Repeat(" ", maxInputBytes+1))
	requireErrContains(t, "Parse", err, "input too large")
}

func TestParseNodeLimit(t *testing.T) {
	t.Parallel()
	group := "(constr 0" + strings.Repeat(" (error)", maxCollectionWidth) + ")"
	groups := maxProgramNodes/maxCollectionWidth + 1
	input := "(program 1.1.0 (constr 0 " + strings.Repeat(group+" ", groups) + "))"
	_, err := Parse(input)
	requireErrContains(t, "Parse", err, "too many nodes")
}

func TestParseCollectionWidthLimit(t *testing.T) {
	t.Parallel()
	fields := strings.Repeat(" (error)", maxCollectionWidth+1)
	listItems := strings.TrimSuffix(strings.Repeat("0,", maxCollectionWidth+1), ",")
	tests := map[string]string{
		"constr fields": "(program 1.1.0 (constr 0" + fields + "))",
		"case branches": "(program 1.1.0 (case (error)" + fields + "))",
		"constant list": "(program 1.1.0 (con (list integer) [" + listItems + "]))",
	}
	for name, input := range tests {
		_, err := Parse(input)
		requireErrContains(t, name, err, "too many")
	}
}

func TestParseWithContextProtocolConstrBoundBeforeAllocation(t *testing.T) {
	t.Parallel()
	// The program is never closed: a parser that applies the protocol bound
	// only through ValidateProgram fails on the missing parenthesis instead.
	input := "(program 1.1.0 (constr 0" +
		strings.Repeat(" (error)", maxConstrFieldsPV11+1)
	context := ProgramContext{
		LedgerLanguage: lang.LanguageVersionV3,
		ProtocolMajor:  builtin.VanRossemProtoVersion,
	}
	_, err := ParseWithContext(input, context)
	requireErrContains(t, "ParseWithContext", err, "too many")
}

func TestPrettyDepthLimit(t *testing.T) {
	t.Parallel()
	term := buildNestedDelay(50)
	limits := PrettyLimits{MaxDepth: 10, MaxOutputBytes: 1 << 20}
	out, err := PrettyTermWithLimits[DeBruijn](term, limits)
	requireErrContains(t, "PrettyTermWithLimits", err, "depth")
	if out != "" {
		t.Errorf("expected no output on error, got %d bytes", len(out))
	}

	limits.MaxDepth = 60
	if _, err := PrettyTermWithLimits[DeBruijn](term, limits); err != nil {
		t.Errorf("nesting within the depth limit should format, got: %v", err)
	}
}

func TestPrettyOutputLimitBoundsAllocation(t *testing.T) {
	t.Parallel()
	const maxBytes = 4096
	pp := newLimitedPrettyPrinter(2, PrettyLimits{
		MaxDepth: maxFlatDecodeDepth, MaxOutputBytes: maxBytes,
	})
	// Indentation of a unary chain this deep is quadratic: ~40 MB unbounded.
	printTerm[DeBruijn](pp, buildNestedDelay(2000))
	if pp.err == nil || !strings.Contains(pp.err.Error(), "output") {
		t.Errorf("expected output limit error, got: %v", pp.err)
	}
	if got := pp.builder.Len(); got > maxBytes {
		t.Errorf("builder holds %d bytes, limit is %d", got, maxBytes)
	}
}

func TestPrettyWithLimitsMatchesPrettyForShallowPrograms(t *testing.T) {
	t.Parallel()
	program := &Program[DeBruijn]{
		Version: lang.LanguageVersionV1,
		Term: &Lambda[DeBruijn]{Body: &Apply[DeBruijn]{
			Function: &Delay[DeBruijn]{Term: &Error{}},
			Argument: &Constant{Con: &Integer{Inner: big.NewInt(7)}},
		}},
	}
	got, err := PrettyWithLimits(program, PrettyLimits{
		MaxDepth: 100, MaxOutputBytes: 1 << 20,
	})
	if err != nil {
		t.Fatalf("PrettyWithLimits: %v", err)
	}
	if want := Pretty(program); got != want {
		t.Errorf("PrettyWithLimits = %q, Pretty = %q", got, want)
	}
}

// TestPrettyDefaultsStopDeepTerms checks that the convenience formatter
// terminates on a term whose unbounded rendering would be gigabytes.
func TestPrettyDefaultsStopDeepTerms(t *testing.T) {
	t.Parallel()
	term := buildNestedDelay(maxFlatDecodeDepth - 1)
	if out := PrettyTerm[DeBruijn](term); len(out) > defaultPrettyMaxOutputBytes {
		t.Errorf("PrettyTerm produced %d bytes, default limit is %d", len(out), defaultPrettyMaxOutputBytes)
	}
	if _, err := PrettyTermWithLimits[DeBruijn](term, PrettyLimits{}); err == nil {
		t.Error("default limits should reject a 100k-deep unary term")
	}
}

func nameTerm(unique Unique) Name {
	return Name{Text: "x", Unique: unique}
}

func TestNameToDeBruijnShadowingIndices(t *testing.T) {
	t.Parallel()
	x := nameTerm(0)
	y := nameTerm(1)
	// \x -> \y -> \x -> [x y]  with the inner x shadowing the outer one,
	// followed by a use of the outer x after the shadowing scope closed.
	term := &Lambda[Name]{ParameterName: x, Body: &Constr[Name]{
		Fields: []Term[Name]{
			&Lambda[Name]{ParameterName: y, Body: &Lambda[Name]{
				ParameterName: x,
				Body: &Apply[Name]{
					Function: &Var[Name]{Name: x},
					Argument: &Var[Name]{Name: y},
				},
			}},
			&Var[Name]{Name: x},
		},
	}}
	got, err := NameToDeBruijn(&Program[Name]{Term: term})
	if err != nil {
		t.Fatalf("NameToDeBruijn: %v", err)
	}
	want := &Lambda[DeBruijn]{ParameterName: 0, Body: &Constr[DeBruijn]{
		Fields: []Term[DeBruijn]{
			&Lambda[DeBruijn]{ParameterName: 0, Body: &Lambda[DeBruijn]{
				ParameterName: 0,
				Body: &Apply[DeBruijn]{
					Function: &Var[DeBruijn]{Name: 1},
					Argument: &Var[DeBruijn]{Name: 2},
				},
			}},
			&Var[DeBruijn]{Name: 1},
		},
	}}
	if g, w := PrettyTerm[DeBruijn](got.Term), PrettyTerm[DeBruijn](want); g != w {
		t.Errorf("got:\n%s\nwant:\n%s", g, w)
	}

	free := &Program[Name]{Term: &Var[Name]{Name: x}}
	if _, err := NameToDeBruijn(free); err == nil {
		t.Error("free variable should fail conversion")
	}
	// A name bound only inside a sibling scope is free outside of it.
	sibling := &Program[Name]{Term: &Constr[Name]{Fields: []Term[Name]{
		&Lambda[Name]{ParameterName: x, Body: &Var[Name]{Name: x}},
		&Var[Name]{Name: x},
	}}}
	if _, err := NameToDeBruijn(sibling); err == nil {
		t.Error("name used after its scope closed should fail conversion")
	}
}

// TestNameToDeBruijnLinearInDepth references the outermost binder from the
// bottom of a deep lambda chain many times. Resolving each reference by
// walking the enclosing scopes is quadratic and takes over a minute at this
// size; a direct lookup takes milliseconds, so the bound only trips on the
// former.
func TestNameToDeBruijnLinearInDepth(t *testing.T) {
	t.Parallel()
	const depth = 80_000
	refs := make([]Term[Name], depth)
	for i := range refs {
		refs[i] = &Var[Name]{Name: nameTerm(0)}
	}
	var term Term[Name] = &Constr[Name]{Fields: refs}
	for i := depth - 1; i >= 0; i-- {
		term = &Lambda[Name]{ParameterName: nameTerm(Unique(i)), Body: term}
	}
	start := time.Now()
	got, err := NameToDeBruijn(&Program[Name]{Term: term})
	if err != nil {
		t.Fatalf("NameToDeBruijn: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("conversion took %s for %d nested binders", elapsed, depth)
	}
	inner := got.Term
	for range depth {
		inner = inner.(*Lambda[DeBruijn]).Body
	}
	if idx := inner.(*Constr[DeBruijn]).Fields[0].(*Var[DeBruijn]).Name; idx != depth {
		t.Errorf("outermost reference index = %d, want %d", idx, depth)
	}
}

// TestFlatDecodeAcceptsTransactionSizedUnitList decodes the widest unit list a
// 16 KiB script can encode. A unit list item costs one bit, so a width bound
// near the item count of a transaction-sized script rejects scripts the
// reference implementation accepts.
func TestFlatDecodeAcceptsTransactionSizedUnitList(t *testing.T) {
	t.Parallel()
	items := make([]IConstant, 16*1024*8)
	for i := range items {
		items[i] = &Unit{}
	}
	encoded := encodeV110(t, &Constant{Con: &ProtoList{
		LTyp: &TUnit{}, List: items,
	}})
	for path, err := range decodeAll(encoded) {
		if err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
	_, err := DecodeDeBruijnWithContext(encoded, ProgramContext{
		LedgerLanguage: lang.LanguageVersionV3,
		ProtocolMajor:  builtin.VanRossemProtoVersion,
	})
	if err != nil {
		t.Errorf("DecodeDeBruijnWithContext: %v", err)
	}
}

// TestValidateProgramPrePV11ConstrUnbounded checks that the decoders' default
// width is not reported as a protocol rule before protocol version 11.
func TestValidateProgramPrePV11ConstrUnbounded(t *testing.T) {
	t.Parallel()
	program := &Program[DeBruijn]{
		Version: uplcVersion110,
		Term: &Constr[DeBruijn]{
			Fields: errorFields(maxCollectionWidth + 1),
		},
	}
	err := ValidateProgram(program, ProgramContext{
		LedgerLanguage: lang.LanguageVersionV3,
		ProtocolMajor:  builtin.VanRossemProtoVersion - 1,
	})
	if err != nil {
		t.Errorf("ValidateProgram: %v", err)
	}
}

func checkAllocationBound(t *testing.T, run func()) {
	t.Helper()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	run()
	runtime.ReadMemStats(&after)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 1<<20 {
		t.Errorf("allocated %d bytes with a small output/input allowance", allocated)
	}
}

func TestParseOversizedInputBeforeLexer(t *testing.T) {
	input := strings.Repeat(" ", maxInputBytes+1)
	checkAllocationBound(t, func() {
		p := NewParser(input)
		_, err := p.ParseProgram()
		requireErrContains(t, "ParseProgram", err, "input too large")
		_, err = p.ParseTerm()
		requireErrContains(t, "ParseTerm", err, "input too large")
	})
}

func TestParseValueNodeAccounting(t *testing.T) {
	for _, input := range []string{"(con value [(#, [(#, 1)])])", "(con data (V [(#, [(#, 1)])]))"} {
		for _, remaining := range []int{1, 2} {
			p := NewParser(input)
			p.nodes = maxProgramNodes - remaining
			_, err := p.ParseTerm()
			requireErrContains(t, input, err, "too many nodes")
		}
		p := NewParser(input)
		p.nodes = maxProgramNodes - 3
		if _, err := p.ParseTerm(); err != nil {
			t.Errorf("three remaining nodes should admit a term, policy and token: %v", err)
		}
	}
}

func TestParseValueWidthBeforeCompletion(t *testing.T) {
	for _, prefix := range []string{"(con value ", "(con data (V "} {
		for _, input := range []string{
			prefix + "[" + strings.Repeat("(#, []),", maxCollectionWidth) + "(",
			prefix + "[(#, [" + strings.Repeat("(#, 1),", maxCollectionWidth) + "(",
		} {
			_, err := NewParser(input).ParseTerm()
			requireErrContains(t, prefix, err, "too many")
		}
	}
}

func TestPrettyRecursiveDepth(t *testing.T) {
	var typ Typ = &TUnit{}
	var pair IConstant = &Unit{}
	var datum data.PlutusData = data.NewInteger(big.NewInt(1))
	var term Term[DeBruijn] = &Error{}
	for range 20 {
		typ = &TList{Typ: typ}
		pair = &ProtoPair{First: pair, Second: &Unit{}}
		datum = &data.List{Items: []data.PlutusData{datum}}
		term = &Case[DeBruijn]{Constr: term}
	}
	for name, node := range map[string]Term[DeBruijn]{
		"pair values":     &Constant{Con: &ProtoList{LTyp: &TUnit{}, List: []IConstant{pair}}},
		"types":           &Constant{Con: &ProtoList{LTyp: typ}},
		"data":            &Constant{Con: &Data{Inner: datum}},
		"case scrutinees": term,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := PrettyTermWithLimits[DeBruijn](node, PrettyLimits{MaxDepth: 10})
			requireErrContains(t, name, err, "depth")
			if _, err := PrettyTermWithLimits[DeBruijn](node, PrettyLimits{MaxDepth: 100}); err != nil {
				t.Errorf("within the depth limit: %v", err)
			}
		})
	}
}

func TestPrettyLeafAllocationBound(t *testing.T) {
	for name, con := range map[string]IConstant{
		"escaped string": &String{Inner: strings.Repeat("\n", 2<<20)},
		"bytes":          &ByteString{Inner: make([]byte, 1<<20)},
		"integer":        &Integer{Inner: new(big.Int).Lsh(big.NewInt(1), 8<<20)},
	} {
		t.Run(name, func(t *testing.T) {
			checkAllocationBound(t, func() {
				_, err := PrettyTermWithLimits[DeBruijn](&Constant{Con: con}, PrettyLimits{MaxOutputBytes: 32})
				requireErrContains(t, name, err, "output")
			})
		})
	}
}

func TestPrettyStopsBeforeNextConstant(t *testing.T) {
	// The nil integer would panic if a sibling were visited after the first
	// item exhausts the output allowance.
	out, err := PrettyTermWithLimits[DeBruijn](&Constant{Con: &ProtoList{
		LTyp: &TByteString{}, List: []IConstant{&ByteString{Inner: make([]byte, 100)}, (*Integer)(nil)},
	}}, PrettyLimits{MaxOutputBytes: 40})
	requireErrContains(t, "PrettyTermWithLimits", err, "output")
	if out != "" {
		t.Errorf("output on failure: %q", out)
	}
}

func TestPrettyStopsBeforeNextData(t *testing.T) {
	out, err := PrettyTermWithLimits[DeBruijn](&Constant{Con: &Data{Inner: &data.List{
		Items: []data.PlutusData{data.NewByteString(make([]byte, 100)), (*data.ByteString)(nil)},
	}}}, PrettyLimits{MaxOutputBytes: 50})
	requireErrContains(t, "PrettyTermWithLimits", err, "output")
	if out != "" {
		t.Errorf("output on failure: %q", out)
	}
}

func TestPrettyExactOutputAllowance(t *testing.T) {
	for _, term := range []Term[DeBruijn]{
		&Constant{Con: &String{Inner: "\"\\\n\t\a\b\f\r\v\x00\x7fλ"}},
		&Constant{Con: &ByteString{Inner: []byte{0, 15, 16, 255}}},
		&Constant{Con: &Integer{Inner: big.NewInt(-1024)}},
		&Constant{Con: &Data{Inner: &data.Value{Inner: &data.Map{Pairs: [][2]data.PlutusData{
			{data.NewByteString(nil), &data.Map{}},
			{data.NewByteString([]byte{1}), &data.Map{Pairs: [][2]data.PlutusData{
				{data.NewByteString(nil), data.NewInteger(big.NewInt(0))},
			}}},
		}}}}},
	} {
		want := PrettyTerm[DeBruijn](term)
		if want == "" {
			t.Fatal("unbounded control failed")
		}
		got, err := PrettyTermWithLimits[DeBruijn](term, PrettyLimits{MaxOutputBytes: len(want)})
		if err != nil || got != want {
			t.Errorf("exact allowance: got %q, %v; want %q", got, err, want)
		}
		_, err = PrettyTermWithLimits[DeBruijn](term, PrettyLimits{MaxOutputBytes: len(want) - 1})
		requireErrContains(t, "one byte below allowance", err, "output")
	}
}

func TestPrettyValueOutputLimit(t *testing.T) {
	tokens := &data.Map{Pairs: [][2]data.PlutusData{
		{data.NewByteString(nil), data.NewInteger(big.NewInt(1))},
		{data.NewByteString([]byte{1}), data.NewInteger(big.NewInt(2))},
	}}
	term := &Constant{Con: &Data{Inner: &data.Value{Inner: &data.Map{Pairs: [][2]data.PlutusData{
		{data.NewByteString(nil), tokens},
		{data.NewByteString([]byte{1}), tokens},
	}}}}}
	_, err := PrettyTermWithLimits[DeBruijn](term, PrettyLimits{MaxOutputBytes: 32})
	requireErrContains(t, "PrettyTermWithLimits", err, "output")
}
