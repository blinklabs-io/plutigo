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

package syn

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
)

// ownershipProgram builds programs of identical shape whose constants differ
// by seed, so that a decode into recycled arena slots overwrites every field
// of an earlier decode.
func ownershipProgram(seed byte) *Program[DeBruijn] {
	n := int64(seed)
	return &Program[DeBruijn]{
		Version: lang.LanguageVersionV3,
		Term: &Apply[DeBruijn]{
			Function: &Lambda[DeBruijn]{
				ParameterName: DeBruijn(0),
				Body: &Constr[DeBruijn]{
					Tag: uint64(seed),
					Fields: []Term[DeBruijn]{
						&Var[DeBruijn]{Name: DeBruijn(1)},
						&Constant{Con: &ByteString{Inner: []byte{seed, seed + 1, seed + 2}}},
						&Constant{Con: &String{Inner: string(rune('a' + seed%26))}},
						&Constant{Con: &Integer{Inner: new(big.Int).Lsh(big.NewInt(n+1), 90)}},
						&Constant{Con: &ProtoList{
							LTyp: &TInteger{},
							List: []IConstant{
								&Integer{Inner: big.NewInt(n)},
								&Integer{Inner: big.NewInt(n + 100)},
							},
						}},
						&Constant{Con: &ProtoList{
							LTyp: &TPair{First: &TInteger{}, Second: &TData{}},
							List: []IConstant{&ProtoPair{
								FstType: &TInteger{},
								SndType: &TData{},
								First:   &Integer{Inner: big.NewInt(-n)},
								Second: &Data{Inner: &data.Constr{
									Tag: big.NewInt(n),
									Fields: []data.PlutusData{
										&data.ByteString{Inner: []byte{seed, 0xaa}},
										&data.List{Items: []data.PlutusData{
											&data.Integer{Inner: big.NewInt(n + 7)},
										}},
									},
								}},
							}},
						}},
					},
				},
			},
			Argument: &Constant{Con: &Integer{Inner: big.NewInt(n)}},
		},
	}
}

func mustEncodeOwnership(t *testing.T, p *Program[DeBruijn]) []byte {
	t.Helper()
	encoded, err := Encode(p)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	return encoded
}

func TestDeBruijnDecoderDecodeResultsAreOwned(t *testing.T) {
	t.Parallel()
	encodedA := mustEncodeOwnership(t, ownershipProgram(3))
	encodedB := mustEncodeOwnership(t, ownershipProgram(11))

	for _, tt := range []struct {
		name   string
		decode func(*DeBruijnDecoder, []byte) (*Program[DeBruijn], error)
	}{
		{"Decode", (*DeBruijnDecoder).Decode},
		{"Decode after DecodeBorrowed", (*DeBruijnDecoder).Decode},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			decoder := NewDeBruijnDecoder()
			if tt.name != "Decode" {
				// Leave the decoder's arenas populated by a borrowed decode.
				if _, err := decoder.DecodeBorrowed(encodedB); err != nil {
					t.Fatalf("borrowed decode failed: %v", err)
				}
			}
			a, err := tt.decode(decoder, encodedA)
			if err != nil {
				t.Fatalf("decode A failed: %v", err)
			}
			wantPretty := Pretty(a)
			wantFlat := mustEncodeOwnership(t, a)

			for range 3 {
				if _, err := tt.decode(decoder, encodedB); err != nil {
					t.Fatalf("decode B failed: %v", err)
				}
				if _, err := decoder.DecodeBorrowed(encodedB); err != nil {
					t.Fatalf("borrowed decode B failed: %v", err)
				}
			}

			if got := Pretty(a); got != wantPretty {
				t.Fatalf("retained program changed by later decode:\n got: %s\nwant: %s", got, wantPretty)
			}
			if got := mustEncodeOwnership(t, a); !bytes.Equal(got, wantFlat) {
				t.Fatalf("retained program re-encodes differently after later decode")
			}
			if !bytes.Equal(wantFlat, encodedA) {
				t.Fatalf("decoded program A does not match its input")
			}
		})
	}
}

func TestDeBruijnDecoderDecodeBorrowed(t *testing.T) {
	t.Parallel()
	encodedA := mustEncodeOwnership(t, ownershipProgram(3))
	encodedB := mustEncodeOwnership(t, ownershipProgram(11))
	decoder := NewDeBruijnDecoder()

	// Each borrowed result is correct until the next decode on the decoder.
	for i, encoded := range [][]byte{encodedA, encodedB, encodedA} {
		p, err := decoder.DecodeBorrowed(encoded)
		if err != nil {
			t.Fatalf("borrowed decode %d failed: %v", i, err)
		}
		if got := mustEncodeOwnership(t, p); !bytes.Equal(got, encoded) {
			t.Fatalf("borrowed decode %d does not round-trip", i)
		}
	}
	if _, err := decoder.DecodeBorrowed([]byte{0xff}); err == nil {
		t.Fatalf("borrowed decode of malformed input succeeded")
	}
}
