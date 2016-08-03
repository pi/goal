// +build windows

package md

import (
	"reflect"
	"syscall"
	"unsafe"
)

var (
	kernelDLL         = syscall.MustLoadDLL("kernel32.dll")
	_VirtualAllocAddr = kernelDLL.MustFindProc("VirtualAlloc").Addr()
	_VirtualFreeAddr  = kernelDLL.MustFindProc("VirtualFree").Addr()
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
	r0, _, e0 := syscall.Syscall6(_VirtualAllocAddr, 4, 0, uintptr(size), _MEM_RESERVE|_MEM_COMMIT, _PAGE_READWRITE, 0, 0)
	if r0 == 0 {
		return nil, syscall.Errno(e0)
	}

	mem := reflect.SliceHeader{
		Data: r0,
		Len:  int(size),
		Cap:  int(size)}
	return *(*[]byte)(unsafe.Pointer(&mem)), nil
}

func VFree(mem []byte) error {
	r0, _, e0 := syscall.Syscall(_VirtualFreeAddr, 3, uintptr(unsafe.Pointer(&mem[0])), 0, _MEM_RELEASE)
	if r0 == 0 {
		return syscall.Errno(e0)
	}
	return nil
}
