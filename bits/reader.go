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
	var buf [md.BytesPerUint]byte
	s := buf[:]

	for readed < n {
		if r.remBits == 0 {
			nr, err := r.r.Read(s)
			if err != nil {
				if err == io.EOF && readed != 0 {
					return val & ((1 << readed) - 1), readed, nil
				} else {
					return 0, 0, err
				}
			}
			r.remBits = uint(nr * 8)
			r.bitBuf = md.BytesToUint(s)
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
