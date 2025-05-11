package syn

import (
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/blinklabs-io/plutigo/pkg/builtin"
)

func Decode[T Binder](bytes []byte) (*Program[T], error) {
	d := newDecoder(bytes)

	major, err := d.word()
	if err != nil {
		return nil, err
	}

	minor, err := d.word()
	if err != nil {
		return nil, err
	}

	patch, err := d.word()
	if err != nil {
		return nil, err
	}

	terms, err := DecodeTerm[T](d)
	if err != nil {
		return nil, err
	}

	version := [3]uint32{uint32(major), uint32(minor), uint32(patch)}

	program := &Program[T]{
		Version: version,
		Term:    terms,
	}

	if err := d.filler(); err != nil {
		return nil, err
	}

	return program, nil
}

func DecodeTerm[T Binder](d *decoder) (Term[T], error) {
	tag, e := d.bits8(TermTagWidth)
	if e != nil {
		return nil, e
	}

	var term Term[T]

	switch tag {
	case VarTag:
		var name T

		binder, err := name.VarDecode(d) // Call on zero-value t
		if err != nil {
			return nil, err
		}

		name, ok := binder.(T) // Assign returned Binder to t
		if !ok {
			return nil, fmt.Errorf("VarDecode returned wrong type: got %T, want %T", binder, name)
		}

		term = &Var[T]{Name: name}
	case DelayTag:
		t, err := DecodeTerm[T](d)
		if err != nil {
			return nil, err
		}

		term = &Delay[T]{
			Term: t,
		}
	case LambdaTag:
		var name T
		binder, err := name.ParameterDecode(d) // Call on zero-value t
		if err != nil {
			return nil, err
		}

		name, ok := binder.(T) // Assign returned Binder to t
		if !ok {
			return nil, fmt.Errorf("ParameterDecode returned wrong type: got %T, want %T", binder, name)
		}

		t, err := DecodeTerm[T](d)
		if err != nil {
			return nil, err
		}

		term = Lambda[T]{
			ParameterName: name,
			Body:          t,
		}
	case ApplyTag:
		function, err := DecodeTerm[T](d)
		if err != nil {
			return nil, err
		}

		argument, err := DecodeTerm[T](d)
		if err != nil {
			return nil, err
		}

		term = &Apply[T]{
			Function: function,
			Argument: argument,
		}
	case ConstantTag:
		panic("decode constant")
	case ForceTag:
		t, err := DecodeTerm[T](d)
		if err != nil {
			return nil, err
		}

		term = &Force[T]{
			Term: t,
		}
	case ErrorTag:
		term = &Error{}
	case BuiltinTag:
		builtinTag, err := d.bits8(BuiltinTagWidth)
		if err != nil {
			return nil, err
		}

		fn, err := builtin.FromByte(builtinTag)
		if err != nil {
			return nil, err
		}

		term = &Builtin{fn}
	case ConstrTag:
		panic("decode constr")
	case CaseTag:
		panic("decode case")
	default:
		return nil, fmt.Errorf("Invalid term tag: %d", tag)
	}

	return term, nil
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

// Decodes a filler of max one byte size.
// Decodes bits until we hit a bit that is 1.
// Expects that the 1 is at the end of the current byte in the buffer.
func (d *decoder) filler() error {
	for {
		ok, err := d.zero()
		if err != nil {
			return err
		}

		if !ok {
			break
		}
	}

	return nil
}

// Decode the next bit in the buffer.
// If the bit was 0 then return true.
// Otherwise return false.
// Throws EndOfBuffer error if used at the end of the array.
func (d *decoder) zero() (bool, error) {
	currentBit, err := d.bit()
	if err != nil {
		return false, err
	}

	return !currentBit, nil
}

// Decode the next bit in the buffer.
// If the bit was 1 then return true.
// Otherwise return false.
// Throws EndOfBuffer error if used at the end of the array.
func (d *decoder) bit() (bool, error) {
	if d.pos >= len(d.buffer) {
		return false, errors.New("end of buffer")
	}

	b := d.buffer[d.pos]&(128>>d.usedBits) > 0

	d.incrementBufferByBit()

	return b, nil
}

// Decode a word of any size.
// This is byte alignment agnostic.
// First we decode the next 8 bits of the buffer.
// We take the 7 least significant bits as the 7 least significant bits of
// the current unsigned integer. If the most significant bit of the 8
// bits is 1 then we take the next 8 and repeat the process above,
// filling in the next 7 least significant bits of the unsigned integer and
// so on. If the most significant bit was instead 0 we stop decoding
// any more bits.
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

// Decode up to 8 bits.
// This is byte alignment agnostic.
// If num_bits is greater than the 8 we throw an IncorrectNumBits error.
// First we decode the next num_bits of bits in the buffer.
// If there are less unused bits in the current byte in the buffer than
// num_bits, then we decode the remaining bits from the most
// significant bits in the next byte in the buffer. Otherwise we decode
// the unused bits from the current byte. Returns the decoded value up
// to a byte in size.
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

// Ensures the buffer has the required bits passed in by required_bits.
// Throws a NotEnoughBits error if there are less bits remaining in the
// buffer than required_bits.
func (d *decoder) ensureBits(requiredBits uint) error {
	if int(requiredBits) > (len(d.buffer)-d.pos)*8-int(d.usedBits) {
		return fmt.Errorf("NotEnoughBits(%d)", requiredBits)
	} else {
		return nil
	}
}

// Decode a string.
// Convert to byte array and then use byte array decoding.
// Decodes a filler to byte align the buffer,
// then decodes the next byte to get the array length up to a max of 255.
// We decode bytes equal to the array length to form the byte array.
// If the following byte for array length is not 0 we decode it and repeat
// above to continue decoding the byte array. We stop once we hit a
// byte array length of 0. If array length is 0 for first byte array
// length the we return a empty array.
func (d *decoder) utf8() (string, error) {
	b, err := d.bytes()
	if err != nil {
		return "", err
	}

	if !utf8.Valid(b) {
		return "", fmt.Errorf("bytes are not valid utf8 %v", b)
	}

	return string(b), nil
}

// Decode a byte array.
// Decodes a filler to byte align the buffer,
// then decodes the next byte to get the array length up to a max of 255.
// We decode bytes equal to the array length to form the byte array.
// If the following byte for array length is not 0 we decode it and repeat
// above to continue decoding the byte array. We stop once we hit a
// byte array length of 0. If array length is 0 for first byte array
// length the we return a empty array.
func (d *decoder) bytes() ([]byte, error) {
	if err := d.filler(); err != nil {
		return nil, err
	}

	return d.byteArray()
}

// Decode a byte array.
// Throws a BufferNotByteAligned error if the buffer is not byte aligned
// Decodes the next byte to get the array length up to a max of 255.
// We decode bytes equal to the array length to form the byte array.
// If the following byte for array length is not 0 we decode it and repeat
// above to continue decoding the byte array. We stop once we hit a
// byte array length of 0. If array length is 0 for first byte array
// length the we return a empty array.
func (d *decoder) byteArray() ([]byte, error) {
	if d.usedBits != 0 {
		return nil, errors.New("buffer not byte aligned")
	}

	if err := d.ensureBytes(1); err != nil {
		return nil, err
	}

	result := make([]byte, 0)
	blkLen := int(d.buffer[d.pos])
	d.pos++

	for blkLen != 0 {
		if err := d.ensureBytes(blkLen + 1); err != nil {
			return nil, err
		}

		decodedArray := d.buffer[d.pos : d.pos+blkLen]
		result = append(result, decodedArray...)

		d.pos += blkLen
		blkLen = int(d.buffer[d.pos])
		d.pos++
	}

	return result, nil
}

// Increment used bits by 1.
// If all 8 bits are used then increment buffer position by 1.
func (d *decoder) incrementBufferByBit() {
	if d.usedBits == 7 {
		d.pos += 1

		d.usedBits = 0
	} else {
		d.usedBits += 1
	}
}

// Ensures the buffer has the required bytes passed in by required_bytes.
// Throws a NotEnoughBytes error if there are less bytes remaining in the
// buffer than required_bytes.
func (d *decoder) ensureBytes(requiredBytes int) error {
	if requiredBytes > len(d.buffer)-d.pos {
		return fmt.Errorf("NotEnoughBytes(%d)", requiredBytes)
	} else {
		return nil
	}
}
