package syn

func Encode[T Binder](program *Program[T]) ([]byte, error) {
	e := newEncoder()

	e.word(uint(program.Version[0])).
		word(uint(program.Version[1])).
		word(uint(program.Version[2]))

	return nil, nil
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
		e.zero()
		e.zero()
	} else if numBits == 2 && val == 1 {
		e.zero()
		e.one()
	} else if numBits == 2 && val == 2 {
		e.one()
		e.zero()
	} else if numBits == 2 && val == 3 {
		e.one()
		e.one()
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
func (e *encoder) zero() {
	if e.usedBits == 7 {
		e.nextWord()
	} else {
		e.usedBits += 1
	}
}

// Write a 1 bit into the current byte.
// Write out to buffer if last used bit in the current byte.
func (e *encoder) one() {
	if e.usedBits == 7 {
		e.currentByte |= 1
		e.nextWord()
	} else {
		e.currentByte |= 128 >> e.usedBits
		e.usedBits += 1
	}
}

// Write the current byte out to the buffer and begin next byte to write
// out. Add current byte to the buffer and set current byte and used
// bits to 0.
func (e *encoder) nextWord() {
	e.buffer = append(e.buffer, e.currentByte)

	e.currentByte = 0
	e.usedBits = 0
}
