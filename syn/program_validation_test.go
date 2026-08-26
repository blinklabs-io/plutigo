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
			name:       "PLC 1.1 is not available to V2 before van Rossem",
			language:   lang.LanguageVersionV2,
			protocol:   10,
			programVer: uplcVersion110,
			term:       &Error{},
			wantErr:    "UPLC version 1.1.0 is not available",
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
		{"V1 before van Rossem with PLC 1.1", lang.LanguageVersionV1, 10, uplcVersion110, true},
		{"V1 at van Rossem with PLC 1.1", lang.LanguageVersionV1, 11, uplcVersion110, false},
		{"V2 at Vasil with PLC 1.0", lang.LanguageVersionV2, 7, uplcVersion100, false},
		{"V2 before van Rossem with PLC 1.1", lang.LanguageVersionV2, 10, uplcVersion110, true},
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
