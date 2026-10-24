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

package data

import (
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"
)

func nestedListJSON(depth int) string {
	var sb strings.Builder
	for range depth {
		sb.WriteString(`{"list":[`)
	}
	sb.WriteString(`{"int":0}`)
	for range depth {
		sb.WriteString(`]}`)
	}
	return sb.String()
}

func nestedMapKeyJSON(depth int) string {
	var sb strings.Builder
	for range depth {
		sb.WriteString(`{"map":[{"k":`)
	}
	sb.WriteString(`{"int":0}`)
	for range depth {
		sb.WriteString(`,"v":{"int":0}}]}`)
	}
	return sb.String()
}

func wideFlatJSON(items int) string {
	var sb strings.Builder
	sb.WriteString(`{"list":[`)
	for i := range items {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`{"int":1}`)
	}
	sb.WriteString(`]}`)
	return sb.String()
}

func wideEmptyConstrListJSON(items int) string {
	var sb strings.Builder
	sb.WriteString(`{"list":[`)
	for i := range items {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`{"constructor":0,"fields":[]}`)
	}
	sb.WriteString(`]}`)
	return sb.String()
}

// TestDecodeJSONDepthLimit verifies the JSON PlutusData decoder enforces the
// same nesting-depth limit as the CBOR decoder, rather than recursing
// arbitrarily deep on untrusted input.
func TestDecodeJSONDepthLimit(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantErrSubstring string
	}{
		{"within limit decodes", nestedListJSON(100), ""},
		{
			"parser rejects nesting before building the full tree",
			nestedListJSON(MaxDecodeNestingDepth() * 2),
			"PlutusData JSON tree nesting exceeds max depth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeJSON([]byte(tt.input))
			if tt.wantErrSubstring == "" {
				if err != nil {
					t.Fatalf("expected success, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrSubstring) {
				t.Fatalf(
					"expected error containing %q, got: %v",
					tt.wantErrSubstring,
					err,
				)
			}
		})
	}
}

func TestDecodeJSONAllowsNestingWithinPlutusDataLimit(t *testing.T) {
	// Each detailed-schema list level adds two JSON container levels. This
	// stays below the PlutusData nesting limit while exceeding the standard
	// library token decoder's independent nesting cap.
	const depth = 5_100
	if _, err := DecodeJSON([]byte(nestedListJSON(depth))); err != nil {
		t.Fatalf("DecodeJSON rejected valid nesting at depth %d: %v", depth, err)
	}
}

func TestParseDeepPlutusDataJSONTreePreservesJSONValues(t *testing.T) {
	const depth = 10_100
	input := []byte(strings.Repeat("[", depth) +
		`{"key\u002dname":"value\"escaped","true":true,` +
		`"false":false,"null":null,"number":-10.25e+3,` +
		`"empty_array":[],"empty_object":{}}` +
		strings.Repeat("]", depth))

	// parsePlutusDataJSONTree reaches the fallback only on toolchains whose
	// encoding/json caps nesting below this depth, so call the fallback
	// directly as well.
	parsers := map[string]func([]byte, int) (*jsonTree, error){
		"parsePlutusDataJSONTree":     parsePlutusDataJSONTree,
		"parseDeepPlutusDataJSONTree": parseDeepPlutusDataJSONTree,
	}
	for name, parse := range parsers {
		t.Run(name, func(t *testing.T) {
			tree, err := parse(input, MaxDecodeNestingDepth())
			if err != nil {
				t.Fatalf("rejected deep JSON: %v", err)
			}
			root := *tree.node(tree.root)
			for range depth {
				if root.kind != jsonKindArray || root.count != 1 {
					t.Fatalf("expected one-item array at each level, got kind %v", root.kind)
				}
				root = tree.children(root)[0]
			}
			if root.kind != jsonKindObject {
				t.Fatalf("innermost value kind = %v, want object", root.kind)
			}
			wantKinds := map[string]jsonValueKind{
				"key-name":     jsonKindString,
				"true":         jsonKindBool,
				"false":        jsonKindBool,
				"null":         jsonKindNull,
				"number":       jsonKindNumber,
				"empty_array":  jsonKindArray,
				"empty_object": jsonKindObject,
			}
			if int(root.count) != len(wantKinds) {
				t.Fatalf("innermost entry count = %d, want %d", root.count, len(wantKinds))
			}
			for _, entry := range tree.children(root) {
				name := tree.keyName(entry)
				want, ok := wantKinds[name]
				if !ok {
					t.Errorf("unexpected decoded key %q", name)
					continue
				}
				if entry.kind != want {
					t.Errorf("%s value kind = %v, want %v", name, entry.kind, want)
				}
				delete(wantKinds, name)
			}
			if len(wantKinds) != 0 {
				t.Errorf("missing decoded keys: %v", wantKinds)
			}
		})
	}
}

func TestDecodeJSONMapDepthBoundary(t *testing.T) {
	t.Run("exactly at PlutusData limit decodes", func(t *testing.T) {
		if _, err := DecodeJSON([]byte(nestedMapKeyJSON(MaxDecodeNestingDepth() - 1))); err != nil {
			t.Fatalf("expected success at depth limit, got: %v", err)
		}
	})
	t.Run(
		"one past PlutusData limit preserves semantic error",
		func(t *testing.T) {
			_, err := DecodeJSON(
				[]byte(nestedMapKeyJSON(MaxDecodeNestingDepth())),
			)
			if err == nil {
				t.Fatal("expected depth error, got nil")
			}
			want := fmt.Sprintf(
				"PlutusData JSON nesting exceeds max depth %d",
				MaxDecodeNestingDepth(),
			)
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("expected error containing %q, got: %v", want, err)
			}
		},
	)
}

func TestDecodeJSONMapPairRejectsUnknownFieldBeforeValueDecode(t *testing.T) {
	input := `{"map":[{"k":{"int":1},"junk":` +
		nestedListJSON(100) +
		`,"v":{"int":2}}]}`

	_, err := DecodeJSON([]byte(input))
	if err == nil {
		t.Fatal("expected unknown Map pair field error, got nil")
	}
	if !strings.Contains(err.Error(), `unexpected Map pair 0 field "junk"`) {
		t.Fatalf("expected unknown Map pair field error, got: %v", err)
	}
}

// TestDecodeJSONParserNodeAllowance verifies parser accounting leaves room
// for JSON structural nodes in an otherwise valid datum below the semantic
// PlutusData node limit.
func TestDecodeJSONParserNodeAllowance(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 500K-item document; skipped in -short mode")
	}

	_, err := DecodeJSON([]byte(wideFlatJSON(MaxDecodeNodes / 2)))
	if err != nil {
		t.Fatalf(
			"DecodeJSON rejected a semantically bounded flat list: %v",
			err,
		)
	}
}

func TestDecodeJSONParserNodeAllowanceRejectsValidConstrList(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a ~1M-item document; skipped in -short mode")
	}

	if _, err := DecodeJSON([]byte(wideEmptyConstrListJSON(MaxDecodeNodes - 10))); err != nil {
		t.Fatalf(
			"DecodeJSON rejected a semantically bounded Constr list: %v",
			err,
		)
	}
}

// TestDecodeJSONNodeLimit verifies the JSON parser enforces its node-count cap
// before it allocates a complete tree for a datum exceeding the semantic cap.
func TestDecodeJSONNodeLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a >2.5M-node document; skipped in -short mode")
	}

	tests := []struct {
		name             string
		input            string
		wantErrSubstring string
	}{
		{
			"parser rejects before building the full tree",
			wideFlatJSON(maxJSONParseNodes / 2),
			"PlutusData JSON tree exceeds max node count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeJSON([]byte(tt.input))
			if tt.wantErrSubstring == "" {
				if err != nil {
					t.Fatalf("expected success, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErrSubstring) {
				t.Fatalf(
					"expected error containing %q, got: %v",
					tt.wantErrSubstring,
					err,
				)
			}
		})
	}
}

func FuzzDecodeJSON(f *testing.F) {
	for _, input := range []string{
		`{"int":0}`,
		`{"list":[{"int":1},{"bytes":"ab"}]}`,
		`{"map":[{"k":{"int":1},"v":{"int":2}}]}`,
		`{"constructor":0,"fields":[{"int":1}]}`,
		`{"int":`,
		nestedListJSON(MaxDecodeNestingDepth()),
		nestedListJSON(maxJSONParseNestingDepth(MaxDecodeNestingDepth())),
	} {
		f.Add([]byte(input))
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > 16*1024 {
			t.Skip()
		}
		_, _ = DecodeJSON(input)
	})
}

// concreteJSONShapes wraps two child documents in each container the exported
// List, Map and Constr JSON decoders accept.
var concreteJSONShapes = []struct {
	name   string
	wrap   func(a, b string) string
	wide   func(items int) string
	decode func(doc []byte) error
}{
	{
		"List",
		func(a, b string) string { return `{"list":[` + a + `,` + b + `]}` },
		wideFlatJSON,
		func(doc []byte) error { return new(List).UnmarshalJSON(doc) },
	},
	{
		"Constr",
		func(a, b string) string { return `{"constructor":0,"fields":[` + a + `,` + b + `]}` },
		func(items int) string {
			return `{"constructor":0,"fields":` + strings.TrimPrefix(wideFlatJSON(items), `{"list":`)
		},
		func(doc []byte) error { return new(Constr).UnmarshalJSON(doc) },
	},
	{
		"Map",
		func(a, b string) string { return `{"map":[{"k":` + a + `,"v":` + b + `}]}` },
		func(items int) string {
			pair := `{"k":{"int":1},"v":{"int":1}}`
			return `{"map":[` + strings.Repeat(pair+",", items/2-1) + pair + `]}`
		},
		func(doc []byte) error { return new(Map).UnmarshalJSON(doc) },
	},
}

// TestConcreteJSONDecodersShareDocumentNodeBudget verifies that the exported
// decoders count nodes across the whole document: each child below is within
// the node limit on its own, but together they exceed it.
func TestConcreteJSONDecodersShareDocumentNodeBudget(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds a ~1M-node document; skipped in -short mode")
	}

	child := wideFlatJSON(MaxDecodeNodes/2 + 1000)
	for _, shape := range concreteJSONShapes {
		doc := []byte(shape.wrap(child, child))
		if _, err := DecodeJSON(doc); err == nil ||
			!strings.Contains(err.Error(), "max node count") {
			t.Fatalf("%s: DecodeJSON error = %v, want max node count", shape.name, err)
		}
		err := shape.decode(doc)
		if err == nil || !strings.Contains(err.Error(), "max node count") {
			t.Errorf("%s decoder error = %v, want max node count", shape.name, err)
		}
	}
}

// TestConcreteJSONDecodersBoundParsedTree verifies that the exported decoders
// reject a document over the parser's node cap instead of materializing it.
func TestConcreteJSONDecodersBoundParsedTree(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("builds a >2.5M-node document; skipped in -short mode")
	}

	for _, shape := range concreteJSONShapes {
		err := shape.decode([]byte(shape.wide(maxJSONParseNodes / 2)))
		if err == nil || !strings.Contains(err.Error(), "tree exceeds max node count") {
			t.Errorf("%s decoder error = %v, want parser node cap error", shape.name, err)
		}
	}
}

// TestConcreteJSONDecodersMatchGenericDepthLimit verifies the exported
// decoders apply the same nesting limit as DecodeJSON.
func TestConcreteJSONDecodersMatchGenericDepthLimit(t *testing.T) {
	t.Parallel()
	deep := nestedListJSON(MaxDecodeNestingDepth() * 2)
	for _, shape := range concreteJSONShapes {
		err := shape.decode([]byte(shape.wrap(deep, `{"int":0}`)))
		if err == nil || !strings.Contains(err.Error(), "max depth") {
			t.Errorf("%s decoder error = %v, want max depth error", shape.name, err)
		}
	}
}

func TestConcreteJSONDecodersRejectOtherVariants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		doc  string
		dst  interface{ UnmarshalJSON([]byte) error }
		want string
	}{
		{"Integer", `{"bytes":"ab"}`, new(Integer), `missing "int" key`},
		{"ByteString", `{"int":1}`, new(ByteString), `missing "bytes" key`},
		{"List", `{"int":1}`, new(List), `missing "list" key`},
		{"Map", `{"int":1}`, new(Map), `missing "map" key`},
		{"Constr", `{"int":1}`, new(Constr), `missing "constructor" key`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.dst.UnmarshalJSON([]byte(tt.doc))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

// TestIntegerStringMatchesBigInt pins Integer.String and MarshalJSON to
// math/big's decimal rendering for boundary values.
func TestIntegerStringMatchesBigInt(t *testing.T) {
	pow := func(bits uint) *big.Int { return new(big.Int).Lsh(big.NewInt(1), bits) }
	tests := []struct {
		name string
		n    *big.Int
	}{
		{"zero", big.NewInt(0)},
		{"one", big.NewInt(1)},
		{"minus one", big.NewInt(-1)},
		{"max int64", big.NewInt(math.MaxInt64)},
		{"min int64", big.NewInt(math.MinInt64)},
		{"2^64", pow(64)},
		{"-2^64", new(big.Int).Neg(pow(64))},
		{"2^64-1", new(big.Int).Sub(pow(64), big.NewInt(1))},
		{"2^1000+7", new(big.Int).Add(pow(1000), big.NewInt(7))},
		{"-2^70000", new(big.Int).Neg(pow(70000))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := Integer{Inner: tt.n}
			dec := tt.n.String()
			if got, want := i.String(), "Integer("+dec+")"; got != want {
				t.Fatalf("String() = %.40s..., want %.40s...", got, want)
			}
			out, err := i.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			if got, want := string(out), `{"int":`+dec+`}`; got != want {
				t.Fatalf("MarshalJSON() = %.40s..., want %.40s...", got, want)
			}
		})
	}
}
