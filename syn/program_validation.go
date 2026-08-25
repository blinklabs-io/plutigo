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

// ValidateProgram checks a decoded program against the selected ledger
// language and protocol version. Validation walks every term, including
// branches which evaluation might not reach.
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

func validateProgramVersion(version lang.LanguageVersion, context ProgramContext) error {
	if version != lang.LanguageVersionV1 && version != lang.LanguageVersionV2 {
		return fmt.Errorf(
			"unsupported UPLC program version %s; supported versions are 1.0.0 and 1.1.0",
			formatVersion(version),
		)
	}

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
	if version == lang.LanguageVersionV2 &&
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

func supportsConstructors(version lang.LanguageVersion) bool {
	return version == lang.LanguageVersionV2
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
