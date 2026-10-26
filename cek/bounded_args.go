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

package cek

import (
	"math/big"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/syn"
)

// From van Rossem (semantics variants D and E) the reference unlifts some
// builtin arguments into bounded types (PlutusCore.Default.Universe.Cardano):
// CInteger, an Integer in [-(2^262143), 2^262143-1], and CByteString, a byte
// string of at most 65536 bytes. An argument outside its bound fails the
// builtin application while unlifting, before the builtin is costed; results
// are never bounded. See ensurableInteger and ensurableByteString and the
// per-builtin denotations in PlutusCore.Default.Builtins.
const (
	boundedIntegerBits         = 262143
	boundedByteStringMaxLength = 65536
)

// boundedArgs lists, per builtin, the argument positions (bit i is the i-th
// argument) unlifted as CInteger and as CByteString.
type boundedArgs struct {
	integers    uint8
	byteStrings uint8
}

// boundedArgsVariantE holds the positions for semantics variant E (Plutus V3
// and later at van Rossem); variant D (V1/V2) differs only in
// consByteString, whose first argument is a CInteger there and a Word8 in E.
var boundedArgsVariantE = func() [builtin.TotalBuiltinCount]boundedArgs {
	var t [builtin.TotalBuiltinCount]boundedArgs
	for _, fn := range []builtin.DefaultFunction{
		builtin.AddInteger,
		builtin.SubtractInteger,
		builtin.MultiplyInteger,
		builtin.DivideInteger,
		builtin.QuotientInteger,
		builtin.RemainderInteger,
		builtin.ModInteger,
		builtin.LessThanInteger,
		builtin.LessThanEqualsInteger,
	} {
		t[fn].integers = 0b11
	}
	for _, fn := range []builtin.DefaultFunction{
		builtin.AppendByteString,
		builtin.EqualsByteString,
		builtin.LessThanByteString,
		builtin.LessThanEqualsByteString,
	} {
		t[fn].byteStrings = 0b11
	}
	for _, fn := range []builtin.DefaultFunction{
		builtin.AndByteString,
		builtin.OrByteString,
		builtin.XorByteString,
	} {
		t[fn].byteStrings = 0b110
	}
	for _, fn := range []builtin.DefaultFunction{
		builtin.IndexByteString,
		builtin.Sha2_256,
		builtin.Sha3_256,
		builtin.Blake2b_256,
		builtin.Keccak_256,
		builtin.Blake2b_224,
		builtin.Ripemd_160,
		builtin.DecodeUtf8,
		builtin.ComplementByteString,
		builtin.ReadBit,
		builtin.CountSetBits,
		builtin.FindFirstSetBit,
		builtin.Bls12_381_G1_HashToGroup,
		builtin.Bls12_381_G2_HashToGroup,
	} {
		t[fn].byteStrings = 0b1
	}
	for _, fn := range []builtin.DefaultFunction{
		builtin.ConsByteString,
		builtin.ByteStringToInteger,
		builtin.VerifyEd25519Signature,
		builtin.VerifySchnorrSecp256k1Signature,
	} {
		t[fn].byteStrings = 0b10
	}
	t[builtin.SliceByteString].byteStrings = 0b100
	return t
}()

var boundedArgsVariantD = func() [builtin.TotalBuiltinCount]boundedArgs {
	t := boundedArgsVariantE
	t[builtin.ConsByteString].integers = 0b1
	return t
}()

// boundedArgsFor returns the bounded argument positions of fn under the
// machine's semantics variant, or the zero value when none are bounded.
func (m *Machine[T]) boundedArgsFor(fn builtin.DefaultFunction) boundedArgs {
	if fn >= builtin.TotalBuiltinCount {
		return boundedArgs{}
	}
	switch m.semantics {
	case SemanticsVariantD:
		return m.withoutIntegerBounds(boundedArgsVariantD[fn])
	case SemanticsVariantE:
		return m.withoutIntegerBounds(boundedArgsVariantE[fn])
	default:
		return boundedArgs{}
	}
}

// withoutIntegerBounds drops the integer bounds from protocol 12 on. The
// reference moves Dijkstra to semantics variants F and G, whose integer
// predicate is false while the byte string predicate stays true.
func (m *Machine[T]) withoutIntegerBounds(b boundedArgs) boundedArgs {
	if m.protoMajor >= 12 {
		b.integers = 0
	}
	return b
}

// checkBoundedArg fails when the pos-th argument of fn is an integer or byte
// string constant outside the bound the reference unlifts it into. Other
// argument kinds are left to the builtin, which reports the type mismatch.
func checkBoundedArg[T syn.Eval](
	fn builtin.DefaultFunction,
	bounds boundedArgs,
	pos int,
	value Value[T],
) error {
	bit := uint8(1) << pos
	if (bounds.integers|bounds.byteStrings)&bit == 0 {
		return nil
	}
	c, ok := value.(*Constant)
	if !ok || c == nil {
		return nil
	}
	switch v := c.Constant.(type) {
	case *syn.Integer:
		if bounds.integers&bit != 0 && v != nil && v.Inner != nil &&
			!boundedIntegerInRange(v) {
			return &BuiltinError{
				Code:    ErrCodeOverflow,
				Builtin: fn.String(),
				Message: "Integer out of bounds",
			}
		}
	case *syn.ByteString:
		if bounds.byteStrings&bit != 0 && v != nil &&
			len(v.Inner) > boundedByteStringMaxLength {
			return &BuiltinError{
				Code:    ErrCodeOverflow,
				Builtin: fn.String(),
				Message: "ByteString overflow",
			}
		}
	}
	return nil
}

// boundedIntegerInRange reports whether -(2^262143) <= v <= 2^262143-1.
func boundedIntegerInRange(v *syn.Integer) bool {
	if _, ok := v.CachedInt64(); ok {
		return true
	}
	return inSignedBitRange(v.Inner, boundedIntegerBits)
}

// inSignedBitRange reports whether -(2^n) <= x <= 2^n-1.
func inSignedBitRange(x *big.Int, n int) bool {
	bits := x.BitLen()
	if bits <= n {
		return true
	}
	// -(2^n) is the one in-range value of bit length n+1.
	return x.Sign() < 0 && bits == n+1 && x.TrailingZeroBits() == uint(n)
}

// checkBoundedArgs1/2/3 apply checkBoundedArg to the arguments of a
// saturated builtin application in order.
func (m *Machine[T]) checkBoundedArgs1(fn builtin.DefaultFunction, a Value[T]) error {
	bounds := m.boundedArgsFor(fn)
	if bounds == (boundedArgs{}) {
		return nil
	}
	return checkBoundedArg[T](fn, bounds, 0, a)
}

func (m *Machine[T]) checkBoundedArgs2(fn builtin.DefaultFunction, a, b Value[T]) error {
	bounds := m.boundedArgsFor(fn)
	if bounds == (boundedArgs{}) {
		return nil
	}
	if err := checkBoundedArg[T](fn, bounds, 0, a); err != nil {
		return err
	}
	return checkBoundedArg[T](fn, bounds, 1, b)
}

func (m *Machine[T]) checkBoundedArgs3(fn builtin.DefaultFunction, a, b, c Value[T]) error {
	bounds := m.boundedArgsFor(fn)
	if bounds == (boundedArgs{}) {
		return nil
	}
	if err := checkBoundedArg[T](fn, bounds, 0, a); err != nil {
		return err
	}
	if err := checkBoundedArg[T](fn, bounds, 1, b); err != nil {
		return err
	}
	return checkBoundedArg[T](fn, bounds, 2, c)
}

func (m *Machine[T]) checkBoundedBuiltinArgs(b *Builtin[T]) error {
	bounds := m.boundedArgsFor(b.Func)
	if bounds == (boundedArgs{}) || b.Args == nil {
		return nil
	}
	pos := 0
	for arg := range b.Args.Iter() {
		if err := checkBoundedArg[T](b.Func, bounds, pos, arg); err != nil {
			return err
		}
		pos++
	}
	return nil
}
