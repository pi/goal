//
// +build unix
//
package gut

import (
	"syscall"
	"unsafe"
)

type MemPool struct {
	mem       []byte
	allocated uint
}

func NewMemPool(size uint) *MemPool {
	p := &MemPool{}
	var err error
	p.mem, err = syscall.Mmap(-1, 0, int(size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		panic(err)
	}
	return p
}

func (p *MemPool) Reset() {
	p.allocated = 0
}
func (p *MemPool) Done() {
	syscall.Munmap(p.mem)
	p.mem = nil
}
func (p *MemPool) Alloc(n uint) (block unsafe.Pointer) {
	n = (n + 7) & ^uint(7)
	block = unsafe.Pointer(&p.mem[p.allocated])
	p.allocated += n
	return
}
