package syn

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/blinklabs-io/plutigo/pkg/builtin"
	"github.com/blinklabs-io/plutigo/pkg/syn/lex"
)

type Parser struct {
	lexer         *lex.Lexer
	curToken      lex.Token
	peekToken     lex.Token
	interned      map[string]Unique
	uniqueCounter Unique
}

func NewParser(input string) *Parser {
	p := &Parser{
		lexer:         lex.NewLexer(input),
		interned:      make(map[string]Unique),
		uniqueCounter: 0,
	}

	p.curToken = p.lexer.NextToken()
	p.peekToken = p.lexer.NextToken()

	return p
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken

	p.peekToken = p.lexer.NextToken()
}

func (p *Parser) expect(typ lex.TokenType) error {
	if p.curToken.Type != typ {
		return fmt.Errorf("expected %v, got %v at position %d", typ, p.curToken.Type, p.curToken.Position)
	}

	p.nextToken()

	return nil
}

func (p *Parser) internName(text string) Name {
	if unique, exists := p.interned[text]; exists {
		return Name{Text: text, Unique: unique}
	}

	unique := p.uniqueCounter

	p.interned[text] = unique

	p.uniqueCounter++

	return Name{Text: text, Unique: unique}
}

func Parse(input string) (*Program[Name], error) {
	p := NewParser(input)

	return p.ParseProgram()
}

func (p *Parser) ParseProgram() (*Program[Name], error) {
	if err := p.expect(lex.TokenLParen); err != nil {
		return nil, err
	}

	if err := p.expect(lex.TokenProgram); err != nil {
		return nil, err
	}

	var version [3]uint32

	for i := 0; i < 3; i++ {
		if p.curToken.Type != lex.TokenNumber {
			return nil, fmt.Errorf("expected version number, got %v at position %d", p.curToken.Type, p.curToken.Position)
		}

		n, err := strconv.ParseUint(p.curToken.Literal, 10, 32)

		if err != nil {
			return nil, fmt.Errorf("invalid version number %s at position %d: %v", p.curToken.Literal, p.curToken.Position, err)
		}

		version[i] = uint32(n)

		p.nextToken()

		if i < 2 {
			if err := p.expect(lex.TokenDot); err != nil {
				return nil, err
			}
		}
	}

	term, err := p.ParseTerm()

	if err != nil {
		return nil, err
	}

	if err := p.expect(lex.TokenRParen); err != nil {
		return nil, err
	}

	if p.curToken.Type != lex.TokenEOF {
		return nil, fmt.Errorf("unexpected token %v after program at position %d", p.curToken.Type, p.curToken.Position)
	}

	return &Program[Name]{Version: version, Term: term}, nil
}

func (p *Parser) ParseTerm() (Term[Name], error) {
	switch p.curToken.Type {
	case lex.TokenIdentifier:
		name := p.internName(p.curToken.Literal)

		p.nextToken()

		return &Var[Name]{Name: name}, nil
	case lex.TokenLParen:
		p.nextToken()
		switch p.curToken.Type {
		case lex.TokenLam:
			return p.parseLambda()
		case lex.TokenDelay:
			return p.parseDelay()
		case lex.TokenForce:
			return p.parseForce()
		case lex.TokenBuiltin:
			return p.parseBuiltin()
		case lex.TokenConstr:
			return p.parseConstr()
		case lex.TokenCase:
			return p.parseCase()
		case lex.TokenCon:
			return p.parseConstant()
		case lex.TokenErrorTerm:
			p.nextToken()

			if err := p.expect(lex.TokenRParen); err != nil {
				return nil, err
			}

			return &Error{}, nil
		default:
			return nil, fmt.Errorf("unexpected token %v in term at position %d", p.curToken.Type, p.curToken.Position)
		}
	case lex.TokenLBracket:
		return p.parseApply()
	default:
		return nil, fmt.Errorf("unexpected token %v in term at position %d", p.curToken.Type, p.curToken.Position)
	}
}

func (p *Parser) parseLambda() (Term[Name], error) {
	if err := p.expect(lex.TokenLam); err != nil {
		return nil, err
	}

	if p.curToken.Type != lex.TokenIdentifier {
		return nil, fmt.Errorf("expected identifier, got %v at position %d", p.curToken.Type, p.curToken.Position)
	}

	name := p.internName(p.curToken.Literal)

	p.nextToken()

	body, err := p.ParseTerm()

	if err != nil {
		return nil, err
	}

	if err := p.expect(lex.TokenRParen); err != nil {
		return nil, err
	}

	return &Lambda[Name]{ParameterName: name, Body: body}, nil
}

func (p *Parser) parseDelay() (Term[Name], error) {
	if err := p.expect(lex.TokenDelay); err != nil {
		return nil, err
	}

	term, err := p.ParseTerm()

	if err != nil {
		return nil, err
	}

	if err := p.expect(lex.TokenRParen); err != nil {
		return nil, err
	}

	return &Delay[Name]{Term: term}, nil
}

func (p *Parser) parseForce() (Term[Name], error) {
	if err := p.expect(lex.TokenForce); err != nil {
		return nil, err
	}

	term, err := p.ParseTerm()

	if err != nil {
		return nil, err
	}

	if err := p.expect(lex.TokenRParen); err != nil {
		return nil, err
	}

	return &Force[Name]{Term: term}, nil
}

func (p *Parser) parseBuiltin() (Term[Name], error) {
	if err := p.expect(lex.TokenBuiltin); err != nil {
		return nil, err
	}

	if p.curToken.Type != lex.TokenIdentifier {
		return nil, fmt.Errorf("expected builtin name, got %v at position %d", p.curToken.Type, p.curToken.Position)
	}

	name := p.curToken.Literal

	fn, ok := builtin.Builtins[name]

	if !ok {
		return nil, fmt.Errorf("unknown builtin function %s at position %d", name, p.curToken.Position)
	}

	p.nextToken()

	if err := p.expect(lex.TokenRParen); err != nil {
		return nil, err
	}

	return &Builtin{DefaultFunction: fn}, nil // Adjust based on builtin package
}

func (p *Parser) parseConstr() (Term[Name], error) {
	if err := p.expect(lex.TokenConstr); err != nil {
		return nil, err
	}

	if p.curToken.Type != lex.TokenNumber {
		return nil, fmt.Errorf("expected tag number, got %v at position %d", p.curToken.Type, p.curToken.Position)
	}

	n, err := strconv.ParseUint(p.curToken.Literal, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid constr tag %s at position %d: %v", p.curToken.Literal, p.curToken.Position, err)
	}

	tag := n

	p.nextToken()

	var fields []Term[Name]

	for p.curToken.Type != lex.TokenRParen {
		term, err := p.ParseTerm()
		if err != nil {
			return nil, err
		}

		fields = append(fields, term)
	}

	if err := p.expect(lex.TokenRParen); err != nil {
		return nil, err
	}

	return &Constr[Name]{Tag: uint(tag), Fields: fields}, nil
}

func (p *Parser) parseCase() (Term[Name], error) {
	if err := p.expect(lex.TokenCase); err != nil {
		return nil, err
	}

	constr, err := p.ParseTerm()
	if err != nil {
		return nil, err
	}

	var branches []Term[Name]

	for p.curToken.Type != lex.TokenRParen {
		branch, err := p.ParseTerm()
		if err != nil {
			return nil, err
		}

		branches = append(branches, branch)
	}

	if err := p.expect(lex.TokenRParen); err != nil {
		return nil, err
	}

	return &Case[Name]{Constr: constr, Branches: branches}, nil
}

func (p *Parser) parseConstant() (Term[Name], error) {
	if err := p.expect(lex.TokenCon); err != nil {
		return nil, err
	}

	switch p.curToken.Type {
	case lex.TokenIdentifier:
		switch p.curToken.Literal {
		case "integer":
			p.nextToken()

			if p.curToken.Type != lex.TokenNumber {
				return nil, fmt.Errorf("expected integer value, got %v at position %d", p.curToken.Type, p.curToken.Position)
			}

			n, ok := p.curToken.Value.(*big.Int)
			if !ok {
				return nil, fmt.Errorf("invalid integer value %s at position %d", p.curToken.Literal, p.curToken.Position)
			}

			p.nextToken()

			if err := p.expect(lex.TokenRParen); err != nil {
				return nil, err
			}

			return &Constant{Con: &Integer{Inner: n}}, nil
		case "bytestring":
			p.nextToken()

			if p.curToken.Type != lex.TokenByteString {
				return nil, fmt.Errorf("expected bytestring value, got %v at position %d", p.curToken.Type, p.curToken.Position)
			}

			b, ok := p.curToken.Value.([]byte)
			if !ok {
				return nil, fmt.Errorf("invalid bytestring value %s at position %d", p.curToken.Literal, p.curToken.Position)
			}

			p.nextToken()

			if err := p.expect(lex.TokenRParen); err != nil {
				return nil, err
			}
			return &Constant{Con: &ByteString{Inner: b}}, nil
		case "string":
			p.nextToken()

			if p.curToken.Type != lex.TokenString {
				return nil, fmt.Errorf("expected string value, got %v at position %d", p.curToken.Type, p.curToken.Position)
			}

			s, ok := p.curToken.Value.(string)
			if !ok {
				return nil, fmt.Errorf("invalid string value %s at position %d", p.curToken.Literal, p.curToken.Position)
			}

			p.nextToken()

			if err := p.expect(lex.TokenRParen); err != nil {
				return nil, err
			}

			return &Constant{Con: &String{Inner: s}}, nil
		case "bool":
			p.nextToken()

			var b bool

			if p.curToken.Type == lex.TokenTrue {
				b = true
			} else if p.curToken.Type == lex.TokenFalse {
				b = false
			} else {
				return nil, fmt.Errorf("expected bool value, got %v at position %d", p.curToken.Type, p.curToken.Position)
			}

			p.nextToken()

			if err := p.expect(lex.TokenRParen); err != nil {
				return nil, err
			}

			return &Constant{Con: &Bool{Inner: b}}, nil
		case "unit":
			p.nextToken()

			if p.curToken.Type != lex.TokenUnit {
				return nil, fmt.Errorf("expected unit value, got %v at position %d", p.curToken.Type, p.curToken.Position)
			}

			p.nextToken()

			if err := p.expect(lex.TokenRParen); err != nil {
				return nil, err
			}

			return &Constant{Con: &Unit{}}, nil
		default:
			return nil, fmt.Errorf("unknown constant type %s at position %d", p.curToken.Literal, p.curToken.Position)
		}
	default:
		return nil, fmt.Errorf("expected constant type, got %v at position %d", p.curToken.Type, p.curToken.Position)
	}
}
func (p *Parser) parseApply() (Term[Name], error) {
	if err := p.expect(lex.TokenLBracket); err != nil {
		return nil, err
	}
	var terms []Term[Name]
	for p.curToken.Type != lex.TokenRBracket {
		term, err := p.ParseTerm()
		if err != nil {
			return nil, err
		}
		terms = append(terms, term)
	}
	if len(terms) < 2 {
		return nil, fmt.Errorf("application requires at least two terms, got %d at position %d", len(terms), p.curToken.Position)
	}
	if err := p.expect(lex.TokenRBracket); err != nil {
		return nil, err
	}
	// Build left-nested Apply structure
	result := terms[0]
	for i := 1; i < len(terms); i++ {
		result = &Apply[Name]{Function: result, Argument: terms[i]}
	}
	return result, nil
}
