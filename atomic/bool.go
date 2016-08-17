package atomic

import (
	sa "sync/atomic"
	"unsafe"
)

type Bool int32

func (p *Bool) Get() bool {
	return (sa.LoadInt32((*int32)(unsafe.Pointer(p))) & 1) != 0
}

func (p *Bool) Set(new bool) {
	var v int32
	if new {
		v = 1
	}
	sa.StoreInt32((*int32)(unsafe.Pointer(p)), v)
}

func (p *Bool) Swap(new bool) (old bool) {
	var v int32
	if new {
		v = 1
	}
	return sa.SwapInt32((*int32)(unsafe.Pointer(p)), v) != 0
}

func (p *Bool) CompareAndSwap(old bool, new bool) (swapped bool) {
	var oi, ni int32

	if old {
		oi = 1
	}
	if new {
		ni = 1
	}
	return sa.CompareAndSwapInt32((*int32)(unsafe.Pointer(p)), oi, ni)
}
