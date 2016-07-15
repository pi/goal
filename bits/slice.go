package bits

//
// BitSlice
//

import "gopkg.in/pi/goal/md"

type BitSlice struct {
	len  uint
	bits []uint
}

func NewBitSlice(args ...uint) *BitSlice {
	var l, c uint
	if len(args) > 1 {
		c = args[1]
	}
	if len(args) > 0 {
		l = args[0]
	}
	//l, c := getLenAndCap(args)
	return &BitSlice{
		bits: make([]uint, (l+md.BitsPerUint-1)>>md.UintSizeShift, (c+md.BitsPerUint-1)>>md.UintSizeShift),
		len:  l,
	}
}

// Len returns slice length in bits
func (s *BitSlice) Len() uint {
	return s.len
}

func (s *BitSlice) SetLen(newLen uint) {
	newLenInUints := (newLen + md.BitsPerUint - 1) >> md.UintSizeShift
	if newLenInUints > uint(len(s.bits)) {
		s.bits = s.bits[:newLenInUints]
	}
	s.len = newLen
}

// Cap returns slice's capacity in bits
func (s *BitSlice) Cap() uint {
	return uint(cap(s.bits) << md.UintSizeShift)
}

func (s *BitSlice) PutBit(index uint, value bool) {
	if index >= s.len {
		panic("bit array index out of bounds")
	}
	wi := index >> md.UintSizeShift
	bi := index & md.UintSizeMask
	if value {
		s.bits[wi] |= 1 << bi
	} else {
		s.bits[wi] &= ^(1 << bi)
	}
}

func (s *BitSlice) Bytes() []byte {
	r := make([]byte, (s.len+7)>>4)
	var v, rem uint
	for i, lim := uint(0), uint(s.len+7)>>4; i < lim; i++ {
		if rem == 0 {
			rem = 32
			v = s.bits[i>>md.UintSizeShift]
		}
		r[i] = byte(v)
		v >>= 8
		rem -= 8
	}
	return r
}

func (s *BitSlice) GetBit(index uint) bool {
	if index >= s.len {
		panic("bit array index out of bounds")
	}
	return ((s.bits[index>>md.UintSizeShift] >> (index & md.UintSizeMask)) & 1) == 1
}

func (s *BitSlice) GetBitRange(from, to uint) uint {
	if from > to || to >= s.len {
		panic("invalid index")
	}
	n := to - from + 1
	if n > md.BitsPerUint {
		panic("invalid number of bits")
	}
	var valueMask uint
	if n == md.BitsPerUint {
		valueMask = ^uint(0)
	} else {
		valueMask = (1 << n) - 1
	}
	lp := from & md.UintSizeMask
	if (from >> md.UintSizeShift) == (to >> md.UintSizeShift) {
		// bits in one uint
		return (s.bits[from>>md.UintSizeShift] >> lp) & valueMask
	} else {
		// bits in two uints
		wi := from >> md.UintSizeShift
		return ((s.bits[wi] >> lp) | (s.bits[wi+1] << (md.BitsPerUint - lp))) & valueMask
	}
}

func (s *BitSlice) PutBitRange(from, to, bits uint) {
	//TODO
	for i := from; i < to; i++ {
		s.PutBit(i, bits&1 == 1)
		bits >>= 1
	}
}

// Get bits with len
func (s *BitSlice) ReadBits(from, n uint) uint {
	if n > md.BitsPerUint {
		panic("too many bits to read")
	}
	if from+n > s.len {
		panic("bit array out of bounds")
	}
	return s.GetBitRange(from, from+n-1)
}

// Put bits with len
func (s *BitSlice) WriteBits(from, n, bits uint) {
	if n > md.BitsPerUint {
		panic("too many bits to write")
	}
	s.PutBitRange(from, from+n-1, bits)
}

// reset all bits to 0
func (s *BitSlice) Clear() {
	for i := 0; i < len(s.bits); i++ {
		s.bits[i] = 0
	}
}

func (s *BitSlice) AppendBits(n, bits uint) {
	if (s.len & md.UintSizeMask) == 0 {
		// on uint boundary
		if n < md.BitsPerUint {
			bits &= (1 << n) - 1
		}
		s.bits = append(s.bits, bits)
		s.len += n
	} else {
		from := s.len
		s.len += n
		if s.len > uint(len(s.bits)*md.BitsPerUint) {
			s.bits = append(s.bits, 0)
		}
		s.PutBitRange(from, from+n-1, bits)
	}
}

func (s *BitSlice) AppendBytes(bits []byte) {
	if (s.len & md.UintSizeMask) == 0 {
		// short way
		s.len += uint(len(bits)) * 8
		v := uint(0)
		bc := uint(0)
		for i := 0; i < len(bits); i++ {
			v := v | (uint(bits[i]) << bc)
			bc += 8
			if bc == md.BitsPerUint {
				s.bits = append(s.bits, v)
				v = 0
				bc = 0
			}
		}
		if bc > 0 {
			s.bits = append(s.bits, v)
		}
	} else {
		// long way
		for i := 0; i < len(bits); i++ {
			s.AppendBits(8, uint(bits[i]))
		}
	}
}

func (s *BitSlice) AppendUint(v uint) {
	if (s.len & md.UintSizeMask) == 0 {
		s.bits = append(s.bits, v)
		s.len += md.BitsPerUint
	} else {
		s.AppendBits(md.BitsPerUint, v)
	}
}

func (s *BitSlice) AppendUint32(v uint32) {
	if (s.len & md.UintSizeMask) == 0 {
		s.bits = append(s.bits, uint(v))
		s.len += 32
	} else {
		s.AppendBits(32, uint(v))
	}
}
