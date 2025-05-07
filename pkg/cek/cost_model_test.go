package cek

import (
	"math/big"
	"testing"
)

func TestBigIntExMem_0(t *testing.T) {
	x := big.NewInt(0)

	y := bigIntExMem(x)()

	if y != ExMem(1) {
		t.Error("HOW???")
	}

	println(y)
}

func TestBigIntExMem_63(t *testing.T) {
	x := big.NewInt(0)

	y := bigIntExMem(x)()

	if y != ExMem(1) {
		t.Error("HOW???")
	}

	println(y)
}

func TestBigIntExMem_64(t *testing.T) {
	x := big.NewInt(0)

	y := bigIntExMem(x)()

	if y != ExMem(1) {
		t.Error("HOW???")
	}

	println(y)
}

func TestBigIntExMem_65(t *testing.T) {
	x := big.NewInt(0)

	y := bigIntExMem(x)()

	if y != ExMem(1) {
		t.Error("HOW???")
	}

	println(y)
}

func TestBigIntExMem_128(t *testing.T) {
	x := big.NewInt(128)

	y := bigIntExMem(x)()

	if y != ExMem(2) {
		t.Error("HOW???")
	}

	println(y)
}

func TestBigIntExMem_1024(t *testing.T) {
	x := big.NewInt(1024)

	y := bigIntExMem(x)()

	if y != ExMem(5) {
		t.Error("HOW???")
	}

	println(y)
}

func TestBigIntExMem_1025(t *testing.T) {
	x := big.NewInt(1025)

	y := bigIntExMem(x)()

	if y != ExMem(5) {
		t.Error("HOW???")
	}

	println(y)
}

func TestBigIntExMem_neg_1025(t *testing.T) {
	x := big.NewInt(-1025)

	y := bigIntExMem(x)()

	if y != ExMem(5) {
		t.Error("HOW???")
	}

	println(y)
}
