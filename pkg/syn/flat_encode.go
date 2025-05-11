package syn

import (
	"fmt"
)

const TERMTAGWIDTH = 4

func Encode[T Binder](program *Program[T]) ([]byte, error) {
	e := newEncoder()

	e.word(uint(program.Version[0])).
		word(uint(program.Version[1])).
		word(uint(program.Version[2]))

	if err := EncodeTerm[T](e, program.Term); err != nil {
		return nil, err
	}

	e.filler()

	return e.buffer, nil
}

func EncodeTerm[T Binder](e *encoder, term Term[T]) error {
	switch t := term.(type) {
	case *Var[T]:
		termError := e.encodeTermTag(0)
		if termError != nil {
			return termError
		}

		err := t.Name.VarEncode(e)
		if err != nil {
			return err
		}
	case *Delay[T]:
		termError := e.encodeTermTag(1)
		if termError != nil {
			return termError
		}

		err := EncodeTerm[T](e, t.Term)
		if err != nil {
			return err
		}
	case *Lambda[T]:
		termError := e.encodeTermTag(2)
		if termError != nil {
			return termError
		}

		err := t.ParameterName.ParameterEncode(e)
		if err != nil {
			return err
		}

		error := EncodeTerm[T](e, t.Body)
		if error != nil {
			return error
		}

	case *Apply[T]:
		termError := e.encodeTermTag(3)
		if termError != nil {
			return termError
		}

		err := EncodeTerm[T](e, t.Function)
		if err != nil {
			return err
		}

		error := EncodeTerm[T](e, t.Argument)
		if error != nil {
			return error
		}
	case *Constant:
		termError := e.encodeTermTag(4)
		if termError != nil {
			return termError
		}
		panic("TODO")
	case *Force[T]:
		termError := e.encodeTermTag(5)
		if termError != nil {
			return termError
		}

		err := EncodeTerm[T](e, t.Term)
		if err != nil {
			return err
		}
	case *Error:
		termError := e.encodeTermTag(6)
		if termError != nil {
			return termError
		}
	case *Builtin:
		termError := e.encodeTermTag(7)
		if termError != nil {
			return termError
		}
		panic("TODO")
	case *Constr[T]:
		termError := e.encodeTermTag(8)
		if termError != nil {
			return termError
		}

		e.word(t.Tag)

		err := EncodeList(e, t.Fields, EncodeTerm[T])
		if err != nil {
			return err
		}
	case *Case[T]:
		termError := e.encodeTermTag(9)
		if termError != nil {
			return termError
		}

		err := EncodeTerm[T](e, t.Constr)
		if err != nil {
			return err
		}

		error := EncodeList(e, t.Branches, EncodeTerm[T])
		if error != nil {
			return error
		}
	}

	return nil
}

func EncodeList[T any](e *encoder, items []T, itemEncoder func(*encoder, T) error) error {
	for _, item := range items {
		e.one()
		err := itemEncoder(e, item)
		if err != nil {
			return err
		}
	}

	e.zero()
	return nil
}

type encoder struct {
	buffer      []byte
	usedBits    int64
	currentByte byte
}

func newEncoder() *encoder {
	return &encoder{
		buffer:      []byte{},
		usedBits:    0,
		currentByte: 0,
	}
}

func (e *encoder) encodeTermTag(tag byte) error {
	err := e.safeEncodeBits(TERMTAGWIDTH, tag)

	if err != nil {
		return err
	}

	return nil

}

func (e *encoder) safeEncodeBits(numBits int64, val byte) error {
	if 2**&numBits <= int64(val) {
		return fmt.Errorf("Overflow detected, cannot fit %d in %d bits.", val, numBits)
	}

	e.bits(numBits, val)

	return nil
}

// Encode a unsigned integer of any size.
// This is byte alignment agnostic.
// We encode the 7 least significant bits of the unsigned byte. If the char
// value is greater than 127 we encode a leading 1 followed by
// repeating the above for the next 7 bits and so on.
func (e *encoder) word(c uint) *encoder {
	d := c

	for {
		w := uint8(d & 127)

		d >>= 7

		if d != 0 {
			w |= 128
		}

		e.bits(8, w)

		if d == 0 {
			break
		}
	}

	return e
}

// Encodes up to 8 bits of information and is byte alignment agnostic.
// Uses unused bits in the current byte to write out the passed in byte
// value. Overflows to the most significant digits of the next byte if
// number of bits to use is greater than unused bits. Expects that
// number of bits to use is greater than or equal to required bits by the
// value. The param num_bits is i64 to match unused_bits type.
func (e *encoder) bits(numBits int64, val byte) *encoder {
	if numBits == 1 && val == 0 {
		e.zero()
	} else if numBits == 1 && val == 1 {
		e.one()
	} else if numBits == 2 && val == 0 {
		e.zero().zero()
	} else if numBits == 2 && val == 1 {
		e.zero().one()
	} else if numBits == 2 && val == 2 {
		e.one().zero()
	} else if numBits == 2 && val == 3 {
		e.one().one()
	} else {
		e.usedBits += numBits
		unusedBits := 8 - e.usedBits
		if unusedBits == 0 {
			e.currentByte |= val

			e.nextWord()
		} else if unusedBits > 0 {
			e.currentByte |= val << byte(unusedBits)
		} else {
			used := -unusedBits

			e.currentByte |= val >> used

			e.nextWord()

			e.currentByte = val << (8 - used)

			e.usedBits = used
		}
	}

	return e
}

// A filler amount of end 0's followed by a 1 at the end of a byte.
// Used to byte align the buffer by padding out the rest of the byte.
func (e *encoder) filler() *encoder {
	e.currentByte |= 1
	e.nextWord()

	return e
}

// Write a 0 bit into the current byte.
// Write out to buffer if last used bit in the current byte.
func (e *encoder) zero() *encoder {
	if e.usedBits == 7 {
		e.nextWord()
	} else {
		e.usedBits += 1
	}

	return e
}

// Write a 1 bit into the current byte.
// Write out to buffer if last used bit in the current byte.
func (e *encoder) one() *encoder {
	if e.usedBits == 7 {
		e.currentByte |= 1
		e.nextWord()
	} else {
		e.currentByte |= 128 >> e.usedBits
		e.usedBits += 1
	}

	return e
}

// Write the current byte out to the buffer and begin next byte to write
// out. Add current byte to the buffer and set current byte and used
// bits to 0.
func (e *encoder) nextWord() *encoder {
	e.buffer = append(e.buffer, e.currentByte)

	e.currentByte = 0
	e.usedBits = 0

	return e
}
