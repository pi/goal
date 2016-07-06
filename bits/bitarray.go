package goal

//
// Sparse bit array
//

type BitArray struct {
	len  uint
	data []uint
}

func NewBitArray(len uint) *BitArray {
	return &BitArray{
		data: make([]uint, (len>>_BitsPerUint)+1),
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
	wi := index >> _BitsPerUint
	bi := index & _UintBitsMask
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
	return (a.data[index>>_BitsPerUint] >> (index & _UintBitsMask)) & 1
}

func (a *BitArray) GetBits(from, to uint) uint {
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
	if (from >> _BitsPerUint) == (to >> _BitsPerUint) {
		// bits in one uint
		return (a.data[from>>_BitsPerUint] >> lp) & valueMask
	} else {
		// bits in two uints
		wi := from >> _BitsPerUint
		return ((a.data[wi] >> lp) | (a.data[wi+1] << (_BitsPerUint - lp))) & valueMask
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
	if n > _BitsPerUint {
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
