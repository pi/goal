package md

const Is64Bit = (^uintptr(0) >> 63) == 1
const UintSizeBits = 5 + (^uint(0) >> 63)
const BitsPerUint = (1 << UintSizeBits)
const UintBitsMask = BitsPerUint - 1
