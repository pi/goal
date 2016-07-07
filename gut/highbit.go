package gut

var smallBitLenTable = [16]int{
	0,
	1,
	2,
	2,
	3,
	3,
	3,
	3,
	4,
	4,
	4,
	4,
	4,
	4,
	4,
	4,
}

func BitLen(x uint) (n int) {
	if x >= 0x80000000 {
		x >>= 32
		n += 32
	}
	if x >= 0x8000 {
		x >>= 16
		n += 16
	}
	if x >= 0x80 {
		x >>= 8
		n += 8
	}
	if x >= 0x8 {
		x >>= 4
		n += 4
	}

	return n + smallBitLenTable[x]
	/*
		if x >= 0x2 {
			x >>= 2
			n += 2
		}
		if x >= 0x1 {
			n++
		}
		return n
	*/
}
