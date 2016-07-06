package md

const UintSizeBits = 5 + (^uint(0) >> 63)
const BitsPerUint = (1 << UintSizeBits)
const UintBitsMask = BitsPerUint - 1
