package gut

import (
	"reflect"
	"sync/atomic"
	"unsafe"

	"github.com/ardente/goal/md"
)

type UnsafeMemoryPool struct {
	mem       []byte
	allocated uint64
	shared    bool
}

func NewUnsafeMemoryPool(size uint) *UnsafeMemoryPool {
	p := &UnsafeMemoryPool{}
	var err error
	p.mem, err = md.VAlloc(size)
	if err != nil {
		panic(err)
	}
	return p
}
func NewSharedUnsafeMemoryPool(size uint) *UnsafeMemoryPool {
	p := NewUnsafeMemoryPool(size)
	p.shared = true
	return p
}
func (p *UnsafeMemoryPool) Reset() {
	p.allocated = 0
}
func (p *UnsafeMemoryPool) Done() {
	err := md.VFree(p.mem)
	if err != nil {
		panic(err)
	}
	p.mem = nil
}
func (p *UnsafeMemoryPool) Alloc(n uint) (block unsafe.Pointer) {
	n = (n + 7) & ^uint(7)
	if uint64(n) > uint64(cap(p.mem))-p.allocated {
		return unsafe.Pointer(uintptr(0))
	}
	if p.shared {
		ptr := atomic.AddUint64(&p.allocated, uint64(n))
		if ptr > uint64(cap(p.mem)) {
			// race oom
			return unsafe.Pointer(uintptr(0))
		}
		block = unsafe.Pointer(&p.mem[ptr-uint64(n)])
	} else {
		block = unsafe.Pointer(&p.mem[p.allocated])
		p.allocated += uint64(n)
	}
	return
}
func (p *UnsafeMemoryPool) allocSlice(n uint, bytesPerElement uint) *reflect.SliceHeader {
	ptr := uintptr(p.Alloc(n * bytesPerElement))
	if ptr == 0 {
		return nil
	}
	return &reflect.SliceHeader{
		Data: ptr,
		Cap:  int(n),
		Len:  int(n),
	}
}
func (p *UnsafeMemoryPool) AllocBytes(n uint) []byte {
	if sh := p.allocSlice(n, 1); sh == nil {
		return nil
	} else {
		return *(*[]byte)(unsafe.Pointer(sh))
	}
}
func (p *UnsafeMemoryPool) AllocInts(n uint) []int {
	if sh := p.allocSlice(n, 8); sh == nil {
		return nil
	} else {
		return *(*[]int)(unsafe.Pointer(sh))
	}
}
func (p *UnsafeMemoryPool) AllocUints(n uint) []uint {
	if sh := p.allocSlice(n, 8); sh == nil {
		return nil
	} else {
		return *(*[]uint)(unsafe.Pointer(sh))
	}
}
func (p *UnsafeMemoryPool) AllocFloats32(n uint) []uint32 {
	if sh := p.allocSlice(n, 4); sh == nil {
		return nil
	} else {
		return *(*[]uint32)(unsafe.Pointer(sh))
	}
}
func (p *UnsafeMemoryPool) AllocFloats64(n uint) []float64 {
	if sh := p.allocSlice(n, 8); sh == nil {
		return nil
	} else {
		return *(*[]float64)(unsafe.Pointer(sh))
	}
}
