/* The Computer Language Benchmarks Game
 * http://benchmarksgame.alioth.debian.org/
 *
 * contributed by The Go Authors.
 * based on C program by Kevin Carson
 * flag.Arg hack by Isaac Gouy
 *
 * 2013-04
 * modified by Jamil Djadala to use goroutines
 *
 * 2016-07
 * modified by Sergey Philippov to use syscall.Mmap
 */

package main

import (
	"flag"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"time"

	"gopkg.in/pi/goal/gut"
)

var n = 20

type Node struct {
	item        int
	left, right *Node
}

const bytesPerNode = 8 * 3

func nodesPerTree(depth int) uint {
	return (1<<uint(depth))*2 - 1
}

func bottomUpTree(p *gut.UnsafeMemoryPool, item, depth int) (n *Node) {
	n = (*Node)(p.Alloc(bytesPerNode))
	n.item = item

	if depth > 0 {
		n.left = bottomUpTree(p, 2*item-1, depth-1)
		n.right = bottomUpTree(p, 2*item, depth-1)
	} else {
		n.left = nil
		n.right = nil
	}
	return
}

func (n *Node) itemCheck() int {
	if n.left == nil {
		return n.item
	}
	return n.item + n.left.itemCheck() - n.right.itemCheck()
}

const minDepth = 4

func main() {
	startTime := time.Now()

	flag.Parse()
	if flag.NArg() > 0 {
		n, _ = strconv.Atoi(flag.Arg(0))
	}

	maxDepth := n
	if minDepth+2 > n {
		maxDepth = minDepth + 2
	}
	stretchDepth := maxDepth + 1

	ltpool, _ := gut.NewUnsafeMemoryPool(nodesPerTree(stretchDepth) * bytesPerNode)

	check_l := bottomUpTree(ltpool, 0, stretchDepth).itemCheck()
	fmt.Printf("stretch tree of depth %d\t check: %d\n", stretchDepth, check_l)
	ltpool.Reset()

	longLivedTree := bottomUpTree(ltpool, 0, maxDepth)

	var wg sync.WaitGroup
	result := make([]string, maxDepth+1)

	gate := make(chan bool, runtime.NumCPU())

	for depth_l := minDepth; depth_l <= maxDepth; depth_l += 2 {
		gate <- true
		wg.Add(1)
		go func(depth int, check int, r *string) {
			defer wg.Done()
			iterations := 1 << uint(maxDepth-depth+minDepth)
			check = 0

			pool, _ := gut.NewUnsafeMemoryPool(nodesPerTree(depth) * bytesPerNode)
			for i := 1; i <= iterations; i++ {
				pool.Reset()
				check += bottomUpTree(pool, i, depth).itemCheck()
				pool.Reset()
				check += bottomUpTree(pool, -i, depth).itemCheck()
			}
			pool.Done()
			*r = fmt.Sprintf("%d\t trees of depth %d\t check: %d", iterations*2, depth, check)
			<-gate
		}(depth_l, check_l, &result[depth_l])
	}
	wg.Wait()
	for depth := minDepth; depth <= maxDepth; depth += 2 {
		fmt.Println(result[depth])
	}
	fmt.Printf("long lived tree of depth %d\t check: %d\n", maxDepth, longLivedTree.itemCheck())
	ltpool.Done()

	fmt.Printf("time spent: %v\n", time.Since(startTime))
}
