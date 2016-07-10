// +build windows

package md

import (
	"syscall"
	"unsafe"
)

var (
	kernelDLL     = syscall.NewLazyDLL("kernel32.dll")
	_VirtualAlloc = kernelDLL.NewProc("VirtualAlloc")
	_VirtualFree  = kernelDLL.NewProc("VirtualFree")
)

const (
	_MEM_COMMIT   = 0x1000
	_MEM_RESERVE  = 0x2000
	_MEM_DECOMMIT = 0x4000
	_MEM_RELEASE  = 0x8000

	_PAGE_READWRITE = 0x0004
	_PAGE_NOACCESS  = 0x0001
)

func VAlloc(size uint) ([]byte, error) {
	r0, _, e0 := syscall.Syscall6(_VirtualAlloc.Addr(), 4, 0, uintptr(size), _MEM_RESERVE|_MEM_COMMIT, _PAGE_READWRITE, 0, 0)
	if r0 == 0 {
		return nil, syscall.Errno(e0)
	}

	mem := struct {
		addr uintptr
		len  uint
		cap  uint
	}{r0, size, size}
	return *(*[]byte)(unsafe.Pointer(&mem)), nil
}

func VFree(mem []byte) error {
	r0, _, e0 := syscall.Syscall6(_VirtualFree.Addr(), 4, uintptr(unsafe.Pointer(&mem[0])), 0, _MEM_RELEASE, 0, 0, 0)
	if r0 == 0 {
		return syscall.Errno(e0)
	}
	return nil
}
