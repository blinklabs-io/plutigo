package syn

import (
	"encoding/hex"
	"fmt"
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

	// Decode-time well-formedness no longer whitelists specific UPLC term
	// versions (blinklabs-io/plutigo#414): a program declaring a version
	// outside {1.0.0, 1.1.0}, such as 1.2.0, must still decode successfully,
	// since it may be a transaction's own stored-but-unexecuted
	// reference-script output. Only ValidateTermVersionForExecution gates
	// term version, and only immediately before execution; see
	// TestValidateTermVersionForExecutionRejectsRealReferenceScriptVersion
	// and TestDecodeAcceptsRealNeverExecutedReferenceScriptVersion below.
	program.Version = lang.LanguageVersion{1, 2, 0}
	encoded, err = Encode(program)
	if err != nil {
		t.Fatalf("Encode() with non-whitelisted version failed: %v", err)
	}
	decoded, err = DecodeWithContext[DeBruijn](encoded, ProgramContext{
		LedgerLanguage: lang.LanguageVersionV3,
		ProtocolMajor:  9,
	})
	if err != nil {
		t.Fatalf("DecodeWithContext() with non-whitelisted version failed: %v, want success", err)
	}
	if decoded.Version != program.Version {
		t.Fatalf("decoded version = %v, want %v", decoded.Version, program.Version)
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

// realPreviewReferenceScriptFlatHex is the flat-encoded UPLC body of the
// PlutusV2Script reference script at output 0 of Preview-testnet
// transaction f5a0e06f3147c499c324041d16f510db07dd875aa0a80b7b0b2f2e990f9d7e45
// (block 2423582, abs slot 58083610; 2.2M+ confirmations and
// valid_contract=true per Koios). This is the transaction's script bytes
// after stripping the tag-24/double-bytestring CBOR wrapping and the
// innermost CBOR bytestring header (0x58 0x79, a 121-byte string); see
// blinklabs-io/plutigo#414. The trailing 0x01 completes the flat format's
// mandatory byte-aligned filler terminator to the CBOR-declared 121-byte
// length, which the issue's hex quote (120 bytes) omitted.
const realPreviewReferenceScriptFlatHex = "593191013330004c011e581c9ba20e5c61a696763941498712fbf454804953dfec55aa34891aabea004c012258205ed4e781bef7635ac63e9672a779f80245f9c98d7f68fcdebcfec207442cb140004c01259f581c8d2a90fab86ad0869ce94cbaad93a1217bc2f2bafb84e3bc27b5b92d4466494147ff0001"

// realPreviewReferenceScriptVersion is this script's declared UPLC term
// version, decoded from its flat header. It is not a Plutus ledger-language
// version, and is likely a deliberate test/probe value rather than a
// version any real compiler would emit -- but that is exactly the
// well-formed, never-executed content that decode-time validation must not
// reject.
var realPreviewReferenceScriptVersion = lang.LanguageVersion{89, 49, 145}

// TestDecodeAcceptsRealNeverExecutedReferenceScriptVersion is the regression
// case for blinklabs-io/plutigo#414: a real, canonical Preview-testnet
// transaction carries this reference script, never invoked by any
// transaction, whose declared UPLC term version is outside {1.0.0, 1.1.0}.
// Decode-time well-formedness must accept it regardless of protocol
// version, since upstream plutus-ledger-api only gates program version at
// execution time (PlutusLedgerApi.Common.Eval.mkTermToEvaluate), which a
// stored-but-unexecuted reference script never reaches.
func TestDecodeAcceptsRealNeverExecutedReferenceScriptVersion(t *testing.T) {
	t.Parallel()

	flatProgram, err := hex.DecodeString(realPreviewReferenceScriptFlatHex)
	if err != nil {
		t.Fatalf("failed to decode test fixture hex: %v", err)
	}

	for _, protocolMajor := range []uint{9, 10, 11} {
		t.Run(fmt.Sprintf("protocol %d", protocolMajor), func(t *testing.T) {
			t.Parallel()

			decoded, err := DecodeDeBruijnWithContext(flatProgram, ProgramContext{
				LedgerLanguage: lang.LanguageVersionV2,
				ProtocolMajor:  protocolMajor,
			})
			if err != nil {
				t.Fatalf(
					"DecodeDeBruijnWithContext() failed: %v, want success (a real reference script must decode regardless of its UPLC term version)",
					err,
				)
			}
			if decoded.Version != realPreviewReferenceScriptVersion {
				t.Fatalf("decoded version = %v, want %v", decoded.Version, realPreviewReferenceScriptVersion)
			}
		})
	}
}

// TestValidateTermVersionForExecutionRejectsRealReferenceScriptVersion
// proves the version gate blinklabs-io/plutigo#414 moves out of decode time
// still applies before a script is actually run: no longer being rejected
// at decode time must not make this real reference script's UPLC term
// version universally executable.
func TestValidateTermVersionForExecutionRejectsRealReferenceScriptVersion(t *testing.T) {
	t.Parallel()

	err := ValidateTermVersionForExecution(realPreviewReferenceScriptVersion, ProgramContext{
		LedgerLanguage: lang.LanguageVersionV2,
		ProtocolMajor:  9,
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported UPLC program version") {
		t.Fatalf("ValidateTermVersionForExecution() error = %v, want unsupported-version error", err)
	}
}

// TestSupportsConstructorsIsLexicographicallyAtLeast110 proves constr/case
// support is gated by UPLC term version >= 1.1.0 using lexicographic
// ordering, not by equality to 1.1.0 (blinklabs-io/plutigo#414): a version
// above 1.1.0, such as the real #414 reference script's 89.49.145, only
// became reachable here once the decode-time version whitelist was
// removed, and an equality-only check would have wrongly rejected it.
func TestSupportsConstructorsIsLexicographicallyAtLeast110(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version lang.LanguageVersion
		wantErr bool
	}{
		{"below 1.1.0 does not support constructors", lang.LanguageVersion{1, 0, 0}, true},
		// A larger patch component must not outrank a smaller minor one.
		{"below 1.1.0 with a higher patch does not support constructors", lang.LanguageVersion{1, 0, 9}, true},
		{"exactly 1.1.0 supports constructors", lang.LanguageVersion{1, 1, 0}, false},
		{"above 1.1.0 by patch supports constructors", lang.LanguageVersion{1, 1, 1}, false},
		{"above 1.1.0 (the real #414 version) supports constructors", realPreviewReferenceScriptVersion, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateProgram(&Program[DeBruijn]{
				Version: tt.version,
				Term:    &Constr[DeBruijn]{Tag: 0},
			}, ProgramContext{
				LedgerLanguage: lang.LanguageVersionV3,
				ProtocolMajor:  9,
			})
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "constr is not available") {
					t.Fatalf("ValidateProgram() error = %v, want constr-not-available error", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateProgram() failed: %v", err)
			}
		})
	}
}

// TestDecodeDeBruijnForExecutionAppliesExecutionGates proves the
// execution-decoding entry point applies the phase-2 gates that
// [DecodeDeBruijnWithContext] deliberately omits, so a consumer that reaches
// for it cannot decode a program and then hand the CEK machine a term whose
// UPLC version is illegal to execute. The same bytes must still decode
// through DecodeDeBruijnWithContext, which is the phase-1 well-formedness
// path a transaction's own stored-but-unexecuted reference-script output
// takes.
func TestDecodeDeBruijnForExecutionAppliesExecutionGates(t *testing.T) {
	t.Parallel()

	referenceScript, err := hex.DecodeString(realPreviewReferenceScriptFlatHex)
	if err != nil {
		t.Fatalf("failed to decode test fixture hex: %v", err)
	}
	encode := func(t *testing.T, version lang.LanguageVersion) []byte {
		t.Helper()
		encoded, err := Encode(&Program[DeBruijn]{Version: version, Term: &Error{}})
		if err != nil {
			t.Fatalf("Encode() failed: %v", err)
		}
		return encoded
	}

	tests := []struct {
		name     string
		program  []byte
		context  ProgramContext
		wantErr  string
		wantVers lang.LanguageVersion
	}{
		{
			name:     "executable term version passes",
			program:  encode(t, uplcVersion100),
			context:  ProgramContext{LedgerLanguage: lang.LanguageVersionV2, ProtocolMajor: 9},
			wantVers: uplcVersion100,
		},
		{
			name:    "term version outside the executable set is rejected",
			program: referenceScript,
			context: ProgramContext{LedgerLanguage: lang.LanguageVersionV2, ProtocolMajor: 9},
			wantErr: "unsupported UPLC program version",
		},
		{
			name:    "van Rossem gate still applies",
			program: encode(t, uplcVersion110),
			context: ProgramContext{LedgerLanguage: lang.LanguageVersionV2, ProtocolMajor: 10},
			wantErr: "UPLC version 1.1.0 is not available",
		},
		{
			name:     "van Rossem gate opens at its protocol version",
			program:  encode(t, uplcVersion110),
			context:  ProgramContext{LedgerLanguage: lang.LanguageVersionV2, ProtocolMajor: 11},
			wantVers: uplcVersion110,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Whatever the execution path decides, the well-formedness path
			// must accept these bytes: the gates below are phase-2 only.
			if _, err := DecodeDeBruijnWithContext(tt.program, tt.context); err != nil {
				t.Fatalf("DecodeDeBruijnWithContext() failed: %v, want success", err)
			}

			program, err := DecodeDeBruijnForExecution(tt.program, tt.context)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("DecodeDeBruijnForExecution() error = %v, want error containing %q", err, tt.wantErr)
				}
				if program != nil {
					t.Fatalf("DecodeDeBruijnForExecution() program = %v, want nil on error", program)
				}
				return
			}
			if err != nil {
				t.Fatalf("DecodeDeBruijnForExecution() failed: %v", err)
			}
			if program.Version != tt.wantVers {
				t.Fatalf("decoded version = %v, want %v", program.Version, tt.wantVers)
			}
		})
	}
}
