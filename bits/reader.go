package bits

import (
	"io"

	"github.com/ardente/goal/md"
)

type BitReader struct {
	r       io.Reader
	bitBuf  uint
	remBits uint
}

func NewReader(r io.Reader) *BitReader {
	return &BitReader{
		r: r,
	}
}

// Read read bits from underlying reader
// return bits, number of bits readed, error
func (r *BitReader) Read(n uint) (uint, uint, error) {
	var val, readed uint

	for readed < n {
		if r.remBits == 0 {
			buf := make([]byte, md.BytesPerUint)
			nr, err := r.r.Read(buf)
			if err != nil {
				return val & ((1 << readed) - 1), readed, err
			}
			r.remBits = 0
			r.bitBuf = 0
			for i := 0; i < nr; i++ {
				r.bitBuf |= uint(buf[i]) << r.remBits
				r.remBits += 8
			}
		}
		toRead := n - readed
		if toRead > r.remBits {
			toRead = r.remBits
		}
		val |= r.bitBuf << readed
		r.bitBuf >>= toRead
		r.remBits -= toRead
		readed += toRead
	}
	return val & ((1 << readed) - 1), readed, nil
}
