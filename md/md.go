package md

const UintSizeShift = 5 + (^uint(0) >> 63)
const BitsPerUint = (1 << UintSizeShift)
const UintSizeMask = BitsPerUint - 1

func init() {
	if BitsPerUint != 64 {
		panic("64-bit system required")
	}
}
