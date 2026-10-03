package syn

import (
	"math/big"
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
// walking the enclosing scopes takes well over a minute at this size; a direct
// lookup takes milliseconds, so the generous bound only trips on the former.
func TestNameToDeBruijnLinearInDepth(t *testing.T) {
	t.Parallel()
	const depth = 40_000
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
