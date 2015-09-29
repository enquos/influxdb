package pd1

// bool encoding uses 1 bit per value.  Each compressed byte slice contains a 1 byte header
// indicating the compression type, followed by a variable byte encoded length indicating
// how many booleans are packed in the slice.  The remaining bytes contains 1 byte for every
// 8 boolean values encoded.

import "encoding/binary"

type BoolEncoder interface {
	Write(b bool)
	Bytes() ([]byte, error)
}

type BoolDecoder interface {
	Next() bool
	Read() bool
}

type boolEncoder struct {
	// The encoded bytes
	bytes []byte

	// The current byte being encoded
	b byte

	// The number of bools packed into b
	i int

	// The total number of bools written
	n int
}

func NewBoolEncoder() BoolEncoder {
	return &boolEncoder{}
}

func (e *boolEncoder) Write(b bool) {
	// If we have filled the current byte, flush it
	if e.i >= 8 {
		e.flush()
	}

	// Use 1 bit for each boolen value, shift the current byte
	// by 1 and set the least signficant bit acordingly
	e.b = e.b << 1
	if b {
		e.b |= 1
	}

	// Increment the current bool count
	e.i += 1
	// Increment the total bool count
	e.n += 1
}

func (e *boolEncoder) flush() {
	// Pad remaining byte w/ 0s
	for e.i < 8 {
		e.b = e.b << 1
		e.i += 1
	}

	// If we have bits set, append them to the byte slice
	if e.i > 0 {
		e.bytes = append(e.bytes, e.b)
		e.b = 0
		e.i = 0
	}
}

func (e *boolEncoder) Bytes() ([]byte, error) {
	// Ensure the current byte is flushed
	e.flush()
	b := make([]byte, 10+1)

	// Store the encoding type in the 4 high bits of the first byte
	b[0] = byte(EncodingBitPacked) << 4

	i := 1
	// Encode the number of bools written
	i += binary.PutUvarint(b[i:], uint64(e.n))

	// Append the packed booleans
	return append(b[:i], e.bytes...), nil
}

type boolDecoder struct {
	b []byte
	i int
	n int
}

func NewBoolDecoder(b []byte) BoolDecoder {
	// First byte stores the encoding type, only have 1 bit-packet format
	// currently ignore for now.
	b = b[1:]
	count, n := binary.Uvarint(b)
	return &boolDecoder{b: b[n:], i: -1, n: int(count)}
}

func (e *boolDecoder) Next() bool {
	e.i += 1
	return e.i < e.n
}

func (e *boolDecoder) Read() bool {

	// Index into the byte slice
	idx := e.i / 8

	// Bit position
	pos := (8 - e.i%8) - 1

	// The mask to select the bit
	mask := byte(1 << uint(pos))

	// The packed byte
	v := e.b[idx]

	// Returns true if the bit is set
	return v&mask == mask
}