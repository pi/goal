package md

import (
	"unsafe"
)

const UintSizeShift = 5 + (^uint(0) >> 63)
const BitsPerUint = (1 << UintSizeShift)
const BytesPerUint = BitsPerUint / 8
const UintSizeMask = BitsPerUint - 1

const MaxInt = int(1 << (UintSizeShift - 1))

const MinExactFloatInt = -18014398509481984 // int(^(uint(1) << 54)) + 1
const MaxExactFloatInt = 18014398509481984  // int(1 << uint(54))

// UintToBytes convert uint to binary representation bytes to uint value using machine byte order
func UintToBytes(v uint, b []byte) {
	if len(b) < BytesPerUint {
		panic("too small buffer")
	}
	*(*uint)(unsafe.Pointer(&b[0])) = v
}

// BytesToUint convert binary representation bytes to uint value using machine byte order
func BytesToUint(b []byte) uint {
	if len(b) < BytesPerUint {
		panic("too small buffer")
	}
	return *(*uint)(unsafe.Pointer(&b[0]))
}

func init() {
	if BitsPerUint != 64 {
		panic("64-bit system required")
	}

	var buf [8]byte
	*(*uint)(unsafe.Pointer(&buf[0])) = 1
	if buf[0] != 1 {
		panic("little-endian system required")
	}
}
