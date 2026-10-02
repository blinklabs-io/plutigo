package syn

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/lang"
)

func encodeIntegerListProgram(t *testing.T, values []*big.Int) []byte {
	t.Helper()
	items := make([]IConstant, len(values))
	for i, v := range values {
		items[i] = &Integer{Inner: v}
	}
	encoded, err := Encode(&Program[DeBruijn]{
		Version: lang.LanguageVersionV1,
		Term: &Constant{Con: &ProtoList{
			LTyp: &TInteger{},
			List: items,
		}},
	})
	if err != nil {
		t.Fatalf("Encode() failed: %v", err)
	}
	return encoded
}

func decodedIntegers(t *testing.T, program *Program[DeBruijn]) []*Integer {
	t.Helper()
	list := program.Term.(*Constant).Con.(*ProtoList)
	ints := make([]*Integer, len(list.List))
	for i, item := range list.List {
		ints[i] = item.(*Integer)
	}
	return ints
}

func TestDecodeIntegerConstantsShareNoMagnitude(t *testing.T) {
	two64 := new(big.Int).Lsh(big.NewInt(1), 64)
	values := []*big.Int{
		big.NewInt(0),
		big.NewInt(1),
		big.NewInt(-1),
		big.NewInt(math.MaxInt64),
		big.NewInt(math.MinInt64),
		new(big.Int).Add(big.NewInt(math.MaxInt64), big.NewInt(1)),
		new(big.Int).Sub(big.NewInt(math.MinInt64), big.NewInt(1)),
		new(big.Int).Sub(two64, big.NewInt(1)),
		new(big.Int).Set(two64),
		new(big.Int).Neg(two64),
		new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1)),
		new(big.Int).Add(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(3)),
		new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 200)),
		big.NewInt(42),
	}
	// Adding 1 grows a magnitude into its own spare word in place, which is
	// where an overlapping slab slice would write into a neighbour; adding
	// 2^300 reallocates every magnitude and exercises capacity capping.
	bumps := []*big.Int{big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 300)}
	for _, bump := range bumps {
		for _, decode := range []struct {
			name string
			fn   func([]byte) (*Program[DeBruijn], error)
		}{
			{"fresh", Decode[DeBruijn]},
			{"reused", NewDeBruijnDecoder().Decode},
		} {
			t.Run(fmt.Sprintf("%s/bump-%d-bits", decode.name, bump.BitLen()), func(t *testing.T) {
				program, err := decode.fn(encodeIntegerListProgram(t, values))
				if err != nil {
					t.Fatalf("decode failed: %v", err)
				}
				ints := decodedIntegers(t, program)
				for i, got := range ints {
					if got.Inner.Cmp(values[i]) != 0 {
						t.Fatalf("integer %d = %s, want %s", i, got.Inner, values[i])
					}
					want, ok := values[i].Int64(), values[i].IsInt64()
					gotInt, gotOK := got.CachedInt64()
					if gotOK != ok || (ok && gotInt != want) {
						t.Fatalf("integer %d CachedInt64 = %d,%v, want %d,%v", i, gotInt, gotOK, want, ok)
					}
				}
				// Magnitudes come from one arena slab; an in-place update of one
				// integer must not reach its neighbours.
				for i := range ints {
					ints[i].Inner.Add(ints[i].Inner, bump)
					for j, other := range ints {
						want := values[j]
						if j <= i {
							want = new(big.Int).Add(values[j], bump)
						}
						if other.Inner.Cmp(want) != 0 {
							t.Fatalf("after bumping %d, integer %d = %s, want %s", i, j, other.Inner, want)
						}
					}
				}
			})
		}
	}
}

func TestDecodeIntegerConstantAllocationsDoNotScaleWithCount(t *testing.T) {
	values := make([]*big.Int, 256)
	for i := range values {
		v := new(big.Int).Lsh(big.NewInt(int64(i+1)), 100)
		if i%2 == 1 {
			v.Neg(v)
		}
		values[i] = v
	}
	encoded := encodeIntegerListProgram(t, values)
	allocs := testing.AllocsPerRun(20, func() {
		if _, err := Decode[DeBruijn](encoded); err != nil {
			t.Fatal(err)
		}
	})
	// Each integer used to allocate its accumulation words, a temporary
	// big.Int and a copied magnitude; arena chunks are logarithmic in count.
	if allocs > 64 {
		t.Fatalf("decoding %d large integers made %.0f allocations, want at most 64", len(values), allocs)
	}
}

var typSink Typ

func TestCompositeConstantTypesDoNotAllocate(t *testing.T) {
	pair := ProtoPair{FstType: &TData{}, SndType: &TData{}}
	list := ProtoList{LTyp: &TData{}}
	mapList := ProtoList{LTyp: &TPair{First: &TData{}, Second: &TData{}}}
	allocs := testing.AllocsPerRun(100, func() {
		typSink = pair.Typ()
		typSink = list.Typ()
		typSink = mapList.Typ()
	})
	if allocs != 0 {
		t.Fatalf("composite Typ() made %.0f allocations, want 0", allocs)
	}
	if !EqualType(mapList.Typ(), &TList{Typ: &TPair{First: &TData{}, Second: &TData{}}}) {
		t.Fatalf("interned map type %#v is not list (pair data data)", mapList.Typ())
	}
	if !EqualType(ProtoList{LTyp: &TInteger{}}.Typ(), &TList{Typ: &TInteger{}}) {
		t.Fatal("interned list integer type mismatch")
	}
}
