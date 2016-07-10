package main

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

type treeNode struct {
	left, right *treeNode
	value       int
}

const bytesPerNode = 8 * 3

type memPool struct {
	mem       []byte
	allocated uint
}

func newMemPool(size uint) *memPool {
	p := &memPool{}
	var err error
	p.mem, err = syscall.Mmap(-1, 0, int(size), syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		panic(err)
	}
	return p
}

func (p *memPool) Done() {
	syscall.Munmap(p.mem)
	p.mem = nil
}

func (p *memPool) Alloc(n uint) (block unsafe.Pointer) {
	block = unsafe.Pointer(&p.mem[p.allocated])
	p.allocated += (n + 7) & ^uint(7)
	return
}

func depthToNodeCount(depth int) int {
	return (1<<uint(depth))*2 - 1
}

var nodes int

func buildTreeNode(p *memPool, depth int, value int) (n *treeNode) {
	nodes++
	n = (*treeNode)(p.Alloc(bytesPerNode))
	//n = &treeNode{}
	n.value = value
	if depth > 0 {
		n.left = buildTreeNode(p, depth-1, value-1)
		n.right = buildTreeNode(p, depth-1, value-1)
	}
	return
}

func main() {
	var n treeNode
	fmt.Printf("%d\n", unsafe.Sizeof(n))
	st := time.Now()
	p := newMemPool(uint(depthToNodeCount(22)*bytesPerNode) * 2)
	buildTreeNode(p, 22, 1000000)
	fmt.Printf("nodes: %d. time: %v\n", nodes, time.Since(st))
	p.Done()
}
