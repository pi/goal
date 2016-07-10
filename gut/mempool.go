package gut

import (
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
	if p.shared {
		ptr := atomic.AddUint64(&p.allocated, uint64(n))
		block = unsafe.Pointer(&p.mem[ptr-uint64(n)])
	} else {
		block = unsafe.Pointer(&p.mem[p.allocated])
		p.allocated += uint64(n)
	}
	return
}
