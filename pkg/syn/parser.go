package syn

import (
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/blinklabs-io/plutigo/pkg/builtin"
)

func Parse(input string) (*Program[Name], error) {
	input = normalizeInput(input)

	// debugLog(1, "Normalized input:\n%s", input)

	if !strings.HasPrefix(input, "(program") {
		return nil, fmt.Errorf("program must start with (program")
	}

	// Extract version and body
	versionEnd := strings.Index(input, ")")
	if versionEnd == -1 {
		return nil, fmt.Errorf("missing closing parenthesis for program")
	}

	body := strings.TrimSpace(input[versionEnd+1:])
	if len(body) == 0 {
		return nil, fmt.Errorf("empty program body")
	}

	term, err := ParseTerm(body)

	if err != nil {
		return nil, fmt.Errorf("failed to parse program body: %w", err)
	}

	return &Program[Name]{Term: term}, nil
}

func ParseTerm(input string) (Term[Name], error) {
	input = strings.TrimSpace(input)

	// debugLog(3, "ParseTerm input: %q", input)

	// Handle parenthesized terms
	if strings.HasPrefix(input, "(") && strings.HasSuffix(input, ")") {
		inner := strings.TrimSpace(input[1 : len(input)-1])
		return parseParenthesizedTerm(inner)
	}

	// Handle applications
	if strings.HasPrefix(input, "[") {
		return ParseApplication(input)
	}

	// Handle simple terms (variables, etc.)
	return parseSimpleTerm(input)
}

func parseParenthesizedTerm(input string) (Term[Name], error) {
	parts := strings.SplitN(input, " ", 2)

	if len(parts) == 0 {
		return nil, fmt.Errorf("empty parenthesized term")
	}

	keyword := parts[0]

	rest := ""

	if len(parts) > 1 {
		rest = strings.TrimSpace(parts[1])
	}

	switch keyword {
	case "builtin":
		return parseBuiltin(rest)
	case "force":
		return parseForce(rest)
	case "delay":
		return parseDelay(rest)
	case "lam":
		return parseLambda(rest)
	case "con":
		return parseConstant(rest)
	default:
		return nil, fmt.Errorf("unknown keyword in parenthesized term: %s", keyword)
	}
}

func normalizeInput(input string) string {
	// Remove comments
	input = regexp.MustCompile(`--.*`).ReplaceAllString(input, "")

	// Normalize whitespace
	input = regexp.MustCompile(`\s+`).ReplaceAllString(input, " ")

	// Trim leading/trailing whitespace
	return strings.TrimSpace(input)
}

func parseBuiltin(input string) (Builtin, error) {
	name := strings.TrimSpace(input)
	if defaultFunction, ok := builtin.Builtins[name]; ok {
		return Builtin{defaultFunction}, nil
	}

	return Builtin{}, fmt.Errorf("unknown builtin: %s", name)
}

func parseForce(input string) (Term[Name], error) {
	term, err := ParseTerm(input)
	if err != nil {
		return nil, fmt.Errorf("force parse failed: %w", err)
	}

	return Force[Name]{Term: term}, nil
}

func parseDelay(input string) (Term[Name], error) {
	term, err := ParseTerm(input)
	if err != nil {
		return nil, fmt.Errorf("delay parse failed: %w", err)
	}
	return Lambda[Name]{Body: term}, nil
}

func parseLambda(input string) (Term[Name], error) {
	// Skip variable name (not used in De Bruijn indexing)
	bodyStart := strings.Index(input, " ")
	if bodyStart == -1 {
		return nil, fmt.Errorf("invalid lambda format")
	}
	body := strings.TrimSpace(input[bodyStart:])

	term, err := ParseTerm(body)
	if err != nil {
		return nil, fmt.Errorf("lambda body parse failed: %w", err)
	}

	v := Name{
		Text:   "placeholder",
		Unique: Unique(0),
	}

	return Lambda[Name]{ParameterName: v, Body: term}, nil
}

func parseConstant(input string) (Term[Name], error) {
	parts := strings.SplitN(input, " ", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid constant format")
	}

	typ := parts[0]
	value := strings.TrimSpace(parts[1])

	switch typ {
	case "integer":
		val, ok := new(big.Int).SetString(value, 10)
		if !ok {
			return nil, fmt.Errorf("invalid integer: %s", value)
		}

		return Constant{Integer{Inner: val}}, nil
	case "bool":
		val, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("invalid bool: %w", err)
		}

		return Constant{Bool{Inner: val}}, nil
	case "unit":
		return Constant{Unit{}}, nil
	default:
		return nil, errors.New("unknown constant type")
	}
}

func ParseApplication(input string) (Term[Name], error) {
	if !strings.HasPrefix(input, "[") || !strings.HasSuffix(input, "]") {
		return nil, fmt.Errorf("invalid application format")
	}

	inner := strings.TrimSpace(input[1 : len(input)-1])
	if inner == "" {
		return nil, fmt.Errorf("empty application")
	}

	terms, err := splitTerms(inner)
	if err != nil {
		return nil, fmt.Errorf("failed to split application terms: %w", err)
	}

	if len(terms) < 2 {
		return nil, fmt.Errorf("application requires at least 2 terms")
	}

	fun, err := ParseTerm(terms[0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse function: %w", err)
	}

	var result Term[Name] = fun
	for _, argStr := range terms[1:] {
		arg, err := ParseTerm(argStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse argument: %w", err)
		}
		result = Apply[Name]{Function: result, Argument: arg}
	}

	return result, nil
}

func splitTerms(input string) ([]string, error) {
	var terms []string
	var current strings.Builder
	depth := 0

	for _, r := range input {
		switch r {
		case '(', '[':
			depth++
			current.WriteRune(r)
		case ')', ']':
			depth--
			current.WriteRune(r)
		case ' ':
			if depth == 0 {
				if current.Len() > 0 {
					terms = append(terms, current.String())
					current.Reset()
				}
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		terms = append(terms, current.String())
	}

	return terms, nil
}

func parseSimpleTerm(input string) (Term[Name], error) {
	input = strings.TrimSpace(input)

	// debugLog(3, "parseSimpleTerm input: %q", input)

	if _, err := strconv.Atoi(input); err == nil {
		index, err := strconv.Atoi(input)

		if err != nil {
			return nil, fmt.Errorf("invalid variable index: %w", err)
		}

		v := Var[Name]{
			Name: Name{
				Text:   "placeholder",
				Unique: Unique(index),
			},
		}

		return v, nil
	}

	return nil, fmt.Errorf("unsupported term format: %q", input)
}
