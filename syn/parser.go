package syn

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"strconv"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/syn/lex"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

type Parser struct {
	lexer         *lex.Lexer
	curToken      lex.Token
	peekToken     lex.Token
	interned      map[string]Unique
	uniqueCounter Unique
	version       [3]uint32
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
		return fmt.Errorf(
			"expected %v, got %v at position %d",
			typ,
			p.curToken.Type,
			p.curToken.Position,
		)
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

	for i := range 3 {
		if p.curToken.Type != lex.TokenNumber {
			return nil, fmt.Errorf(
				"expected version number, got %v at position %d",
				p.curToken.Type,
				p.curToken.Position,
			)
		}

		n, err := strconv.ParseUint(p.curToken.Literal, 10, 32)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid version number %s at position %d: %w",
				p.curToken.Literal,
				p.curToken.Position,
				err,
			)
		}

		version[i] = uint32(n)

		p.nextToken()

		if i < 2 {
			if err := p.expect(lex.TokenDot); err != nil {
				return nil, err
			}
		}
	}

	p.version = version

	term, err := p.ParseTerm()
	if err != nil {
		return nil, err
	}

	if err := p.expect(lex.TokenRParen); err != nil {
		return nil, err
	}

	if p.curToken.Type != lex.TokenEOF {
		return nil, fmt.Errorf(
			"unexpected token %v after program at position %d",
			p.curToken.Type,
			p.curToken.Position,
		)
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
			return nil, fmt.Errorf(
				"unexpected token %v in term at position %d",
				p.curToken.Type,
				p.curToken.Position,
			)
		}
	case lex.TokenLBracket:
		return p.parseApply()
	default:
		return nil, fmt.Errorf(
			"unexpected token %v in term at position %d",
			p.curToken.Type,
			p.curToken.Position,
		)
	}
}

func (p *Parser) parseLambda() (Term[Name], error) {
	if err := p.expect(lex.TokenLam); err != nil {
		return nil, err
	}

	if p.curToken.Type != lex.TokenIdentifier {
		return nil, fmt.Errorf(
			"expected identifier, got %v at position %d",
			p.curToken.Type,
			p.curToken.Position,
		)
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
		return nil, fmt.Errorf(
			"expected builtin name, got %v at position %d",
			p.curToken.Type,
			p.curToken.Position,
		)
	}

	name := p.curToken.Literal

	fn, ok := builtin.Builtins[name]

	if !ok {
		return nil, fmt.Errorf(
			"unknown builtin function %s at position %d",
			name,
			p.curToken.Position,
		)
	}

	p.nextToken()

	if err := p.expect(lex.TokenRParen); err != nil {
		return nil, err
	}

	return &Builtin{DefaultFunction: fn}, nil // Adjust based on builtin package
}

func (p *Parser) parseConstr() (Term[Name], error) {
	if p.isBefore_v1_1_0() {
		return nil, errors.New("constr can't be used before 1.1.0")
	}

	if err := p.expect(lex.TokenConstr); err != nil {
		return nil, err
	}

	if p.curToken.Type != lex.TokenNumber {
		return nil, fmt.Errorf(
			"expected tag number, got %v at position %d",
			p.curToken.Type,
			p.curToken.Position,
		)
	}

	n, err := strconv.ParseUint(p.curToken.Literal, 10, strconv.IntSize)
	if err != nil {
		return nil, fmt.Errorf(
			"invalid constr tag %s at position %d: %w",
			p.curToken.Literal,
			p.curToken.Position,
			err,
		)
	}

	tag := uint(n)

	p.nextToken()

	fields := make([]Term[Name], 0, 8)

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
	if p.isBefore_v1_1_0() {
		return nil, errors.New("case can't be used before 1.1.0")
	}

	if err := p.expect(lex.TokenCase); err != nil {
		return nil, err
	}

	constr, err := p.ParseTerm()
	if err != nil {
		return nil, err
	}

	branches := make([]Term[Name], 0, 4)

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

	typeSpec, err := p.parseTypeSpec()
	if err != nil {
		return nil, err
	}

	switch ts := typeSpec.(type) {
	case *TInteger:
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
	case *TByteString:
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
	case *TString:
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
	case *TBool:
		var b bool

		switch p.curToken.Type {
		case lex.TokenTrue:
			b = true
		case lex.TokenFalse:
			b = false
		default:
			return nil, fmt.Errorf("expected bool value, got %v at position %d", p.curToken.Type, p.curToken.Position)
		}

		p.nextToken()

		if err := p.expect(lex.TokenRParen); err != nil {
			return nil, err
		}

		return &Constant{Con: &Bool{Inner: b}}, nil
	case *TUnit:
		if p.curToken.Type != lex.TokenUnit {
			return nil, fmt.Errorf("expected unit value, got %v at position %d", p.curToken.Type, p.curToken.Position)
		}

		p.nextToken()

		if err := p.expect(lex.TokenRParen); err != nil {
			return nil, err
		}

		return &Constant{Con: &Unit{}}, nil
	case *TData:
		if err := p.expect(lex.TokenLParen); err != nil {
			return nil, err
		}

		dataVal, err := p.parsePlutusData()
		if err != nil {
			return nil, err
		}

		if err := p.expect(lex.TokenRParen); err != nil {
			return nil, err
		}

		if err := p.expect(lex.TokenRParen); err != nil {
			return nil, err
		}

		return &Constant{Con: &Data{Inner: dataVal}}, nil
	case *TList:
		if err := p.expect(lex.TokenLBracket); err != nil {
			return nil, err
		}

		var items []IConstant

		for p.curToken.Type != lex.TokenRBracket {
			item, err := p.parseConstantValue(ts.Typ)
			if err != nil {
				return nil, err
			}

			if reflect.TypeOf(item.Typ()) != reflect.TypeOf(ts.Typ) {
				return nil, fmt.Errorf("list element of type %T does not match expected type %T at position %d", item.Typ(), ts.Typ, p.curToken.Position)
			}

			items = append(items, item)

			if p.curToken.Type != lex.TokenRBracket {
				if err := p.expect(lex.TokenComma); err != nil {
					return nil, err
				}
			}
		}

		if err := p.expect(lex.TokenRBracket); err != nil {
			return nil, err
		}

		if err := p.expect(lex.TokenRParen); err != nil {
			return nil, err
		}

		return &Constant{Con: &ProtoList{LTyp: ts.Typ, List: items}}, nil
	case *TPair:
		if err := p.expect(lex.TokenLParen); err != nil {
			return nil, err
		}

		first, err := p.parseConstantValue(ts.First)
		if err != nil {
			return nil, err
		}

		if reflect.TypeOf(first.Typ()) != reflect.TypeOf(ts.First) {
			return nil, fmt.Errorf("pair first element of type %T does not match expected type %T at position %d", first.Typ(), ts.First, p.curToken.Position)
		}

		if err := p.expect(lex.TokenComma); err != nil {
			return nil, err
		}

		second, err := p.parseConstantValue(ts.Second)
		if err != nil {
			return nil, err
		}

		if reflect.TypeOf(second.Typ()) != reflect.TypeOf(ts.Second) {
			return nil, fmt.Errorf("pair second element of type %T does not match expected type %T at position %d", second.Typ(), ts.Second, p.curToken.Position)
		}

		if err := p.expect(lex.TokenRParen); err != nil {
			return nil, err
		}

		if err := p.expect(lex.TokenRParen); err != nil {
			return nil, err
		}

		return &Constant{Con: &ProtoPair{FstType: ts.First, SndType: ts.Second, First: first, Second: second}}, nil
	case *TBls12_381G1Element:
		if p.curToken.Type != lex.TokenPoint {
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

		if len(b) != 48 {
			return nil, fmt.Errorf("bls12_381_g1_element must be 48 bytes, got %d", len(b))
		}

		uncompressed := new(bls.G1Affine)

		_, err := uncompressed.SetBytes(b)
		if err != nil {
			return nil, err
		}

		jac := new(bls.G1Jac).FromAffine(uncompressed)

		return &Constant{Con: &Bls12_381G1Element{Inner: jac}}, nil
	case *TBls12_381G2Element:
		if p.curToken.Type != lex.TokenPoint {
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

		if len(b) != 96 {
			return nil, fmt.Errorf("bls12_381_g2_element must be 96 bytes, got %d", len(b))
		}

		uncompressed := new(bls.G2Affine)

		_, err := uncompressed.SetBytes(b)
		if err != nil {
			return nil, err
		}

		jac := new(bls.G2Jac).FromAffine(uncompressed)

		return &Constant{Con: &Bls12_381G2Element{Inner: jac}}, nil
	case *TValue:
		if err := p.expect(lex.TokenLBracket); err != nil {
			return nil, err
		}

		items := make([]IConstant, 0, 8)

		for p.curToken.Type != lex.TokenRBracket {
			if err := p.expect(lex.TokenLParen); err != nil {
				return nil, err
			}

			// Key bytestring
			if p.curToken.Type != lex.TokenByteString {
				return nil, fmt.Errorf("expected bytestring key for value, got %v at position %d", p.curToken.Type, p.curToken.Position)
			}

			kb, ok := p.curToken.Value.([]byte)
			if !ok {
				return nil, fmt.Errorf("invalid bytestring key %s at position %d", p.curToken.Literal, p.curToken.Position)
			}

			// policy key length must be <= 32 bytes
			if len(kb) > 32 {
				return nil, fmt.Errorf("policy key too long (%d bytes) at position %d", len(kb), p.curToken.Position)
			}

			p.nextToken()

			if err := p.expect(lex.TokenComma); err != nil {
				return nil, err
			}

			// Inner list of pairs
			if err := p.expect(lex.TokenLBracket); err != nil {
				return nil, err
			}

			innerItems := make([]IConstant, 0, 8)

			for p.curToken.Type != lex.TokenRBracket {
				if err := p.expect(lex.TokenLParen); err != nil {
					return nil, err
				}

				if p.curToken.Type != lex.TokenByteString {
					return nil, fmt.Errorf("expected bytestring in inner pair, got %v at position %d", p.curToken.Type, p.curToken.Position)
				}

				ib, ok := p.curToken.Value.([]byte)
				if !ok {
					return nil, fmt.Errorf("invalid bytestring value %s at position %d", p.curToken.Literal, p.curToken.Position)
				}

				// token key length must be <= 32 bytes
				if len(ib) > 32 {
					return nil, fmt.Errorf("token key too long (%d bytes) at position %d", len(ib), p.curToken.Position)
				}

				p.nextToken()

				if err := p.expect(lex.TokenComma); err != nil {
					return nil, err
				}

				if p.curToken.Type != lex.TokenNumber {
					return nil, fmt.Errorf("expected integer in inner pair, got %v at position %d", p.curToken.Type, p.curToken.Position)
				}

				n, ok := p.curToken.Value.(*big.Int)
				if !ok {
					return nil, fmt.Errorf("invalid integer value %s at position %d", p.curToken.Literal, p.curToken.Position)
				}
				// Token amounts must fit in the allowed range:
				// minimum: -(2^127), maximum: (2^127 - 1)
				limit := new(big.Int).Lsh(big.NewInt(1), 127)           // 2^127
				limitMinusOne := new(big.Int).Sub(limit, big.NewInt(1)) // 2^127 - 1
				negLimit := new(big.Int).Neg(limit)                     // -2^127
				if n.Sign() >= 0 {
					if n.Cmp(limitMinusOne) > 0 {
						return nil, fmt.Errorf("integer in value token out of range %s at position %d", p.curToken.Literal, p.curToken.Position)
					}
				} else {
					if n.Cmp(negLimit) < 0 {
						return nil, fmt.Errorf("integer in value token out of range %s at position %d", p.curToken.Literal, p.curToken.Position)
					}
				}

				p.nextToken()

				if err := p.expect(lex.TokenRParen); err != nil {
					return nil, err
				}

				innerItems = append(innerItems, &ProtoPair{FstType: &TByteString{}, SndType: &TInteger{}, First: &ByteString{Inner: ib}, Second: &Integer{Inner: n}})

				if p.curToken.Type != lex.TokenRBracket {
					if err := p.expect(lex.TokenComma); err != nil {
						return nil, err
					}
				}
			}

			if err := p.expect(lex.TokenRBracket); err != nil {
				return nil, err
			}

			if err := p.expect(lex.TokenRParen); err != nil {
				return nil, err
			}

			outerPair := &ProtoPair{
				FstType: &TByteString{},
				SndType: &TList{Typ: &TPair{First: &TByteString{}, Second: &TInteger{}}},
				First:   &ByteString{Inner: kb},
				Second:  &ProtoList{LTyp: &TPair{First: &TByteString{}, Second: &TInteger{}}, List: innerItems},
			}

			items = append(items, outerPair)

			if p.curToken.Type != lex.TokenRBracket {
				if err := p.expect(lex.TokenComma); err != nil {
					return nil, err
				}
			}
		}

		if err := p.expect(lex.TokenRBracket); err != nil {
			return nil, err
		}

		if err := p.expect(lex.TokenRParen); err != nil {
			return nil, err
		}

		// Canonicalize the value constant: merge duplicate token keys by summing,
		// remove zero amounts and empty policy entries, and sort deterministically.
		vm := make(map[string]map[string]*big.Int)

		for _, it := range items {
			pp, ok := it.(*ProtoPair)
			if !ok {
				continue
			}
			polBs, ok := pp.First.(*ByteString)
			pol := ""
			if ok {
				pol = string(polBs.Inner)
			}
			if _, exists := vm[pol]; !exists {
				vm[pol] = make(map[string]*big.Int)
			}
			if lst, ok := pp.Second.(*ProtoList); ok {
				for _, tk := range lst.List {
					tkp, ok := tk.(*ProtoPair)
					if !ok {
						continue
					}
					tkBs, ok1 := tkp.First.(*ByteString)
					amt, ok2 := tkp.Second.(*Integer)
					if ok1 && ok2 {
						key := string(tkBs.Inner)
						val := new(big.Int).Set(amt.Inner)
						if cur, ex := vm[pol][key]; ex {
							vm[pol][key] = new(big.Int).Add(cur, val)
						} else {
							vm[pol][key] = val
						}
					}
				}
			}
		}

		// Validate summed token amounts fit in the allowed range
		limit := new(big.Int).Lsh(big.NewInt(1), 127)           // 2^127
		limitMinusOne := new(big.Int).Sub(limit, big.NewInt(1)) // 2^127 - 1
		negLimit := new(big.Int).Neg(limit)                     // -2^127
		for pol, tokens := range vm {
			for key, amt := range tokens {
				if amt.Sign() >= 0 {
					if amt.Cmp(limitMinusOne) > 0 {
						return nil, fmt.Errorf("summed token amount for policy %q key %q out of range %s", pol, key, amt.String())
					}
				} else {
					if amt.Cmp(negLimit) < 0 {
						return nil, fmt.Errorf("summed token amount for policy %q key %q out of range %s", pol, key, amt.String())
					}
				}
			}
		}

		// Build canonical list
		policies := make([]string, 0, len(vm))
		for policy := range vm {
			policies = append(policies, policy)
		}
		sort.Strings(policies)

		res := make([]IConstant, 0)
		for _, pol := range policies {
			tokens := vm[pol]
			// collect non-zero tokens
			if len(tokens) == 0 {
				continue
			}
			keys := make([]string, 0, len(tokens))
			for k := range tokens {
				if tokens[k] == nil || tokens[k].Sign() == 0 {
					continue
				}
				keys = append(keys, k)
			}
			if len(keys) == 0 {
				continue
			}
			sort.Strings(keys)
			inner := make([]IConstant, 0, len(keys))
			for _, k := range keys {
				inner = append(inner, &ProtoPair{FstType: &TByteString{}, SndType: &TInteger{}, First: &ByteString{Inner: []byte(k)}, Second: &Integer{Inner: tokens[k]}})
			}
			res = append(res, &ProtoPair{FstType: &TByteString{}, SndType: &TList{Typ: &TPair{First: &TByteString{}, Second: &TInteger{}}}, First: &ByteString{Inner: []byte(pol)}, Second: &ProtoList{LTyp: &TPair{First: &TByteString{}, Second: &TInteger{}}, List: inner}})
		}

		valType := &TPair{First: &TByteString{}, Second: &TList{Typ: &TPair{First: &TByteString{}, Second: &TInteger{}}}}

		return &Constant{Con: &ProtoList{LTyp: valType, List: res}}, nil
	default:
		return nil, fmt.Errorf("unexpected type spec %v at position %d", typeSpec, p.curToken.Position)
	}
}

func (p *Parser) parseConstantValue(typ Typ) (IConstant, error) {
	switch t := typ.(type) {
	case *TInteger:
		if p.curToken.Type != lex.TokenNumber {
			return nil, fmt.Errorf("expected integer value, got %v at position %d", p.curToken.Type, p.curToken.Position)
		}

		n, ok := p.curToken.Value.(*big.Int)
		if !ok {
			return nil, fmt.Errorf("invalid integer value %s at position %d", p.curToken.Literal, p.curToken.Position)
		}

		p.nextToken()

		return &Integer{Inner: n}, nil
	case *TByteString:
		if p.curToken.Type != lex.TokenByteString {
			return nil, fmt.Errorf("expected bytestring value, got %v at position %d", p.curToken.Type, p.curToken.Position)
		}

		b, ok := p.curToken.Value.([]byte)
		if !ok {
			return nil, fmt.Errorf("invalid bytestring value %s at position %d", p.curToken.Literal, p.curToken.Position)
		}

		p.nextToken()

		return &ByteString{Inner: b}, nil
	case *TString:
		if p.curToken.Type != lex.TokenString {
			return nil, fmt.Errorf("expected string value, got %v at position %d", p.curToken.Type, p.curToken.Position)
		}

		s, ok := p.curToken.Value.(string)
		if !ok {
			return nil, fmt.Errorf("invalid string value %s at position %d", p.curToken.Literal, p.curToken.Position)
		}

		p.nextToken()

		return &String{Inner: s}, nil
	case *TBool:
		var b bool

		switch p.curToken.Type {
		case lex.TokenTrue:
			b = true
		case lex.TokenFalse:
			b = false
		default:
			return nil, fmt.Errorf("expected bool value, got %v at position %d", p.curToken.Type, p.curToken.Position)
		}

		p.nextToken()

		return &Bool{Inner: b}, nil
	case *TUnit:
		if p.curToken.Type != lex.TokenUnit {
			return nil, fmt.Errorf("expected unit value, got %v at position %d", p.curToken.Type, p.curToken.Position)
		}

		p.nextToken()

		return &Unit{}, nil
	case *TBls12_381G1Element:
		if p.curToken.Type != lex.TokenPoint {
			return nil, fmt.Errorf("expected point value, got %v at position %d", p.curToken.Type, p.curToken.Position)
		}

		b, ok := p.curToken.Value.([]byte)
		if !ok {
			return nil, fmt.Errorf("invalid bytestring value %s at position %d", p.curToken.Literal, p.curToken.Position)
		}

		p.nextToken()

		if len(b) != 48 {
			return nil, fmt.Errorf("bls12_381_g1_element must be 48 bytes, got %d", len(b))
		}

		uncompressed := new(bls.G1Affine)

		_, err := uncompressed.SetBytes(b)
		if err != nil {
			return nil, err
		}

		jac := new(bls.G1Jac).FromAffine(uncompressed)

		return &Bls12_381G1Element{Inner: jac}, nil
	case *TBls12_381G2Element:
		if p.curToken.Type != lex.TokenPoint {
			return nil, fmt.Errorf("expected point value, got %v at position %d", p.curToken.Type, p.curToken.Position)
		}

		b, ok := p.curToken.Value.([]byte)
		if !ok {
			return nil, fmt.Errorf("invalid bytestring value %s at position %d", p.curToken.Literal, p.curToken.Position)
		}

		p.nextToken()

		if len(b) != 96 {
			return nil, fmt.Errorf("bls12_381_g2_element must be 96 bytes, got %d", len(b))
		}

		uncompressed := new(bls.G2Affine)

		_, err := uncompressed.SetBytes(b)
		if err != nil {
			return nil, err
		}

		jac := new(bls.G2Jac).FromAffine(uncompressed)

		return &Bls12_381G2Element{Inner: jac}, nil
	case *TData:
		dataVal, err := p.parsePlutusData()
		if err != nil {
			return nil, err
		}

		return &Data{Inner: dataVal}, nil
	case *TList:
		if err := p.expect(lex.TokenLBracket); err != nil {
			return nil, err
		}

		var items []IConstant

		for p.curToken.Type != lex.TokenRBracket {
			item, err := p.parseConstantValue(t.Typ)
			if err != nil {
				return nil, err
			}

			if reflect.TypeOf(item.Typ()) != reflect.TypeOf(t.Typ) {
				return nil, fmt.Errorf("list element of type %T does not match expected type %T at position %d", item.Typ(), t.Typ, p.curToken.Position)
			}

			items = append(items, item)

			if p.curToken.Type != lex.TokenRBracket {
				if err := p.expect(lex.TokenComma); err != nil {
					return nil, err
				}
			}
		}

		if err := p.expect(lex.TokenRBracket); err != nil {
			return nil, err
		}

		return &ProtoList{LTyp: t.Typ, List: items}, nil
	case *TPair:
		if err := p.expect(lex.TokenLParen); err != nil {
			return nil, err
		}

		first, err := p.parseConstantValue(t.First)
		if err != nil {
			return nil, err
		}

		if reflect.TypeOf(first.Typ()) != reflect.TypeOf(t.First) {
			return nil, fmt.Errorf("pair first element of type %T does not match expected type %T at position %d", first.Typ(), t.First, p.curToken.Position)
		}

		if err := p.expect(lex.TokenComma); err != nil {
			return nil, err
		}

		second, err := p.parseConstantValue(t.Second)
		if err != nil {
			return nil, err
		}

		if reflect.TypeOf(second.Typ()) != reflect.TypeOf(t.Second) {
			return nil, fmt.Errorf("pair second element of type %T does not match expected type %T at position %d", second.Typ(), t.Second, p.curToken.Position)
		}

		if err := p.expect(lex.TokenRParen); err != nil {
			return nil, err
		}

		return &ProtoPair{FstType: t.First, SndType: t.Second, First: first, Second: second}, nil
	default:
		return nil, fmt.Errorf("unexpected type %v at position %d", typ, p.curToken.Position)
	}
}

func (p *Parser) parsePlutusData() (data.PlutusData, error) {
	switch p.curToken.Type {
	case lex.TokenI:
		p.nextToken()

		if p.curToken.Type != lex.TokenNumber {
			return nil, fmt.Errorf(
				"expected integer value for I, got %v at position %d",
				p.curToken.Type,
				p.curToken.Position,
			)
		}

		n, ok := p.curToken.Value.(*big.Int)
		if !ok {
			return nil, fmt.Errorf(
				"invalid integer value %s at position %d",
				p.curToken.Literal,
				p.curToken.Position,
			)
		}

		p.nextToken()

		return data.NewInteger(n), nil
	case lex.TokenB:
		p.nextToken()

		if p.curToken.Type != lex.TokenByteString {
			return nil, fmt.Errorf(
				"expected bytestring value for B, got %v at position %d",
				p.curToken.Type,
				p.curToken.Position,
			)
		}

		b, ok := p.curToken.Value.([]byte)
		if !ok {
			return nil, fmt.Errorf(
				"invalid bytestring value %s at position %d",
				p.curToken.Literal,
				p.curToken.Position,
			)
		}

		p.nextToken()

		return data.NewByteString(b), nil
	case lex.TokenPlutusList:
		p.nextToken()

		if err := p.expect(lex.TokenLBracket); err != nil {
			return nil, err
		}

		var items []data.PlutusData

		for p.curToken.Type != lex.TokenRBracket {
			item, err := p.parsePlutusData()
			if err != nil {
				return nil, err
			}

			items = append(items, item)

			if p.curToken.Type != lex.TokenRBracket {
				if err := p.expect(lex.TokenComma); err != nil {
					return nil, err
				}
			}
		}

		if err := p.expect(lex.TokenRBracket); err != nil {
			return nil, err
		}

		return data.NewList(items...), nil
	case lex.TokenMap:
		p.nextToken()

		if err := p.expect(lex.TokenLBracket); err != nil {
			return nil, err
		}

		var pairs [][2]data.PlutusData

		for p.curToken.Type != lex.TokenRBracket {
			if err := p.expect(lex.TokenLParen); err != nil {
				return nil, err
			}

			key, err := p.parsePlutusData()
			if err != nil {
				return nil, err
			}

			if err := p.expect(lex.TokenComma); err != nil {
				return nil, err
			}

			value, err := p.parsePlutusData()
			if err != nil {
				return nil, err
			}

			if err := p.expect(lex.TokenRParen); err != nil {
				return nil, err
			}

			pairs = append(pairs, [2]data.PlutusData{key, value})

			if p.curToken.Type != lex.TokenRBracket {
				if err := p.expect(lex.TokenComma); err != nil {
					return nil, err
				}
			}
		}

		if err := p.expect(lex.TokenRBracket); err != nil {
			return nil, err
		}

		return data.NewMap(pairs), nil
	case lex.TokenPlutusConstr:
		p.nextToken()

		if p.curToken.Type != lex.TokenNumber {
			return nil, fmt.Errorf(
				"expected tag number for Constr, got %v at position %d",
				p.curToken.Type,
				p.curToken.Position,
			)
		}

		nu, err := strconv.ParseUint(p.curToken.Literal, 10, 64)
		if err != nil {
			return nil, fmt.Errorf(
				"invalid constr tag %s at position %d: %w",
				p.curToken.Literal,
				p.curToken.Position,
				err,
			)
		}

		if nu > uint64(^uint(0)) {
			return nil, fmt.Errorf(
				"constr tag %d out of range at position %d",
				nu,
				p.curToken.Position,
			)
		}
		tag := uint(nu)

		p.nextToken()

		if err := p.expect(lex.TokenLBracket); err != nil {
			return nil, err
		}

		var fields []data.PlutusData

		for p.curToken.Type != lex.TokenRBracket {
			field, err := p.parsePlutusData()
			if err != nil {
				return nil, err
			}

			fields = append(fields, field)

			if p.curToken.Type != lex.TokenRBracket {
				if err := p.expect(lex.TokenComma); err != nil {
					return nil, err
				}
			}
		}

		if err := p.expect(lex.TokenRBracket); err != nil {
			return nil, err
		}

		return data.NewConstr(tag, fields...), nil
	default:
		return nil, fmt.Errorf(
			"expected PlutusData constructor (I, B, List, Map, Constr), got %v at position %d",
			p.curToken.Type,
			p.curToken.Position,
		)
	}
}

func (p *Parser) parseTypeSpec() (Typ, error) {
	// Check for invalid bare list or pair
	if p.curToken.Type == lex.TokenList || p.curToken.Type == lex.TokenPair {
		return nil, fmt.Errorf(
			"expected left parenthesis for %d type, got %v (literal: %s) at position %d",
			p.curToken.Type,
			p.curToken.Type,
			p.curToken.Literal,
			p.curToken.Position,
		)
	}

	// Handle parenthesized type specs (e.g., (list data), (pair integer bool))
	if p.curToken.Type == lex.TokenLParen {
		p.nextToken()

		typ, err := p.parseInnerTypeSpec()
		if err != nil {
			return nil, err
		}

		if p.curToken.Type != lex.TokenRParen {
			return nil, fmt.Errorf(
				"expected right parenthesis after type spec, got %v (literal: %s) at position %d",
				p.curToken.Type,
				p.curToken.Literal,
				p.curToken.Position,
			)
		}

		p.nextToken()

		return typ, nil
	}

	// Handle simple types (e.g., integer, data)
	return p.parseInnerTypeSpec()
}

func (p *Parser) parseInnerTypeSpec() (Typ, error) {
	switch p.curToken.Type {
	case lex.TokenIdentifier:
		typName := p.curToken.Literal

		p.nextToken()

		switch typName {
		case "integer":
			return &TInteger{}, nil
		case "bytestring":
			return &TByteString{}, nil
		case "string":
			return &TString{}, nil
		case "unit":
			return &TUnit{}, nil
		case "bool":
			return &TBool{}, nil
		case "data":
			return &TData{}, nil
		case "bls12_381_G1_element":
			return &TBls12_381G1Element{}, nil
		case "bls12_381_G2_element":
			return &TBls12_381G2Element{}, nil
		case "value":
			return &TValue{}, nil
		default:
			return nil, fmt.Errorf(
				"unknown type %s at position %d",
				typName,
				p.curToken.Position,
			)
		}
	case lex.TokenList, lex.TokenArray:
		p.nextToken()

		// Parse element type, which may be parenthesized (e.g., (list data)) or simple (e.g., data)
		elemType, err := p.parseTypeSpec()
		if err != nil {
			return nil, err
		}

		return &TList{Typ: elemType}, nil
	case lex.TokenPair:
		p.nextToken()

		// Parse two types, each may be parenthesized or simple
		firstType, err := p.parseTypeSpec()
		if err != nil {
			return nil, err
		}

		secondType, err := p.parseTypeSpec()
		if err != nil {
			return nil, err
		}

		return &TPair{First: firstType, Second: secondType}, nil
	default:
		return nil, fmt.Errorf(
			"expected type identifier, list, or pair, got %v (literal: %s) at position %d",
			p.curToken.Type,
			p.curToken.Literal,
			p.curToken.Position,
		)
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
		return nil, fmt.Errorf(
			"application requires at least two terms, got %d at position %d",
			len(terms),
			p.curToken.Position,
		)
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

func (p *Parser) isBefore_v1_1_0() bool {
	return p.version[0] < 2 && p.version[1] < 1
}
