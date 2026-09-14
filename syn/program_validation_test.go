package syn

import (
	"strings"
	"testing"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/lang"
)

func TestDecodeWithContextPreservesAndValidatesProgramVersion(t *testing.T) {
	program := &Program[DeBruijn]{
		Version: uplcVersion110,
		Term:    &Error{},
	}
	encoded, err := Encode(program)
	if err != nil {
		t.Fatalf("Encode() failed: %v", err)
	}

	decoded, err := DecodeWithContext[DeBruijn](encoded, ProgramContext{
		LedgerLanguage: lang.LanguageVersionV3,
		ProtocolMajor:  9,
	})
	if err != nil {
		t.Fatalf("DecodeWithContext() failed: %v", err)
	}
	if decoded.Version != program.Version {
		t.Fatalf("decoded version = %v, want %v", decoded.Version, program.Version)
	}

	program.Version = lang.LanguageVersion{1, 2, 0}
	encoded, err = Encode(program)
	if err != nil {
		t.Fatalf("Encode() with unsupported version failed: %v", err)
	}
	_, err = DecodeWithContext[DeBruijn](encoded, ProgramContext{
		LedgerLanguage: lang.LanguageVersionV3,
		ProtocolMajor:  9,
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported UPLC program version") {
		t.Fatalf("DecodeWithContext() error = %v, want unsupported-version error", err)
	}
}

func TestValidateProgramUsesLedgerLanguageAndProtocol(t *testing.T) {
	tests := []struct {
		name       string
		language   lang.LanguageVersion
		protocol   uint
		programVer lang.LanguageVersion
		term       Term[DeBruijn]
		wantErr    string
	}{
		{
			name:       "V2 batch two builtin at Vasil protocol",
			language:   lang.LanguageVersionV2,
			protocol:   10,
			programVer: uplcVersion100,
			term:       &Builtin{DefaultFunction: builtin.SerialiseData},
		},
		{
			name:       "dead branch builtin is still rejected",
			language:   lang.LanguageVersionV1,
			protocol:   10,
			programVer: uplcVersion100,
			term: &Delay[DeBruijn]{Term: &Builtin{
				DefaultFunction: builtin.SerialiseData,
			}},
			wantErr: "builtin serialiseData is not available",
		},
		{
			// The van Rossem term-version gate is phase-2 (execution-time)
			// only; see TestValidateTermVersionForExecutionMatrix. A
			// decode-time ValidateProgram/DecodeWithContext call must accept
			// this combination, since the program may be a stored
			// reference-script output that is never executed by this
			// transaction.
			name:       "PLC 1.1 decodes for V2 before van Rossem",
			language:   lang.LanguageVersionV2,
			protocol:   10,
			programVer: uplcVersion110,
			term:       &Error{},
		},
		{
			name:       "constructors require PLC 1.1",
			language:   lang.LanguageVersionV3,
			protocol:   9,
			programVer: uplcVersion100,
			term:       &Constr[DeBruijn]{Tag: 0},
			wantErr:    "constr is not available",
		},
		{
			name:       "PLC 1.1 constructors are accepted",
			language:   lang.LanguageVersionV3,
			protocol:   9,
			programVer: uplcVersion110,
			term:       &Constr[DeBruijn]{Tag: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProgram(&Program[DeBruijn]{
				Version: tt.programVer,
				Term:    tt.term,
			}, ProgramContext{
				LedgerLanguage: tt.language,
				ProtocolMajor:  tt.protocol,
			})
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateProgram() failed: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateProgram() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateProgramVersionMatrix(t *testing.T) {
	tests := []struct {
		name     string
		language lang.LanguageVersion
		protocol uint
		version  lang.LanguageVersion
		wantErr  bool
	}{
		{"V1 at Alonzo with PLC 1.0", lang.LanguageVersionV1, 5, uplcVersion100, false},
		{"V1 at zero protocol", lang.LanguageVersionV1, 0, uplcVersion100, true},
		// Decoding (phase 1) no longer rejects PLC 1.1 pre-van-Rossem for
		// V1/V2: that gate is execution-only. See
		// TestValidateTermVersionForExecutionMatrix.
		{"V1 before van Rossem with PLC 1.1 decodes", lang.LanguageVersionV1, 10, uplcVersion110, false},
		{"V1 at van Rossem with PLC 1.1", lang.LanguageVersionV1, 11, uplcVersion110, false},
		{"V2 at Vasil with PLC 1.0", lang.LanguageVersionV2, 7, uplcVersion100, false},
		{"V2 before van Rossem with PLC 1.1 decodes", lang.LanguageVersionV2, 10, uplcVersion110, false},
		{"V2 at van Rossem with PLC 1.1", lang.LanguageVersionV2, 11, uplcVersion110, false},
		{"V3 at Chang with PLC 1.0", lang.LanguageVersionV3, 9, uplcVersion100, false},
		{"V3 at Chang with PLC 1.1", lang.LanguageVersionV3, 9, uplcVersion110, false},
		{"V4 before Dijkstra", lang.LanguageVersionV4, 11, uplcVersion100, true},
		{"V4 at Dijkstra with PLC 1.1", lang.LanguageVersionV4, 12, uplcVersion110, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProgram(&Program[DeBruijn]{
				Version: tt.version,
				Term:    &Error{},
			}, ProgramContext{
				LedgerLanguage: tt.language,
				ProtocolMajor:  tt.protocol,
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateProgram() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateTermVersionForExecutionMatrix proves the van Rossem
// term-version-vs-ledger-language gate moved to ValidateTermVersionForExecution
// rather than disappearing: it must still reject PLC 1.1 for V1/V2 below
// protocol 11, accept it at/after 11, and never fire for V3/V4 regardless of
// protocol version.
func TestValidateTermVersionForExecutionMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		language lang.LanguageVersion
		protocol uint
		version  lang.LanguageVersion
		wantErr  bool
	}{
		{"V1 before van Rossem with PLC 1.1 is rejected", lang.LanguageVersionV1, 10, uplcVersion110, true},
		{"V1 at van Rossem with PLC 1.1 is accepted", lang.LanguageVersionV1, 11, uplcVersion110, false},
		{"V1 with PLC 1.0 is unaffected", lang.LanguageVersionV1, 10, uplcVersion100, false},
		{"V2 before van Rossem with PLC 1.1 is rejected", lang.LanguageVersionV2, 10, uplcVersion110, true},
		{"V2 at van Rossem with PLC 1.1 is accepted", lang.LanguageVersionV2, 11, uplcVersion110, false},
		{"V3 with PLC 1.1 before protocol 11 is unaffected", lang.LanguageVersionV3, 9, uplcVersion110, false},
		{"V3 with PLC 1.1 at protocol 11 is unaffected", lang.LanguageVersionV3, 11, uplcVersion110, false},
		{"V4 with PLC 1.1 is unaffected", lang.LanguageVersionV4, 12, uplcVersion110, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateTermVersionForExecution(tt.version, ProgramContext{
				LedgerLanguage: tt.language,
				ProtocolMajor:  tt.protocol,
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateTermVersionForExecution() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && (err == nil || !strings.Contains(err.Error(), "UPLC version 1.1.0 is not available")) {
				t.Fatalf("ValidateTermVersionForExecution() error = %v, want UPLC-version-not-available error", err)
			}
		})
	}
}

// TestDecodeWithContextAcceptsPreVanRossemV2TermVersion110 is the regression
// case from gouroboros#2316: a PlutusV2 script using UPLC term-version 1.1.0
// must decode successfully below protocol major 11, since decode/well-formed
// checking (phase 1) is applied uniformly to witness scripts and to a
// transaction's own stored-but-unexecuted reference-script outputs, and real
// cardano-ledger's phase-1 check never gates the term version. The
// once-per-invocation gate is [ValidateTermVersionForExecution], exercised
// separately below.
func TestDecodeWithContextAcceptsPreVanRossemV2TermVersion110(t *testing.T) {
	t.Parallel()

	program := &Program[DeBruijn]{
		Version: uplcVersion110,
		Term:    &Error{},
	}
	encoded, err := Encode(program)
	if err != nil {
		t.Fatalf("Encode() failed: %v", err)
	}

	context := ProgramContext{
		LedgerLanguage: lang.LanguageVersionV2,
		ProtocolMajor:  builtin.VanRossemProtoVersion - 1,
	}

	decoded, err := DecodeWithContext[DeBruijn](encoded, context)
	if err != nil {
		t.Fatalf("DecodeWithContext() failed: %v, want success (phase-1 decode must not apply the execution-only van Rossem gate)", err)
	}
	if decoded.Version != uplcVersion110 {
		t.Fatalf("decoded version = %v, want %v", decoded.Version, uplcVersion110)
	}

	if err := ValidateTermVersionForExecution(decoded.Version, context); err == nil {
		t.Fatalf("ValidateTermVersionForExecution() succeeded, want rejection below protocol %d", builtin.VanRossemProtoVersion)
	} else if !strings.Contains(err.Error(), "UPLC version 1.1.0 is not available") {
		t.Fatalf("ValidateTermVersionForExecution() error = %v, want UPLC-version-not-available error", err)
	}

	atVanRossem := ProgramContext{
		LedgerLanguage: lang.LanguageVersionV2,
		ProtocolMajor:  builtin.VanRossemProtoVersion,
	}
	if _, err := DecodeWithContext[DeBruijn](encoded, atVanRossem); err != nil {
		t.Fatalf("DecodeWithContext() at van Rossem failed: %v", err)
	}
	if err := ValidateTermVersionForExecution(uplcVersion110, atVanRossem); err != nil {
		t.Fatalf("ValidateTermVersionForExecution() at van Rossem failed: %v", err)
	}
}

func TestDecodeWithContextRejectsDeadBranchBuiltin(t *testing.T) {
	program := &Program[DeBruijn]{
		Version: uplcVersion100,
		Term: &Apply[DeBruijn]{
			Function: &Builtin{DefaultFunction: builtin.IfThenElse},
			Argument: &Delay[DeBruijn]{Term: &Builtin{
				DefaultFunction: builtin.SerialiseData,
			}},
		},
	}
	encoded, err := Encode(program)
	if err != nil {
		t.Fatalf("Encode() failed: %v", err)
	}
	_, err = DecodeWithContext[DeBruijn](encoded, ProgramContext{
		LedgerLanguage: lang.LanguageVersionV1,
		ProtocolMajor:  10,
	})
	if err == nil || !strings.Contains(err.Error(), "builtin serialiseData is not available") {
		t.Fatalf("DecodeWithContext() error = %v, want dead-branch builtin error", err)
	}
}

func TestDecodeWithContextRejectsSyntaxBeforeProgramVersion(t *testing.T) {
	program := &Program[DeBruijn]{
		Version: uplcVersion100,
		Term:    &Constr[DeBruijn]{Tag: 0},
	}
	encoded, err := Encode(program)
	if err != nil {
		t.Fatalf("Encode() failed: %v", err)
	}
	_, err = DecodeWithContext[DeBruijn](encoded, ProgramContext{
		LedgerLanguage: lang.LanguageVersionV3,
		ProtocolMajor:  9,
	})
	if err == nil || !strings.Contains(err.Error(), "constr is not available") {
		t.Fatalf("DecodeWithContext() error = %v, want unavailable-syntax error", err)
	}
}

func TestParseWithContextRejectsUnavailableSyntaxAndBuiltin(t *testing.T) {
	_, err := ParseWithContext(
		"(program 1.0.0 (constr 0))",
		ProgramContext{
			LedgerLanguage: lang.LanguageVersionV3,
			ProtocolMajor:  9,
		},
	)
	if err == nil || !strings.Contains(err.Error(), "constr can't be used before 1.1.0") {
		t.Fatalf("ParseWithContext() syntax error = %v", err)
	}

	_, err = ParseWithContext(
		"(program 1.0.0 (delay (builtin serialiseData)))",
		ProgramContext{
			LedgerLanguage: lang.LanguageVersionV1,
			ProtocolMajor:  10,
		},
	)
	if err == nil || !strings.Contains(err.Error(), "builtin serialiseData is not available") {
		t.Fatalf("ParseWithContext() builtin error = %v", err)
	}
}

func TestContextualValidationConstrFieldLimit(t *testing.T) {
	tests := []struct {
		name       string
		protocol   uint
		fieldCount int
		wantErr    string
	}{
		{
			name:       "pre-PV11 remains unbounded",
			protocol:   10,
			fieldCount: 1025,
		},
		{
			name:       "PV11 below limit",
			protocol:   11,
			fieldCount: 1023,
		},
		{
			name:       "PV11 at limit",
			protocol:   11,
			fieldCount: 1024,
		},
		{
			name:       "PV11 above limit",
			protocol:   11,
			fieldCount: 1025,
			wantErr:    "constr with 1025 fields is not available in protocol version 11",
		},
		{
			name:       "post-PV11 above limit",
			protocol:   12,
			fieldCount: 1025,
			wantErr:    "constr with 1025 fields is not available in protocol version 12",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields := make([]Term[DeBruijn], tt.fieldCount)
			for i := range fields {
				fields[i] = &Error{}
			}
			program := &Program[DeBruijn]{
				Version: uplcVersion110,
				Term: &Constr[DeBruijn]{
					Tag:    0,
					Fields: fields,
				},
			}
			context := ProgramContext{
				LedgerLanguage: lang.LanguageVersionV3,
				ProtocolMajor:  tt.protocol,
			}
			encoded, err := Encode(program)
			if err != nil {
				t.Fatalf("Encode() failed: %v", err)
			}

			entryPoints := []struct {
				name string
				run  func() (int, error)
			}{
				{
					name: "ValidateProgram",
					run: func() (int, error) {
						return len(fields), ValidateProgram(program, context)
					},
				},
				{
					name: "DecodeWithContext generic",
					run: func() (int, error) {
						decoded, err := DecodeWithContext[Name](encoded, context)
						if err != nil {
							return 0, err
						}
						constr, ok := decoded.Term.(*Constr[Name])
						if !ok {
							return -1, nil
						}
						return len(constr.Fields), nil
					},
				},
				{
					name: "DecodeDeBruijnWithContext",
					run: func() (int, error) {
						decoded, err := DecodeDeBruijnWithContext(encoded, context)
						if err != nil {
							return 0, err
						}
						constr, ok := decoded.Term.(*Constr[DeBruijn])
						if !ok {
							return -1, nil
						}
						return len(constr.Fields), nil
					},
				},
			}

			for _, entryPoint := range entryPoints {
				t.Run(entryPoint.name, func(t *testing.T) {
					gotFields, err := entryPoint.run()
					if tt.wantErr != "" {
						if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
							t.Fatalf("error = %v, want %q", err, tt.wantErr)
						}
						return
					}
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					if gotFields != tt.fieldCount {
						t.Fatalf("decoded field count = %d, want %d", gotFields, tt.fieldCount)
					}
				})
			}
		})
	}
}
