package md

import (
	"encoding/binary"
	"time"
	"unsafe"
)

const UintSizeShift = 5 + (^uint(0) >> 63)
const BitsPerUint = (1 << UintSizeShift)
const BytesPerUint = BitsPerUint / 8
const UintSizeMask = BitsPerUint - 1

const BitsPerInt = BitsPerUint
const BytesPerInt = BitsPerInt

const MaxInt = int(1 << (UintSizeShift - 1))

const MinExactFloatInt = -18014398509481984 // int(^(uint(1) << 54)) + 1
const MaxExactFloatInt = 18014398509481984  // int(1 << uint(54))

var (
	IsBigEndian    bool
	IsLittleEndian bool
	NativeEndian   binary.ByteOrder
)

// UintToBytes puts 64-bit unsigned integer to to bytes in processor byte order
func UintToBytes(v uint, b []byte) {
	if len(b) < BytesPerUint {
		panic("too small buffer")
	}
	*(*uint)(unsafe.Pointer(&b[0])) = v
}

// UintFromBytes extracts 64-bit unsigned integer from bytes in processor byte order
func UintFromBytes(b []byte) uint {
	if len(b) < BytesPerUint {
		panic("too small buffer")
	}
	return *(*uint)(unsafe.Pointer(&b[0]))
}

// UintToLittleEndianBytes puts 64-bit unsigned integer to bytes in little-endian order
func UintToLittleEndianBytes(v uint, b []byte) {
	if IsLittleEndian {
		UintToBytes(v, b)
	} else {
		binary.LittleEndian.PutUint64(b, uint64(v))
	}
}

// UintFromLittleEndianBytes extracts 64-bit unsigned integer from bytes in little-endian order
func UintFromLittleEndianBytes(b []byte) uint {
	if IsLittleEndian {
		return UintFromBytes(b)
	} else {
		return uint(binary.LittleEndian.Uint64(b))
	}
}

//go:noescape
func runtimeNanotime() uint64

//go:linkname runtimeNanotime runtime.nanotime
func Monotime() time.Duration {
	return time.Duration(runtimeNanotime())
}

func init() {
	if BitsPerUint != 64 {
		panic("64-bit system required")
	}

	var buf [8]byte
	*(*uint)(unsafe.Pointer(&buf[0])) = 1
	IsBigEndian = buf[0] != 1
	IsLittleEndian = !IsBigEndian
	if IsBigEndian {
		NativeEndian = binary.BigEndian
	} else {
		NativeEndian = binary.LittleEndian
	}
}
