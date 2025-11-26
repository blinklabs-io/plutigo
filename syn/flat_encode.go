package syn

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/blinklabs-io/plutigo/data"
)

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

		err := EncodeConstant(e, t.Con)
		if err != nil {
			return err
		}
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

		e.bits(BuiltinTagWidth, byte(t.DefaultFunction))
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

func EncodeList[T any](
	e *encoder,
	items []T,
	itemEncoder func(*encoder, T) error,
) error {
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
	err := e.safeEncodeBits(TermTagWidth, tag)
	if err != nil {
		return err
	}

	return nil
}

func (e *encoder) safeEncodeBits(numBits byte, val byte) error {
	if 2<<numBits <= val {
		return fmt.Errorf(
			"overflow detected, cannot fit %d in %d bits",
			val,
			numBits,
		)
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
func (e *encoder) bits(numBits byte, val byte) *encoder {
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
		e.usedBits += int64(numBits)
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

// Encode a string.
// Convert to byte array and then use byte array encoding.
// Uses filler to byte align the buffer, then writes byte array length up
// to 255. Following that it writes the next 255 bytes from the array.
// After reaching the end of the buffer we write a 0 byte. Only write 0
// byte if the byte array is empty.
func (e *encoder) utf8(s string) error {
	return e.bytes([]byte(s))
}

// Encode a byte array.
// Uses filler to byte align the buffer, then writes byte array length up
// to 255. Following that it writes the next 255 bytes from the array.
// We repeat writing length up to 255 and the next 255 bytes until we reach
// the end of the byte array. After reaching the end of the byte array
// we write a 0 byte. Only write 0 byte if the byte array is empty.
func (e *encoder) bytes(x []byte) error {
	// use filler to write current buffer so bits used gets reset
	return e.filler().byteArray(x)
}

// Encode a byte array in a byte aligned buffer. Throws exception if any
// bits for the current byte were used. Writes byte array length up to
// 255 Following that it writes the next 255 bytes from the array.
// We repeat writing length up to 255 and the next 255 bytes until we reach
// the end of the byte array. After reaching the end of the buffer we
// write a 0 byte. Only write 0 if the byte array is empty.
func (e *encoder) byteArray(arr []byte) error {
	if e.usedBits != 0 {
		return errors.New("BufferNotByteAligned")
	}

	e.writeBlk(arr)

	return nil
}

// Writes byte array length up to 255
// Following that it writes the next 255 bytes from the array.
// After reaching the end of the buffer we write a 0 byte. Only write 0 if
// the byte array is empty. This is byte alignment agnostic.
func (e *encoder) writeBlk(arr []byte) {
	for len(arr) > 0 {
		chunkLen := min(len(arr), 255)

		e.buffer = append(e.buffer, byte(chunkLen))
		e.buffer = append(e.buffer, arr[:chunkLen]...)

		arr = arr[chunkLen:]
	}

	e.buffer = append(e.buffer, 0)
}

// integer encodes an arbitrarily sized signed integer to the buffer.
// It is byte-alignment agnostic. First applies zigzag encoding to convert
// the signed integer to unsigned, then encodes it as a bigWord.
func (e *encoder) integer(i *big.Int) *encoder {
	e.bigWord(zigzag(i))

	return e
}

// bigWord encodes an unsigned integer to the buffer.
// It is byte-alignment agnostic. Encodes 7-bit chunks, setting the MSB to 1
// if more chunks follow, or 0 if it’s the last chunk.
func (e *encoder) bigWord(c *big.Int) *encoder {
	d := new(big.Int).Set(c)

	for {
		// temp = d % 128
		temp := new(big.Int)
		temp.Mod(d, big.NewInt(128))

		// Convert to uint8
		w := uint8(temp.Int64())

		// d >>= 7
		d.Rsh(d, 7)

		// If d != 0, set MSB (w |= 128)
		if d.Sign() != 0 {
			w |= 128
		}

		// Write 8 bits
		e.bits(8, w)

		// Stop if d == 0
		if d.Sign() == 0 {
			break
		}
	}

	return e
}

func EncodeConstant(e *encoder, constant IConstant) error {
	// Encode type tags
	if err := encodeConstantType(e, constant.Typ()); err != nil {
		return err
	}

	// Encode value based on type
	return encodeConstantValue(e, constant)
}

func encodeConstantType(e *encoder, typ Typ) error {
	switch t := typ.(type) {
	case *TInteger:
		e.one()
		e.bits(ConstTagWidth, IntegerTag)
		e.zero()
	case *TByteString:
		e.one()
		e.bits(ConstTagWidth, ByteStringTag)
		e.zero()
	case *TString:
		e.one()
		e.bits(ConstTagWidth, StringTag)
		e.zero()
	case *TUnit:
		e.one()
		e.bits(ConstTagWidth, UnitTag)
		e.zero()
	case *TBool:
		e.one()
		e.bits(ConstTagWidth, BoolTag)
		e.zero()
	case *TData:
		e.one()
		e.bits(ConstTagWidth, DataTag)
		e.zero()
	case *TList:
		e.bits(ConstTagWidth, ProtoListOneTag)
		e.bits(ConstTagWidth, ProtoListTwoTag)
		if err := encodeConstantType(e, t.Typ); err != nil {
			return err
		}
	case *TPair:
		e.bits(ConstTagWidth, ProtoPairOneTag)
		e.bits(ConstTagWidth, ProtoPairTwoTag)
		e.bits(ConstTagWidth, ProtoPairThreeTag)
		if err := encodeConstantType(e, t.First); err != nil {
			return err
		}
		if err := encodeConstantType(e, t.Second); err != nil {
			return err
		}
	default:
		return errors.New("unsupported constant type")
	}
	return nil
}

func encodeConstantValue(e *encoder, constant IConstant) error {
	switch c := constant.(type) {
	case *Integer:
		e.integer(c.Inner)
	case *ByteString:
		return e.bytes(c.Inner)
	case *String:
		return e.utf8(c.Inner)
	case *Unit:
		// Unit has no value to encode
	case *Bool:
		if c.Inner {
			e.one()
		} else {
			e.zero()
		}
	case *ProtoList:
		return EncodeList(e, c.List, func(e *encoder, item IConstant) error {
			return encodeConstantValue(e, item)
		})
	case *ProtoPair:
		if err := encodeConstantValue(e, c.First); err != nil {
			return err
		}
		return encodeConstantValue(e, c.Second)
	case *Data:
		cborBytes, err := data.Encode(c.Inner)
		if err != nil {
			return err
		}
		return e.bytes(cborBytes)
	default:
		return errors.New("unsupported constant value")
	}
	return nil
}
