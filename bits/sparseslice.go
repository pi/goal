package bits

//
// Sparse bit slice
//

// prefix: sbs

import (
	"github.com/ardente/goal/md"
)

const sbsBitsPerChunkSizeShift = 10
const sbsBitsPerChunk = 1 << sbsBitsPerChunkSizeShift
const sbsBitsPerChunkMask = sbsBitsPerChunk - 1
const sbsUintsPerChunk = sbsBitsPerChunk >> md.UintSizeShift

type sbsChunk [sbsUintsPerChunk]uint

type SparseBitSlice struct {
	len    uint
	chunks map[uint]*sbsChunk
}

func NewSparseBitSlice() *SparseBitSlice {
	return &SparseBitSlice{
		chunks: make(map[uint]*sbsChunk),
		len:    0,
	}
}

func (s *SparseBitSlice) Len() uint {
	return s.len
}

func (s *SparseBitSlice) SetLen(newLen uint) {
	if newLen > s.len {
		s.len = newLen
	} else {
		oldLen := s.len
		s.len = newLen
		for dc := ((newLen + sbsBitsPerChunk - 1) >> sbsBitsPerChunkSizeShift) + 1; dc < ((oldLen + sbsBitsPerChunk - 1) >> sbsBitsPerChunkSizeShift); dc++ {
			delete(s.chunks, dc)
		}
	}
}

func (s *SparseBitSlice) Clone() *SparseBitSlice {
	c := NewSparseBitSlice()
	for ci, ch := range s.chunks {
		nc := *ch
		c.chunks[ci] = &nc
	}
	c.len = s.len
	return c
}

func (s *SparseBitSlice) Bytes() []byte {
	r := make([]byte, (s.Len()+7)/8)
	for ci, ch := range s.chunks {
		i := (ci << sbsBitsPerChunkSizeShift) >> 4 // chunk index to byte index
		for j := 0; j < sbsUintsPerChunk; j++ {
			u := (*ch)[j]
			r[i] = byte(u)
			r[i+1] = byte(u >> 8)
			r[i+2] = byte(u >> 16)
			r[i+3] = byte(u >> 24)
			i += 4
			if md.BitsPerUint == 64 {
				ull := uint64(u) // this cast makes vet happy
				r[i+4] = byte(ull >> 32)
				r[i+5] = byte(ull >> 40)
				r[i+6] = byte(ull >> 48)
				r[i+7] = byte(ull >> 56)
				i += 4
			}
		}
	}
	return r
}

func (s *SparseBitSlice) PutBit(index uint, value bool) {
	if index >= s.len {
		panic("bit slice index out of bounds")
	}
	chunkIndex := uint(index >> sbsBitsPerChunkSizeShift)
	chunk, exists := s.chunks[chunkIndex]
	if !exists {
		chunk = new(sbsChunk)
		s.chunks[chunkIndex] = chunk
	}
	wi := uint(index&sbsBitsPerChunkMask) >> md.UintSizeShift
	bi := uint(index & md.UintSizeMask)
	if value {
		(*chunk)[wi] |= 1 << bi
	} else {
		(*chunk)[wi] &= ^(1 << bi)
	}
}

func (s *SparseBitSlice) GetBit(index uint) bool {
	if index >= s.len {
		panic("bit array index out of bounds")
	}
	chunkIndex := uint(index >> sbsBitsPerChunkSizeShift)
	chunk, exists := s.chunks[chunkIndex]
	if !exists {
		return false
	}
	return (((*chunk)[uint(index&sbsBitsPerChunkMask)>>md.UintSizeShift] >> (index & md.UintSizeMask)) & 1) == 1
}

func (s *SparseBitSlice) uintFor(bitIndex uint) uint {
	chunk, exists := s.chunks[bitIndex>>sbsBitsPerChunkSizeShift]
	if !exists {
		return 0
	}
	return (*chunk)[uint(bitIndex&sbsBitsPerChunkMask)>>md.UintSizeShift]
}

func (s *SparseBitSlice) twoUintsFor(bitIndex uint) (uint, uint) {
	ci := bitIndex >> sbsBitsPerChunkSizeShift
	ui := uint(bitIndex&sbsBitsPerChunkMask) >> md.UintSizeShift
	chunk, exists := s.chunks[ci]
	var lo uint
	if !exists {
		lo = 0
	} else {
		lo = (*chunk)[ui]
		if (ui + 1) < sbsUintsPerChunk {
			// two uints in one chunk
			return lo, (*chunk)[ui+1]
		}
	}
	chunk, exists = s.chunks[ci+1]
	if exists {
		return lo, (*chunk)[0]
	}
	return lo, 0
}

func (s *SparseBitSlice) GetBitRange(from, to uint) uint {
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
	if lp == 0 {
		// on uint boundary
		return s.uintFor(from) & valueMask
	} else if (from >> md.UintSizeShift) == (to >> md.UintSizeShift) {
		// all bits in one uint
		return (s.uintFor(from) >> lp) & valueMask
	} else {
		// bits in two uints
		lo, hi := s.twoUintsFor(from)
		return ((lo >> lp) | (hi << (md.BitsPerUint - lp))) & valueMask
	}
}

func (s *SparseBitSlice) PutBitRange(from, to, bits uint) {
	//TODO optimize
	for i := from; i <= to; i++ {
		s.PutBit(i, (bits&1) == 1)
		bits >>= 1
	}
}

// Get bits with len
func (s *SparseBitSlice) ReadBits(from, n uint) uint {
	if n > md.BitsPerUint {
		panic("too many bits to read")
	}
	return s.GetBitRange(from, from+n-1)
}

// Put bits with len
func (s *SparseBitSlice) WriteBits(from, n, bits uint) {
	s.PutBitRange(from, from+n-1, bits)
}

func (s *SparseBitSlice) Clear() {
	s.chunks = make(map[uint]*sbsChunk)
	s.len = 0
}

func (s *SparseBitSlice) AppendBits(n, bits uint) {
	from := s.len
	s.len += n
	s.PutBitRange(from, s.len-1, bits)
}
