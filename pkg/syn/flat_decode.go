package syn

import (
	"errors"
	"fmt"
)

const DECODETERMTAG = 4

func Decode[T Binder](bytes []byte) (*Program[T], error) {
	d := newDecoder(bytes)

	major, merr := d.word()
	if merr != nil {
		return nil, merr
	}

	minor, err := d.word()
	if err != nil {
		return nil, err
	}

	patch, perr := d.word()
	if perr != nil {
		return nil, perr
	}

	terms, terr := DecodeTerm[T](d)
	if terr != nil {
		return nil, terr
	}

	program := &Program[T]{
		Version: [3]uint32{uint32(major), uint32(minor), uint32(patch)},
		Term:    *terms,
	}

	return program, nil
}

func DecodeTerm[T Binder](d *decoder) (*Term[T], error) {
	tag, e := d.decodeTermTag()
	if e != nil {
		return nil, e
	}

	var term Term[T]

	switch tag {
	case 0:
		// TODO BinderVarDecode
		term = Var[T]{}
	case 1:
		t, err := DecodeTerm[T](d)
		if err != nil {
			return nil, err
		}

		term = Delay[T]{
			Term: *t,
		}
	case 2:
		// TODO BinderParameterDecode
		t, err := DecodeTerm[T](d)
		if err != nil {
			return nil, err
		}

		term = Lambda[T]{
			Body: *t,
		}
	case 3:
		function, err := DecodeTerm[T](d)
		if err != nil {
			return nil, err
		}

		argument, aerr := DecodeTerm[T](d)
		if aerr != nil {
			return nil, aerr
		}

		term = Apply[T]{
			Function: *function,
			Argument: *argument,
		}
	case 4:
		panic("TODO")
	case 5:
		t, err := DecodeTerm[T](d)
		if err != nil {
			return nil, err
		}

		term = Force[T]{
			Term: *t,
		}
	case 6:
		term = Error{}
	case 7:
		panic("TODO")
	case 8:
		panic("TODO")
	case 9:
		panic("TODO")
	default:
		return nil, fmt.Errorf("Invalid term tag: %d", tag)
	}

	return &term, nil
}

type decoder struct {
	buffer   []byte
	usedBits int64
	pos      int
}

func newDecoder(bytes []byte) *decoder {
	return &decoder{
		buffer:   bytes,
		usedBits: 0,
		pos:      0,
	}
}

func (d *decoder) decodeTermTag() (byte, error) {
	return d.bits8(DECODETERMTAG)
}

// / Decode a word of any size.
// / This is byte alignment agnostic.
// / First we decode the next 8 bits of the buffer.
// / We take the 7 least significant bits as the 7 least significant bits of
// / the current unsigned integer. If the most significant bit of the 8
// / bits is 1 then we take the next 8 and repeat the process above,
// / filling in the next 7 least significant bits of the unsigned integer and
// / so on. If the most significant bit was instead 0 we stop decoding
// / any more bits.
func (d *decoder) word() (uint, error) {

	var finalWord uint
	shl := 0
	for {
		word8, err := d.bits8(8)
		if err != nil {
			return 0, err
		}
		word7 := word8 & 127
		finalWord |= uint(word7) << shl
		shl += 7
		leadingBit := word8 & 128
		if leadingBit == 0 {
			break
		}
	}
	return finalWord, nil
}

// / Decode up to 8 bits.
// / This is byte alignment agnostic.
// / If num_bits is greater than the 8 we throw an IncorrectNumBits error.
// / First we decode the next num_bits of bits in the buffer.
// / If there are less unused bits in the current byte in the buffer than
// / num_bits, then we decode the remaining bits from the most
// / significant bits in the next byte in the buffer. Otherwise we decode
// / the unused bits from the current byte. Returns the decoded value up
// / to a byte in size.
func (d *decoder) bits8(numBits byte) (byte, error) {
	if numBits > 8 {
		return 0, errors.New("IncorrectNumBits")
	}

	err := d.ensureBits(uint(numBits))
	if err != nil {
		return 0, err
	}

	unusedBits := 8 - d.usedBits

	leadingZeros := 8 - numBits

	r := (d.buffer[d.pos] << byte(d.usedBits)) >> leadingZeros

	var x byte

	if numBits > byte(unusedBits) {
		x = r | (d.buffer[d.pos+1] >> (byte(unusedBits) + leadingZeros))
	} else {
		x = r
	}

	d.dropBits(uint(numBits))

	return x, nil

}

func (d *decoder) dropBits(numBits uint) {
	allUsedBits := numBits + uint(d.usedBits)
	d.usedBits = int64(allUsedBits) % 8
	d.pos += int(allUsedBits) / 8
}

// / Ensures the buffer has the required bits passed in by required_bits.
// / Throws a NotEnoughBits error if there are less bits remaining in the
// / buffer than required_bits.
func (d *decoder) ensureBits(requiredBits uint) error {
	if int(requiredBits) > (len(d.buffer)-d.pos)*8-int(d.usedBits) {
		return fmt.Errorf("NotEnoughBits(%d)", requiredBits)
	} else {
		return nil
	}
}
