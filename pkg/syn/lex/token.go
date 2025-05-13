package lex

type TokenType int

const (
	TokenEOF TokenType = iota
	TokenError
	TokenLParen       // (
	TokenRParen       // )
	TokenLBracket     // [
	TokenRBracket     // ]
	TokenDot          // .
	TokenComma        // ,
	TokenNumber       // e.g., 123
	TokenIdentifier   // e.g., x, addInteger
	TokenString       // e.g., "hello"
	TokenByteString   // e.g., #aaBB
	TokenTrue         // True
	TokenFalse        // False
	TokenUnit         // ()
	TokenLam          // lam
	TokenDelay        // delay
	TokenForce        // force
	TokenBuiltin      // builtin
	TokenConstr       // constr
	TokenCase         // case
	TokenCon          // con
	TokenErrorTerm    // error
	TokenProgram      // program
	TokenList         // list
	TokenPair         // pair
	TokenI            // I
	TokenB            // B
	TokenPlutusList   // List
	TokenMap          // Map
	TokenPlutusConstr // Constr
)

type Token struct {
	Type     TokenType
	Literal  string
	Position int
	Value    interface{} // For numbers (*big.Int), strings (string), bytestrings ([]byte)
}
