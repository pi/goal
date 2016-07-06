package goal

//
// Sparse bit slice
//

const _BitsPerChunkSizeBits = 13
const _BitsPerChunk = 1 << _BitsPerChunkSizeBits
const _BitsPerChunkSizeMask = _BitsPerChunk - 1

const _UintsPerChunk = _BitsPerChunk >> _UintSizeBits

type bitSliceChunk [_UintsPerChunk]uint

type BitSlice struct {
	len    uint
	chunks map[uint]*bitSliceChunk
}

func NewBitSlice() *BitSlice {
	return &BitSlice{
		chunks: make(map[uint]*bitSliceChunk),
		len:    0,
	}
}

func (a *BitSlice) Len() uint {
	return a.len
}

func (a *BitSlice) SetLen(newLen uint) {
	if newLen > a.len {
		a.len = newLen
	} else {
		oldLen := a.len
		a.len = newLen
		for dc := ((newLen + _BitsPerChunk - 1) >> _BitsPerChunkSizeBits) + 1; dc < ((oldLen + _BitsPerChunk - 1) >> _BitsPerChunkSizeBits); dc++ {
			delete(a.chunks, dc)
		}
	}
}

func (a *BitSlice) Clone() *BitSlice {
	c := NewBitSlice()
	for ci, ch := range a.chunks {
		var nc bitSliceChunk = *ch
		c.chunks[ci] = &nc
	}
	return c
}

func (a *BitSlice) PutBit(index uint, value uint) {
	if index >= a.len {
		panic("bit slice index out of bounds")
	}
	chunkIndex := uint(index >> _BitsPerChunkSizeBits)
	chunk, exists := a.chunks[chunkIndex]
	if !exists {
		chunk = new(bitSliceChunk)
		a.chunks[chunkIndex] = chunk
	}
	wi := uint(index&_BitsPerChunkSizeMask) >> _BitsPerUint
	bi := uint(index & _UintBitsMask)
	if (value & 1) == 0 {
		(*chunk)[wi] &= ^(1 << bi)
	} else {
		(*chunk)[wi] |= 1 << bi
	}
}

func (a *BitSlice) GetBit(index uint) uint {
	if index >= a.len {
		panic("bit array index out of bounds")
	}
	chunkIndex := uint(index >> _BitsPerChunkSizeBits)
	chunk, exists := a.chunks[chunkIndex]
	if !exists {
		return 0
	}
	return (((*chunk)[uint(index&_BitsPerChunkSizeMask)>>_BitsPerUint]) >> (index & _UintBitsMask)) & 1
}

func (a *BitSlice) uintFor(bitIndex uint) uint {
	chunk, exists := a.chunks[bitIndex>>_BitsPerChunkSizeBits]
	if !exists {
		return 0
	} else {
		return (*chunk)[uint(bitIndex&_BitsPerChunkSizeMask)>>_BitsPerUint]
	}
}

func (a *BitSlice) twoUintsFor(bitIndex uint) (uint, uint) {
	ci := bitIndex >> _BitsPerChunkSizeBits
	ui := uint(bitIndex&_BitsPerChunkSizeMask) >> _BitsPerUint
	chunk, exists := a.chunks[ci]
	var lo uint
	if !exists {
		lo = 0
	} else {
		lo = (*chunk)[ui]
		if (ui + 1) < _UintsPerChunk {
			// two uints in one chunk
			return lo, (*chunk)[ui+1]
		}
	}
	chunk, exists = a.chunks[ci+1]
	if exists {
		return lo, (*chunk)[0]
	}
	return lo, 0
}

func (a *BitSlice) Get(from, to uint) uint {
	if from > to || to >= a.len {
		panic("invalid index")
	}
	n := to - from + 1
	if n > _BitsPerUint {
		panic("invalid number of bits")
	}
	var valueMask uint
	if n == _BitsPerUint {
		valueMask = ^uint(0)
	} else {
		valueMask = (1 << n) - 1
	}
	lp := from & _UintBitsMask
	if lp == 0 {
		// on uint boundary
		return a.uintFor(from) & valueMask
	} else if (from >> _BitsPerUint) == (to >> _BitsPerUint) {
		// all bits in one uint
		return (a.uintFor(from) >> lp) & valueMask
	} else {
		// bits in two uints
		lo, hi := a.twoUintsFor(from)
		return ((lo >> lp) | (hi << (_BitsPerUint - lp))) & valueMask
	}
}

func (a *BitSlice) PutBits(from, to, bits uint) {
	//TODO optimize
	for i := from; i < to; i++ {
		a.PutBit(i, bits&1)
		bits >>= 1
	}
}

// Get bits with len
func (a *BitSlice) ReadBits(from, n uint) uint {
	if n > _BitsPerUint {
		panic("too many bits to read")
	}
	return a.Get(from, from+n-1)
}

// Put bits with len
func (a *BitSlice) WriteBits(from, n, bits uint) {
	a.PutBits(from, from+n-1, bits)
}

func (a *BitSlice) Clear() {
	a.chunks = make(map[uint]*bitSliceChunk)
	a.len = 0
}

func (a *BitSlice) AppendBits(n, bits uint) {
	from := a.len
	a.len += n
	a.PutBits(from, a.len-1, bits)
}
