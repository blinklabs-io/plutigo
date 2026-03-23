package syn

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"unicode/utf8"

	"github.com/blinklabs-io/plutigo/builtin"
	"github.com/blinklabs-io/plutigo/data"
	"github.com/blinklabs-io/plutigo/lang"
)

const decodeTermChunkSize = 384

func Decode[T Binder](bytes []byte) (*Program[T], error) {
	d := newDecoder(bytes)
	arena := newTermArena[T]()

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

	terms, err := decodeTermWithArena[T](d, arena)
	if err != nil {
		return nil, err
	}

	if major > math.MaxUint32 || minor > math.MaxUint32 ||
		patch > math.MaxUint32 {
		return nil, errors.New("version numbers too large")
	}

	version := lang.LanguageVersion{uint32(major), uint32(minor), uint32(patch)}

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
	return decodeTermWithArena(d, newTermArena[T]())
}

func decodeTermWithArena[T Binder](d *decoder, arena *termArena[T]) (Term[T], error) {
	tag, e := d.bits8(TermTagWidth)
	if e != nil {
		return nil, e
	}

	var term Term[T]

	switch tag {
	case VarTag:
		name, err := decodeVarBinder[T](d)
		if err != nil {
			return nil, err
		}

		term = arena.allocVar(name)
	case DelayTag:
		t, err := decodeTermWithArena[T](d, arena)
		if err != nil {
			return nil, err
		}

		term = arena.allocDelay(t)
	case LambdaTag:
		name, err := decodeParameterBinder[T](d)
		if err != nil {
			return nil, err
		}

		t, err := decodeTermWithArena[T](d, arena)
		if err != nil {
			return nil, err
		}

		term = arena.allocLambda(name, t)
	case ApplyTag:
		function, err := decodeTermWithArena[T](d, arena)
		if err != nil {
			return nil, err
		}

		argument, err := decodeTermWithArena[T](d, arena)
		if err != nil {
			return nil, err
		}

		term = arena.allocApply(function, argument)
	case ConstantTag:
		constant, err := DecodeConstant(d)
		if err != nil {
			return nil, err
		}

		term = arena.allocConstant(constant)
	case ForceTag:
		t, err := decodeTermWithArena[T](d, arena)
		if err != nil {
			return nil, err
		}

		term = arena.allocForce(t)
	case ErrorTag:
		term = arena.allocError()
	case BuiltinTag:
		builtinTag, err := d.bits8(BuiltinTagWidth)
		if err != nil {
			return nil, err
		}

		fn, err := builtin.FromByte(builtinTag)
		if err != nil {
			return nil, err
		}

		term = arena.allocBuiltin(fn)
	case ConstrTag:
		constrTag, err := d.word()
		if err != nil {
			return nil, err
		}

		fields, err := DecodeList(d, func(d *decoder) (Term[T], error) {
			return decodeTermWithArena[T](d, arena)
		})
		if err != nil {
			return nil, err
		}

		term = arena.allocConstr(constrTag, fields)
	case CaseTag:
		constr, err := decodeTermWithArena[T](d, arena)
		if err != nil {
			return nil, err
		}

		branches, err := DecodeList(d, func(d *decoder) (Term[T], error) {
			return decodeTermWithArena[T](d, arena)
		})
		if err != nil {
			return nil, err
		}

		term = arena.allocCase(constr, branches)
	default:
		return nil, fmt.Errorf("invalid term tag: %d", tag)
	}

	return term, nil
}

type arenaChunks[S any] struct {
	chunks   [][]S
	chunkIdx int
	offset   int
}

func (a *arenaChunks[S]) alloc() *S {
	if a.chunkIdx < len(a.chunks) {
		chunk := a.chunks[a.chunkIdx]
		if a.offset < len(chunk) {
			slot := &chunk[a.offset]
			a.offset++
			return slot
		}
	}

	chunk := make([]S, decodeTermChunkSize)
	a.chunks = append(a.chunks, chunk)
	a.chunkIdx = len(a.chunks) - 1
	a.offset = 1
	return &chunk[0]
}

type termArena[T Binder] struct {
	vars      arenaChunks[Var[T]]
	delays    arenaChunks[Delay[T]]
	forces    arenaChunks[Force[T]]
	lambdas   arenaChunks[Lambda[T]]
	applies   arenaChunks[Apply[T]]
	constrs   arenaChunks[Constr[T]]
	cases     arenaChunks[Case[T]]
	errors    arenaChunks[Error]
	constants arenaChunks[Constant]
	builtins  arenaChunks[Builtin]
}

func newTermArena[T Binder]() *termArena[T] {
	return &termArena[T]{}
}

func (a *termArena[T]) allocVar(name T) *Var[T] {
	term := a.vars.alloc()
	term.Name = name
	return term
}

func (a *termArena[T]) allocDelay(term Term[T]) *Delay[T] {
	delay := a.delays.alloc()
	delay.Term = term
	return delay
}

func (a *termArena[T]) allocForce(term Term[T]) *Force[T] {
	force := a.forces.alloc()
	force.Term = term
	return force
}

func (a *termArena[T]) allocLambda(name T, body Term[T]) *Lambda[T] {
	lambda := a.lambdas.alloc()
	lambda.ParameterName = name
	lambda.Body = body
	return lambda
}

func (a *termArena[T]) allocApply(function Term[T], argument Term[T]) *Apply[T] {
	apply := a.applies.alloc()
	apply.Function = function
	apply.Argument = argument
	return apply
}

func (a *termArena[T]) allocConstr(tag uint, fields []Term[T]) *Constr[T] {
	constr := a.constrs.alloc()
	constr.Tag = tag
	constr.Fields = fields
	return constr
}

func (a *termArena[T]) allocCase(constr Term[T], branches []Term[T]) *Case[T] {
	caseTerm := a.cases.alloc()
	caseTerm.Constr = constr
	caseTerm.Branches = branches
	return caseTerm
}

func (a *termArena[T]) allocError() *Error {
	return a.errors.alloc()
}

func (a *termArena[T]) allocConstant(constant IConstant) *Constant {
	term := a.constants.alloc()
	term.Con = constant
	return term
}

func (a *termArena[T]) allocBuiltin(fn builtin.DefaultFunction) *Builtin {
	term := a.builtins.alloc()
	term.DefaultFunction = fn
	return term
}

func decodeVarBinder[T Binder](d *decoder) (T, error) {
	var zero T

	switch any(zero).(type) {
	case DeBruijn:
		i, err := d.word()
		if err != nil {
			return zero, err
		}
		if i > math.MaxInt {
			return zero, fmt.Errorf("DeBruijn index too large: %d", i)
		}
		return any(DeBruijn(i)).(T), nil
	case NamedDeBruijn:
		text, err := d.utf8()
		if err != nil {
			return zero, err
		}
		i, err := d.word()
		if err != nil {
			return zero, err
		}
		if i > math.MaxInt {
			return zero, fmt.Errorf("DeBruijn index too large: %d", i)
		}
		return any(NamedDeBruijn{
			Text:  text,
			Index: DeBruijn(i),
		}).(T), nil
	case Name:
		text, err := d.utf8()
		if err != nil {
			return zero, err
		}
		i, err := d.word()
		if err != nil {
			return zero, err
		}
		return any(Name{
			Text:   text,
			Unique: Unique(i),
		}).(T), nil
	default:
		binder, err := zero.VarDecode(d)
		if err != nil {
			return zero, err
		}
		name, ok := binder.(T)
		if !ok {
			return zero, fmt.Errorf(
				"VarDecode returned wrong type: got %T, want %T",
				binder,
				zero,
			)
		}
		return name, nil
	}
}

func decodeParameterBinder[T Binder](d *decoder) (T, error) {
	var zero T

	switch any(zero).(type) {
	case DeBruijn:
		return any(DeBruijn(0)).(T), nil
	case NamedDeBruijn:
		text, err := d.utf8()
		if err != nil {
			return zero, err
		}
		i, err := d.word()
		if err != nil {
			return zero, err
		}
		if i > math.MaxInt {
			return zero, fmt.Errorf("DeBruijn index too large: %d", i)
		}
		return any(NamedDeBruijn{
			Text:  text,
			Index: DeBruijn(i),
		}).(T), nil
	case Name:
		text, err := d.utf8()
		if err != nil {
			return zero, err
		}
		i, err := d.word()
		if err != nil {
			return zero, err
		}
		return any(Name{
			Text:   text,
			Unique: Unique(i),
		}).(T), nil
	default:
		binder, err := zero.ParameterDecode(d)
		if err != nil {
			return zero, err
		}
		name, ok := binder.(T)
		if !ok {
			return zero, fmt.Errorf(
				"ParameterDecode returned wrong type: got %T, want %T",
				binder,
				zero,
			)
		}
		return name, nil
	}
}

func DecodeConstant(d *decoder) (IConstant, error) {
	var tags constantTagSeq
	err := decodeConstantTags(d, &tags)
	if err != nil {
		return nil, err
	}
	typ, err := decodeConstantType(&tags)
	if err != nil {
		return nil, err
	}
	return decodeConstantValue(d, typ)
}

func decodeConstantValue(d *decoder, typ Typ) (IConstant, error) {
	var constant IConstant

	switch t := typ.(type) {
	// Integer
	case *TInteger:
		i, err := d.integer()
		if err != nil {
			return nil, err
		}

		constant = &Integer{i}

	// ByteString
	case *TByteString:
		b, err := d.bytes()
		if err != nil {
			return nil, err
		}

		constant = &ByteString{b}

	// String
	case *TString:
		s, err := d.utf8()
		if err != nil {
			return nil, err
		}

		constant = &String{s}

	// Unit
	case *TUnit:
		constant = &Unit{}

	// Bool
	case *TBool:
		v, err := d.bit()
		if err != nil {
			return nil, err
		}

		constant = &Bool{v}

	// ProtoList
	case *TList:
		items, err := DecodeList(d, func(d *decoder) (IConstant, error) { return decodeConstantValue(d, t.Typ) })
		if err != nil {
			return nil, err
		}
		constant = &ProtoList{
			LTyp: t.Typ,
			List: items,
		}

	// ProtoPair
	case *TPair:
		first, err := decodeConstantValue(d, t.First)
		if err != nil {
			return nil, err
		}
		second, err := decodeConstantValue(d, t.Second)
		if err != nil {
			return nil, err
		}
		constant = &ProtoPair{
			FstType: t.First,
			SndType: t.Second,
			First:   first,
			Second:  second,
		}

	// Data
	case *TData:
		cborBytes, err := d.bytes()
		if err != nil {
			return nil, err
		}

		pd, err := data.Decode(cborBytes)
		if err != nil {
			return nil, err
		}

		constant = &Data{pd}

	default:
		return nil, errors.New("unknown constant constructor")
	}

	return constant, nil
}

var (
	cachedTInteger    Typ = &TInteger{}
	cachedTByteString Typ = &TByteString{}
	cachedTString     Typ = &TString{}
	cachedTUnit       Typ = &TUnit{}
	cachedTBool       Typ = &TBool{}
	cachedTData       Typ = &TData{}
)

type constantTagSeq struct {
	small [8]byte
	extra []byte
	n     int
}

func (s *constantTagSeq) append(tag byte) {
	if s.n < len(s.small) {
		s.small[s.n] = tag
	} else {
		s.extra = append(s.extra, tag)
	}
	s.n++
}

func (s *constantTagSeq) at(idx int) byte {
	if idx < len(s.small) {
		return s.small[idx]
	}
	return s.extra[idx-len(s.small)]
}

func (s *constantTagSeq) len() int {
	return s.n
}

func decodeConstantType(tags *constantTagSeq) (Typ, error) {
	typ, next, err := decodeConstantTypeAt(tags, 0)
	if err != nil {
		return nil, err
	}
	if next != tags.len() {
		return nil, errors.New("unknown type tag")
	}
	return typ, nil
}

func decodeConstantTypeAt(tags *constantTagSeq, idx int) (Typ, int, error) {
	if idx >= tags.len() {
		return nil, idx, errors.New("unknown type tag")
	}

	next := tags.at(idx)
	idx++

	switch next {
	case IntegerTag:
		return cachedTInteger, idx, nil
	case ByteStringTag:
		return cachedTByteString, idx, nil
	case StringTag:
		return cachedTString, idx, nil
	case UnitTag:
		return cachedTUnit, idx, nil
	case BoolTag:
		return cachedTBool, idx, nil
	case DataTag:
		return cachedTData, idx, nil
	// NOTE: this also covers ProtoPairOneTag, but it's the same value as ProtoListOneTag.
	case ProtoListOneTag:
		if idx >= tags.len() {
			return nil, idx, errors.New("unknown type tag")
		}
		switch tags.at(idx) {
		case ProtoListTwoTag:
			subType, next, err := decodeConstantTypeAt(tags, idx+1)
			if err != nil {
				return nil, next, err
			}
			return &TList{Typ: subType}, next, nil
		case ProtoPairTwoTag:
			idx++
			if idx >= tags.len() || tags.at(idx) != ProtoPairThreeTag {
				return nil, idx, errors.New("unknown type tag")
			}
			first, next, err := decodeConstantTypeAt(tags, idx+1)
			if err != nil {
				return nil, next, err
			}
			second, next, err := decodeConstantTypeAt(tags, next)
			if err != nil {
				return nil, next, err
			}
			return &TPair{First: first, Second: second}, next, nil
		default:
			return nil, idx, errors.New("unknown type tag")
		}
	default:
		return nil, idx, errors.New("unknown type tag")
	}
}

func decodeConstantTags(d *decoder, tags *constantTagSeq) error {
	for {
		bit, err := d.bit()
		if err != nil {
			return err
		}
		if !bit {
			return nil
		}
		tag, err := d.bits8(ConstTagWidth)
		if err != nil {
			return err
		}
		tags.append(tag)
	}
}

// Decode a list of items with a decoder function.
// This is byte alignment agnostic.
// Decode a bit from the buffer.
// If 0 then stop.
// Otherwise we decode an item in the list with the decoder function passed
// in. Then decode the next bit in the buffer and repeat above.
// Returns a list of items decoded with the decoder function.
func DecodeList[T any](
	d *decoder,
	decoderFunc func(*decoder) (T, error),
) ([]T, error) {
	result := make([]T, 0, 4)

	for {
		bit, err := d.bit()
		if err != nil {
			return nil, err
		}
		if !bit {
			break
		}
		item, err := decoderFunc(d)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, nil
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

	if d.usedBits == 0 {
		for {
			if d.pos >= len(d.buffer) {
				return 0, errors.New("end of buffer")
			}
			word8 := d.buffer[d.pos]
			d.pos++

			finalWord |= uint(word8&127) << shl
			shl += 7
			if word8&128 == 0 {
				return finalWord, nil
			}
		}
	}

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
	if d.pos >= len(d.buffer) {
		return 0, errors.New("end of buffer")
	}
	if numBits == 8 {
		if d.usedBits == 0 {
			x := d.buffer[d.pos]
			d.pos++
			return x, nil
		}
		if d.pos+1 >= len(d.buffer) {
			return 0, fmt.Errorf("NotEnoughBits(%d)", numBits)
		}
		x := (d.buffer[d.pos] << byte(d.usedBits)) | (d.buffer[d.pos+1] >> (8 - byte(d.usedBits)))
		d.pos++
		return x, nil
	}

	remainingBits := (len(d.buffer)-d.pos)*8 - int(d.usedBits)
	if int(numBits) > remainingBits {
		return 0, fmt.Errorf("NotEnoughBits(%d)", numBits)
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
	allUsedBits := int64(numBits) + d.usedBits //nolint:gosec

	d.usedBits = allUsedBits % 8

	d.pos += int(allUsedBits / 8)
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

	blkLen := int(d.buffer[d.pos])
	d.pos++
	result := make([]byte, 0, blkLen)

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

// integer decodes a variable-length signed integer from the buffer.
// It is byte-alignment agnostic. Reads 8 bits at a time, using the 7 least
// significant bits for the unsigned integer, continuing if the MSB is 1,
// stopping if 0, then applies zigzag decoding to get the signed integer.
func (d *decoder) integer() (*big.Int, error) {
	word, err := d.bigWord()
	if err != nil {
		return nil, err
	}

	return unzigzag(word), nil
}

// bigWord decodes a variable-length unsigned integer from the buffer.
// It is byte-alignment agnostic. Reads 8 bits at a time, using the 7 least
// significant bits for the integer, continuing if the MSB is 1, stopping if 0.
func (d *decoder) bigWord() (*big.Int, error) {
	finalWord := new(big.Int)
	shift := uint(0)

	if d.usedBits == 0 {
		for {
			if d.pos >= len(d.buffer) {
				return nil, errors.New("end of buffer")
			}
			word8 := d.buffer[d.pos]
			d.pos++

			finalWord.Or(finalWord, new(big.Int).Lsh(new(big.Int).SetInt64(int64(word8&0x7F)), shift))
			shift += 7
			if word8&0x80 == 0 {
				return finalWord, nil
			}
		}
	}

	for {
		word8, err := d.bits8(8)
		if err != nil {
			return nil, err
		}
		// Get 7 least significant bits: word8 & 0x7F
		word7 := word8 & 0x7F

		// Shift and OR into finalWord
		part := new(big.Int).SetInt64(int64(word7))
		shiftedPart := new(big.Int).Lsh(part, shift)
		finalWord.Or(finalWord, shiftedPart)

		// Increment shift by 7
		shift += 7

		// Check MSB: word8 & 0x80
		leadingBit := word8 & 0x80
		if leadingBit == 0 {
			break
		}
	}

	return finalWord, nil
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
