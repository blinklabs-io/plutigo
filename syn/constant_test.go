package syn

import (
	"math/big"
	"testing"

	"github.com/blinklabs-io/plutigo/data"
	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

func TestIntegerTyp(t *testing.T) {
	integer := Integer{Inner: big.NewInt(42)}

	typ := integer.Typ()

	if _, ok := typ.(*TInteger); !ok {
		t.Errorf("Expected TInteger, got %T", typ)
	}
}

func TestIntegerCachedMetadata(t *testing.T) {
	integer := newInteger(big.NewInt(42))

	got, ok := integer.CachedInt64()
	if !ok || got != 42 {
		t.Fatalf("CachedInt64() = (%d, %t), want (42, true)", got, ok)
	}
	if got := integer.ExMemWords(); got != 1 {
		t.Fatalf("ExMemWords() = %d, want 1", got)
	}
}

func TestIntegerSetInnerRefreshesMetadata(t *testing.T) {
	integer := &Integer{}
	integer.SetInner(new(big.Int).Lsh(big.NewInt(1), 130))

	if got := integer.ExMemWords(); got != 3 {
		t.Fatalf("ExMemWords() = %d, want 3", got)
	}
	if _, ok := integer.CachedInt64(); ok {
		t.Fatal("CachedInt64() unexpectedly reported a large integer as int64")
	}

	integer.SetInner(big.NewInt(-7))
	got, ok := integer.CachedInt64()
	if !ok || got != -7 {
		t.Fatalf("CachedInt64() after reset = (%d, %t), want (-7, true)", got, ok)
	}
}

func TestIntegerExMemWordsNilReceiver(t *testing.T) {
	var integer *Integer

	if got := integer.ExMemWords(); got != 1 {
		t.Fatalf("ExMemWords() on nil receiver = %d, want 1", got)
	}
}

func TestByteStringTyp(t *testing.T) {
	bs := ByteString{Inner: []byte{1, 2, 3}}

	typ := bs.Typ()

	if _, ok := typ.(*TByteString); !ok {
		t.Errorf("Expected TByteString, got %T", typ)
	}
}

func TestStringTyp(t *testing.T) {
	str := String{Inner: "hello"}

	typ := str.Typ()

	if _, ok := typ.(*TString); !ok {
		t.Errorf("Expected TString, got %T", typ)
	}
}

func TestUnitTyp(t *testing.T) {
	unit := Unit{}

	typ := unit.Typ()

	if _, ok := typ.(*TUnit); !ok {
		t.Errorf("Expected TUnit, got %T", typ)
	}
}

func TestBoolTyp(t *testing.T) {
	boolConst := Bool{Inner: true}

	typ := boolConst.Typ()

	if _, ok := typ.(*TBool); !ok {
		t.Errorf("Expected TBool, got %T", typ)
	}
}

func TestProtoListTyp(t *testing.T) {
	elementType := &TInteger{}
	protoList := ProtoList{
		LTyp: elementType,
		List: []IConstant{
			&Integer{Inner: big.NewInt(1)},
			&Integer{Inner: big.NewInt(2)},
		},
	}

	typ := protoList.Typ()

	listTyp, ok := typ.(*TList)
	if !ok {
		t.Errorf("Expected TList, got %T", typ)
		return
	}

	if listTyp.Typ != elementType {
		t.Errorf("Expected element type %T, got %T", elementType, listTyp.Typ)
	}
}

func TestProtoPairTyp(t *testing.T) {
	fstType := &TInteger{}
	sndType := &TBool{}
	protoPair := ProtoPair{
		FstType: fstType,
		SndType: sndType,
		First:   &Integer{Inner: big.NewInt(42)},
		Second:  &Bool{Inner: true},
	}

	typ := protoPair.Typ()

	pairTyp, ok := typ.(*TPair)
	if !ok {
		t.Errorf("Expected TPair, got %T", typ)
		return
	}

	if pairTyp.First != fstType {
		t.Errorf("Expected first type %T, got %T", fstType, pairTyp.First)
	}

	if pairTyp.Second != sndType {
		t.Errorf("Expected second type %T, got %T", sndType, pairTyp.Second)
	}
}

func TestDataTyp(t *testing.T) {
	dataConst := Data{
		Inner: data.NewInteger(big.NewInt(42)),
	} // Simple integer data for testing

	typ := dataConst.Typ()

	if _, ok := typ.(*TData); !ok {
		t.Errorf("Expected TData, got %T", typ)
	}
}

func TestBls12_381G1ElementTyp(t *testing.T) {
	g1 := Bls12_381G1Element{
		Inner: &bls.G1Jac{},
	} // Empty G1 element for testing

	typ := g1.Typ()

	if _, ok := typ.(*TBls12_381G1Element); !ok {
		t.Errorf("Expected TBls12_381G1Element, got %T", typ)
	}
}

func TestBls12_381G2ElementTyp(t *testing.T) {
	g2 := Bls12_381G2Element{
		Inner: &bls.G2Jac{},
	} // Empty G2 element for testing

	typ := g2.Typ()

	if _, ok := typ.(*TBls12_381G2Element); !ok {
		t.Errorf("Expected TBls12_381G2Element, got %T", typ)
	}
}

func TestBls12_381MlResultTyp(t *testing.T) {
	ml := Bls12_381MlResult{Inner: &bls.GT{}} // Empty GT element for testing

	typ := ml.Typ()

	if _, ok := typ.(*TBls12_381MlResult); !ok {
		t.Errorf("Expected TBls12_381MlResult, got %T", typ)
	}
}

// Test that all constant types implement IConstant interface
func TestIConstantInterfaceCompliance(t *testing.T) {
	var _ IConstant = Integer{}
	var _ IConstant = ByteString{}
	var _ IConstant = String{}
	var _ IConstant = Unit{}
	var _ IConstant = Bool{}
	var _ IConstant = ProtoList{}
	var _ IConstant = ProtoPair{}
	var _ IConstant = Data{}
	var _ IConstant = Bls12_381G1Element{}
	var _ IConstant = Bls12_381G2Element{}
	var _ IConstant = Bls12_381MlResult{}
}
