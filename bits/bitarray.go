package bits

//
// Sparse bit array
//

import "github.com/ardente/goal/md"

type BitArray struct {
	len  uint
	data []uint
}

func NewBitArray(len uint) *BitArray {
	return &BitArray{
		data: make([]uint, (len>>md.BitsPerUint)+1),
		len:  len,
	}
}

func (a *BitArray) Len() uint {
	return a.len
}

func (a *BitArray) Put(index uint, value uint) {
	if index >= a.len {
		panic("bit array index out of bounds")
	}
	wi := index >> md.BitsPerUint
	bi := index & md.UintBitsMask
	if (value & 1) == 0 {
		a.data[wi] &= ^(1 << bi)
	} else {
		a.data[wi] |= 1 << bi
	}
}

func (a *BitArray) Get(index uint) uint {
	if index >= a.len {
		panic("bit array index out of bounds")
	}
	return (a.data[index>>md.BitsPerUint] >> (index & md.UintBitsMask)) & 1
}

func (a *BitArray) GetBits(from, to uint) uint {
	if from > to || to >= a.len {
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
	lp := from & md.UintBitsMask
	if (from >> md.BitsPerUint) == (to >> md.BitsPerUint) {
		// bits in one uint
		return (a.data[from>>md.BitsPerUint] >> lp) & valueMask
	} else {
		// bits in two uints
		wi := from >> md.BitsPerUint
		return ((a.data[wi] >> lp) | (a.data[wi+1] << (md.BitsPerUint - lp))) & valueMask
	}
}

func (a *BitArray) PutBits(from, to, bits uint) {
	//TODO
	for i := from; i < to; i++ {
		a.Put(i, bits&1)
		bits >>= 1
	}
}

// Get bits with len
func (a *BitArray) ReadBits(from, n uint) uint {
	if n > md.BitsPerUint {
		panic("too many bits to read")
	}
	if from+n > a.len {
		panic("bit array out of bounds")
	}
	return a.GetBits(from, from+n-1)
}

// Put bits with len
func (a *BitArray) WriteBits(from, n, bits uint) {
	a.PutBits(from, from+n-1, bits)
}

// reset all bits to 0
func (a *BitArray) Clear() {
	for i := 0; i < len(a.data); i++ {
		a.data[i] = 0
	}
}
