package syn

import "errors"

func Decode[T Binder](bytes []byte) (*Program[T], error) {
	d := newDecoder()

	return nil, nil
}

type decoder struct {
	buffer   []byte
	usedBits int64
	pos      int
}

func newDecoder() *decoder {
	return &decoder{}
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
func (d *decoder) bits8(numBits uint8) (uint8, error) {
	if numBits > 8 {
		return 0, errors.New("IncorrectNumBits")
	}

	// self.ensure_bits(num_bits)?;

	// let unused_bits = 8 - self.used_bits as usize;
	// let leading_zeroes = 8 - num_bits;
	// let r = (self.buffer[self.pos] << self.used_bits as usize) >> leading_zeroes;

	// let x = if num_bits > unused_bits {
	//     r | (self.buffer[self.pos + 1] >> (unused_bits + leading_zeroes))
	// } else {
	//     r
	// };

	// self.drop_bits(num_bits);

	// Ok(x)
	//
	panic("TODO")

}
