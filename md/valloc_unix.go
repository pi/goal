// +build !windows

package md

import (
	"syscall"
)

func VAlloc(size uint) ([]byte, error) {
	return syscall.Mmap(-1, 0, int(size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
}

func VFree(mem []byte) error {
	return syscall.Munmap(mem)
}
