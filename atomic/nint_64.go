//+build amd64 arm64 amd64p32

// Atomic operations for native int/uint
// In additon to standard functions there is atomic bitwise operations (with spin if necessary)
package atomic

import (
	"runtime"
	sa "sync/atomic"
	"unsafe"
)

// AddInt see stdlib atomic.AddIntX
func AddInt(p *int, delta int) (new int) {
	return int(sa.AddInt64((*int64)(unsafe.Pointer(p)), int64(delta)))
}

// LoadInt see stdlib atomic.LoadIntX
func LoadInt(p *int) (value int) {
	return int(sa.LoadInt64((*int64)(unsafe.Pointer(p))))
}

// StoreInt see stdlib atomic.StoreIntX
func StoreInt(p *int, new int) {
	sa.StoreInt64((*int64)(unsafe.Pointer(p)), int64(new))
}

// SwapInt see stdlib atomic.SwapIntX
func SwapInt(p *int, new int) (old int) {
	return int(sa.SwapInt64((*int64)(unsafe.Pointer(p)), int64(new)))
}

// CompareAndSwapInt see stdlib atomic.CompareAndSwapIntX
func CompareAndSwapInt(p *int, old, new int) (swapped bool) {
	return sa.CompareAndSwapInt64((*int64)(unsafe.Pointer(p)), int64(old), int64(new))
}

// AddUint see stdlib atomic.AddUintX
func AddUint(p *uint, delta uint) (new uint) {
	return uint(sa.AddUint64((*uint64)(unsafe.Pointer(p)), uint64(delta)))
}

// LoadUint see stdlib atomic.LoadUintX
func LoadUint(p *uint) (value uint) {
	return uint(sa.LoadUint64((*uint64)(unsafe.Pointer(p))))
}

// StoreUint see stdlib atomic.StoreUintX
func StoreUint(p *uint, new uint) {
	sa.StoreUint64((*uint64)(unsafe.Pointer(p)), uint64(new))
}

// SwapUint see stdlib atomic.SwapUintX
func SwapUint(p *uint, new uint) (old uint) {
	return uint(sa.SwapUint64((*uint64)(unsafe.Pointer(p)), uint64(new)))
}

// CompareAndSwapUint see stdlib atomic.CompareAndSwapUintX
func CompareAndSwapUint(p *uint, old, new uint) (swapped bool) {
	return sa.CompareAndSwapUint64((*uint64)(unsafe.Pointer(p)), uint64(old), uint64(new))
}

// Atomic bitwise operations

// AtomicOrInt bitwise OR with spinning if necessary
func AtomicOrInt(p *int, bits int) {
	for {
		old := LoadInt(p)
		if (old & bits) == bits {
			return
		}
		new := old | bits
		if CompareAndSwapInt(p, old, new) {
			return
		}
		runtime.Gosched()
	}
}

// AtomicAndInt bitwise AND with spinning if necessary
func AtomicAndInt(p *int, bits int) {
	for {
		old := LoadInt(p)
		if CompareAndSwapInt(p, old, old&bits) {
			return
		}
		runtime.Gosched()
	}
}

// AtomicXorInt bitwise XOR with spinning if necessary
func AtomicXorInt(p *int, bits int) {
	for {
		old := LoadInt(p)
		if CompareAndSwapInt(p, old, old^bits) {
			return
		}
		runtime.Gosched()
	}
}

// AtomicOrUint bitwise OR with spinning if necessary
func AtomicOrUint(p *uint, bits uint) {
	for {
		old := LoadUint(p)
		if (old & bits) == bits {
			return
		}
		new := old | bits
		if CompareAndSwapUint(p, old, new) {
			return
		}
		runtime.Gosched()
	}
}

// AtomicAndUint bitwise AND with spinning if necessary
func AtomicAndUint(p *uint, bits uint) {
	for {
		old := LoadUint(p)
		if CompareAndSwapUint(p, old, old&bits) {
			return
		}
		runtime.Gosched()
	}
}

// AtomicXorUint bitwise XOR with spinning if necessary
func AtomicXorUint(p *uint, bits uint) {
	for {
		old := LoadUint(p)
		if CompareAndSwapUint(p, old, old^bits) {
			return
		}
		runtime.Gosched()
	}
}
