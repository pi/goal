package gut

import (
	"reflect"
	"sync/atomic"
	"unsafe"

	"github.com/pi/goal/md"
)

type UnsafeMemoryPool struct {
	mem       []byte
	allocated uint64
}

type SharedUnsafeMemoryPool struct {
	UnsafeMemoryPool
}

func NewUnsafeMemoryPool(size uint) (*UnsafeMemoryPool, error) {
	p := &UnsafeMemoryPool{}
	err := p.init(size)
	if err != nil {
		return nil, err
	}
	return p, nil
}
func NewSharedUnsafeMemoryPool(size uint) (*SharedUnsafeMemoryPool, error) {
	p := &SharedUnsafeMemoryPool{}
	err := p.init(size)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *UnsafeMemoryPool) init(size uint) error {
	var err error
	p.mem, err = md.VAlloc(size)
	if err != nil {
		return err
	} else {
		return nil
	}
}
func (p *UnsafeMemoryPool) Reset() {
	p.allocated = 0
}
func (p *UnsafeMemoryPool) Done() error {
	m := p.mem
	if m == nil {
		panic("pool already finalized")
	}
	p.mem = nil
	return md.VFree(m)
}
func (p *UnsafeMemoryPool) Alloc(n uint) (block unsafe.Pointer) {
	n = (n + 7) & ^uint(7)
	if uint64(n) > (uint64(cap(p.mem)) - p.allocated) {
		var null uintptr
		return unsafe.Pointer(null) // work around internal compiler bug: go can't compile unsafe.Pointer(uintptr(0)) here
	}
	block = unsafe.Pointer(&p.mem[p.allocated])
	p.allocated += uint64(n)
	return block
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
func (p *SharedUnsafeMemoryPool) Alloc(n uint) unsafe.Pointer {
	alignedSize := uint64(n+7) & ^uint64(7)
	for {
		oldAllocated := p.allocated
		newAllocated := oldAllocated + alignedSize
		if newAllocated < oldAllocated || newAllocated > uint64(cap(p.mem)) {
			var null uintptr
			return unsafe.Pointer(null) // work around internal compiler bug: Go can't compile unsafe.Pointer(uintptr(0)) here yet
		}
		if atomic.CompareAndSwapUint64(&p.allocated, oldAllocated, newAllocated) {
			return unsafe.Pointer(&p.mem[oldAllocated])
		}
	}
}
