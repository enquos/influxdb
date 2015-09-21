package pd1

/*
This code is originally from: https://github.com/dgryski/go-tsz and has been modified to remove
the timestamp compression fuctionality.

It implements the float compression as presented in: http://www.vldb.org/pvldb/vol8/p1816-teller.pdf.
This implementation uses a sentinel value of NaN which means that float64 NaN cannot be stored using
this version.
*/

import (
	"bytes"
	"math"

	"github.com/dgryski/go-bits"
	"github.com/dgryski/go-bitstream"
)

type FloatEncoder struct {
	val float64

	leading  uint64
	trailing uint64

	buf bytes.Buffer
	bw  *bitstream.BitWriter

	first    bool
	finished bool
}

func NewFloatEncoder() *FloatEncoder {
	s := FloatEncoder{
		first:   true,
		leading: ^uint64(0),
	}

	s.bw = bitstream.NewWriter(&s.buf)

	return &s

}

func (s *FloatEncoder) Bytes() []byte {
	return s.buf.Bytes()
}

func (s *FloatEncoder) Finish() {

	if !s.finished {
		// // write an end-of-stream record
		s.Push(math.NaN())
		s.bw.Flush(bitstream.Zero)
		s.finished = true
	}
}

func (s *FloatEncoder) Push(v float64) {

	if s.first {
		// first point
		s.val = v
		s.first = false
		s.bw.WriteBits(math.Float64bits(v), 64)
		return
	}

	vDelta := math.Float64bits(v) ^ math.Float64bits(s.val)

	if vDelta == 0 {
		s.bw.WriteBit(bitstream.Zero)
	} else {
		s.bw.WriteBit(bitstream.One)

		leading := bits.Clz(vDelta)
		trailing := bits.Ctz(vDelta)

		// TODO(dgryski): check if it's 'cheaper' to reset the leading/trailing bits instead
		if s.leading != ^uint64(0) && leading >= s.leading && trailing >= s.trailing {
			s.bw.WriteBit(bitstream.Zero)
			s.bw.WriteBits(vDelta>>s.trailing, 64-int(s.leading)-int(s.trailing))
		} else {
			s.leading, s.trailing = leading, trailing

			s.bw.WriteBit(bitstream.One)
			s.bw.WriteBits(leading, 5)

			sigbits := 64 - leading - trailing
			s.bw.WriteBits(sigbits, 6)
			s.bw.WriteBits(vDelta>>trailing, int(sigbits))
		}
	}

	s.val = v
}

func (s *FloatEncoder) FloatDecoder() *FloatDecoder {
	iter, _ := NewFloatDecoder(s.buf.Bytes())
	return iter
}

type FloatDecoder struct {
	val float64

	leading  uint64
	trailing uint64

	br *bitstream.BitReader

	b []byte

	first    bool
	finished bool

	err error
}

func NewFloatDecoder(b []byte) (*FloatDecoder, error) {
	br := bitstream.NewReader(bytes.NewReader(b))

	v, err := br.ReadBits(64)
	if err != nil {
		return nil, err
	}

	return &FloatDecoder{
		val:   math.Float64frombits(v),
		first: true,
		br:    br,
		b:     b,
	}, nil
}

func (it *FloatDecoder) Next() bool {
	if it.err != nil || it.finished {
		return false
	}

	if it.first {
		it.first = false
		return true
	}

	// read compressed value
	bit, err := it.br.ReadBit()
	if err != nil {
		it.err = err
		return false
	}

	if bit == bitstream.Zero {
		// it.val = it.val
	} else {
		bit, err := it.br.ReadBit()
		if err != nil {
			it.err = err
			return false
		}
		if bit == bitstream.Zero {
			// reuse leading/trailing zero bits
			// it.leading, it.trailing = it.leading, it.trailing
		} else {
			bits, err := it.br.ReadBits(5)
			if err != nil {
				it.err = err
				return false
			}
			it.leading = bits

			bits, err = it.br.ReadBits(6)
			if err != nil {
				it.err = err
				return false
			}
			mbits := bits
			it.trailing = 64 - it.leading - mbits
		}

		mbits := int(64 - it.leading - it.trailing)
		bits, err := it.br.ReadBits(mbits)
		if err != nil {
			it.err = err
			return false
		}
		vbits := math.Float64bits(it.val)
		vbits ^= (bits << it.trailing)

		val := math.Float64frombits(vbits)
		if math.IsNaN(val) {
			it.finished = true
			return false
		}
		it.val = val
	}

	return true
}

func (it *FloatDecoder) Values() float64 {
	return it.val
}

func (it *FloatDecoder) Err() error {
	return it.err
}