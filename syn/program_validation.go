package syn

import (
	"errors"
	"fmt"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/lang"
)

// ProgramContext identifies the ledger language and protocol version against
// which a UPLC program is to be decoded and validated.
//
// The program header is a Plutus Core language version. It is not a ledger
// language identifier: the ledger language is selected by the caller.
type ProgramContext struct {
	LedgerLanguage lang.LanguageVersion
	ProtocolMajor  uint
}

var (
	uplcVersion100 = lang.LanguageVersion{1, 0, 0}
	uplcVersion110 = lang.LanguageVersion{1, 1, 0}
)

const maxConstrFieldsPV11 = 1024

// ValidateProgram is a phase-1, decode-time well-formedness check: language
// introduction, builtin availability, and structural bounds such as
// constructor arity. It applies uniformly to witness scripts and to a
// transaction's own newly-created reference-script outputs, since real
// cardano-ledger applies the same decode-time gate to both. It does not check
// whether the program's UPLC term version is legal to *execute* for its
// ledger language at the current protocol version -- that is a phase-2,
// execution-time check; see [ValidateTermVersionForExecution]. Validation
// walks every term, including branches which evaluation might not reach.
func ValidateProgram[T any](program *Program[T], context ProgramContext) error {
	if program == nil {
		return errors.New("program is required")
	}
	if err := validateProgramVersion(program.Version, context); err != nil {
		return err
	}

	plutusVersion, err := plutusVersionForLedgerLanguage(context.LedgerLanguage)
	if err != nil {
		return err
	}

	terms := []Term[T]{program.Term}
	for len(terms) > 0 {
		last := len(terms) - 1
		term := terms[last]
		terms = terms[:last]
		if isNilTerm[T](term) {
			return errors.New("program contains a nil term")
		}

		switch t := term.(type) {
		case *Var[T], *Constant, *Error:
			// No version-dependent validation is needed for these terms.
		case *Delay[T]:
			terms = append(terms, t.Term)
		case *Force[T]:
			terms = append(terms, t.Term)
		case *Lambda[T]:
			terms = append(terms, t.Body)
		case *Apply[T]:
			terms = append(terms, t.Function, t.Argument)
		case *Builtin:
			if t.DefaultFunction > builtin.MaxDefaultFunction {
				return fmt.Errorf("builtin tag %d is invalid", t.DefaultFunction)
			}
			if !t.IsAvailableInWithProto(
				plutusVersion,
				context.ProtocolMajor,
			) {
				return fmt.Errorf(
					"builtin %s is not available in Plutus V%d at protocol version %d",
					t.String(),
					plutusVersion,
					context.ProtocolMajor,
				)
			}
		case *Constr[T]:
			if !supportsConstructors(program.Version) {
				return fmt.Errorf("constr is not available in UPLC version %s", formatVersion(program.Version))
			}
			if context.ProtocolMajor >= builtin.VanRossemProtoVersion &&
				len(t.Fields) > maxConstrFieldsPV11 {
				return fmt.Errorf(
					"constr with %d fields is not available in protocol version %d",
					len(t.Fields),
					context.ProtocolMajor,
				)
			}
			terms = append(terms, t.Fields...)
		case *Case[T]:
			if !supportsConstructors(program.Version) {
				return fmt.Errorf("case is not available in UPLC version %s", formatVersion(program.Version))
			}
			terms = append(terms, t.Constr)
			terms = append(terms, t.Branches...)
		default:
			return fmt.Errorf("unsupported term type %T", term)
		}
	}
	return nil
}

// DecodeWithContext decodes and validates a FLAT-encoded UPLC program before
// returning it. The encoded program version is checked immediately after its
// header is read; terms are then checked exhaustively before the program is
// returned.
func DecodeWithContext[T Binder](bytes []byte, context ProgramContext) (*Program[T], error) {
	program, err := decodeWithContext[T](bytes, context)
	if err != nil {
		return nil, err
	}
	if err := ValidateProgram(program, context); err != nil {
		return nil, err
	}
	return program, nil
}

// ParseWithContext parses and validates a textual UPLC program against the
// selected ledger language and protocol version.
func ParseWithContext(input string, context ProgramContext) (*Program[Name], error) {
	program, err := Parse(input)
	if err != nil {
		return nil, err
	}
	if err := ValidateProgram(program, context); err != nil {
		return nil, err
	}
	return program, nil
}

// DecodeDeBruijnWithContext is the De Bruijn-specialized form of
// [DecodeWithContext].
func DecodeDeBruijnWithContext(
	bytes []byte,
	context ProgramContext,
) (*Program[DeBruijn], error) {
	return DecodeWithContext[DeBruijn](bytes, context)
}

func plutusVersionForLedgerLanguage(version lang.LanguageVersion) (builtin.PlutusVersion, error) {
	switch version {
	case lang.LanguageVersionV1:
		return builtin.PlutusV1, nil
	case lang.LanguageVersionV2:
		return builtin.PlutusV2, nil
	case lang.LanguageVersionV3:
		return builtin.PlutusV3, nil
	case lang.LanguageVersionV4:
		return builtin.PlutusV4, nil
	default:
		return 0, fmt.Errorf("unsupported Plutus ledger language version %s", formatVersion(version))
	}
}

// validateProgramVersion is a phase-1, decode-time well-formedness check: it
// applies uniformly to witness scripts and to a transaction's own
// newly-created reference-script outputs (real cardano-ledger's
// deserialiseScript/scriptCBORDecoder gate, applied by
// validateScriptsWellFormedTxOuts to both). It deliberately excludes any
// check of the UPLC term version itself: upstream plutus-ledger-api never
// gates the program version at decode/well-formedness time, only at
// execution time (real cardano-ledger's mkTermToEvaluate, reached only via
// evaluateScriptRestricting/evaluateScriptCounting), which a transaction's own
// stored-but-unexecuted reference-script output can never be. See
// [ValidateTermVersionForExecution] for that check.
func validateProgramVersion(_ lang.LanguageVersion, context ProgramContext) error {
	plutusVersion, err := plutusVersionForLedgerLanguage(context.LedgerLanguage)
	if err != nil {
		return err
	}
	if context.ProtocolMajor == 0 {
		return errors.New("protocol major version must be positive")
	}

	introduced := map[builtin.PlutusVersion]uint{
		builtin.PlutusV1: 5,
		builtin.PlutusV2: 7,
		builtin.PlutusV3: 9,
		builtin.PlutusV4: 12,
	}
	if context.ProtocolMajor < introduced[plutusVersion] {
		return fmt.Errorf(
			"plutus ledger language V%d is not available at protocol version %d",
			plutusVersion,
			context.ProtocolMajor,
		)
	}
	return nil
}

// ValidateTermVersionForExecution checks whether a program's UPLC term
// version is legal to *execute* under the given ledger language and protocol
// version. Unlike [ValidateProgram], a decode-time structural check applied
// uniformly to witness scripts and to stored-but-unexecuted reference-script
// outputs, this check applies only immediately before a script is actually
// run through the CEK machine: a transaction cannot execute a reference
// script it is simultaneously creating, so this must never gate decoding or
// storage, only evaluation.
//
// It combines two gates that upstream plutus-ledger-api applies only at
// execution time (mkTermToEvaluate's plcVersionsAvailableIn): the UPLC
// program version must be one of {1.0.0, 1.1.0} -- decode-time
// well-formedness deliberately does not check this, see
// [validateProgramVersion] -- and, the "van Rossem" gate, UPLC 1.1.0
// sums-of-products are only legal for Plutus V1/V2 at protocol major version
// 11 and above.
func ValidateTermVersionForExecution(version lang.LanguageVersion, context ProgramContext) error {
	if version != uplcVersion100 && version != uplcVersion110 {
		return fmt.Errorf(
			"unsupported UPLC program version %s; supported versions are 1.0.0 and 1.1.0",
			formatVersion(version),
		)
	}

	plutusVersion, err := plutusVersionForLedgerLanguage(context.LedgerLanguage)
	if err != nil {
		return err
	}
	if version == uplcVersion110 &&
		(plutusVersion == builtin.PlutusV1 || plutusVersion == builtin.PlutusV2) &&
		context.ProtocolMajor < builtin.VanRossemProtoVersion {
		return fmt.Errorf(
			"UPLC version 1.1.0 is not available for Plutus V%d at protocol version %d",
			plutusVersion,
			context.ProtocolMajor,
		)
	}
	return nil
}

// supportsConstructors reports whether a UPLC program version is at least
// 1.1.0, the version constr/case syntax was introduced in. This must be a
// lexicographic (major, then minor, then patch) comparison, not equality:
// once validateProgramVersion no longer whitelists specific term versions,
// versions above 1.1.0 are reachable here too. lang.LanguageVersion is a
// plain [3]uint32 array, and Go does not support ordering comparisons on
// array types directly.
func supportsConstructors(version lang.LanguageVersion) bool {
	return versionAtLeast(version, uplcVersion110)
}

// versionAtLeast reports whether version is lexicographically greater than
// or equal to other, comparing major, then minor, then patch in order.
func versionAtLeast(version, other lang.LanguageVersion) bool {
	for i := range version {
		if version[i] != other[i] {
			return version[i] > other[i]
		}
	}
	return true
}

func formatVersion(version lang.LanguageVersion) string {
	return fmt.Sprintf("%d.%d.%d", version[0], version[1], version[2])
}

func isNilTerm[T any](term Term[T]) bool {
	switch t := term.(type) {
	case *Var[T]:
		return t == nil
	case *Delay[T]:
		return t == nil
	case *Force[T]:
		return t == nil
	case *Lambda[T]:
		return t == nil
	case *Apply[T]:
		return t == nil
	case *Builtin:
		return t == nil
	case *Constr[T]:
		return t == nil
	case *Case[T]:
		return t == nil
	case *Error:
		return t == nil
	case *Constant:
		return t == nil
	default:
		return term == nil
	}
}
