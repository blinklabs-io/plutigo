// Copyright 2026 Blink Labs Software
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	TokenPoint        // e.g., 0xc0000
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
	TokenArray        // array
	TokenPair         // pair
	TokenI            // I
	TokenB            // B
	TokenPlutusList   // List
	TokenMap          // Map
	TokenPlutusConstr // Constr
	TokenPlutusValue  // V
)

type Token struct {
	Type     TokenType
	Literal  string
	Position int
	Value    any // For numbers (*big.Int), strings (string), bytestrings ([]byte)
}
