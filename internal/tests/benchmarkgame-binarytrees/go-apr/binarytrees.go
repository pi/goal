/* The Computer Language Benchmarks Game
 * http://benchmarksgame.alioth.debian.org/
 *
 * contributed by The Go Authors.
 * based on C program by Kevin Carson
 * flag.Arg hack by Isaac Gouy
 *
 * 2013-04
 * modified by Jamil Djadala to use goroutines
 */

package main

/*
#cgo CFLAGS: -I/usr/include/apr-1.0 -I/usr/include/apr-1
#cgo LDFLAGS: -lapr-1

#include <apr_pools.h>

*/
import "C"

import (
	"flag"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"time"
)

var n = 20

type Node struct {
	item        int
	left, right *Node
}

func bottomUpTree(item, depth int, p *C.apr_pool_t) (n *Node) {
	n = (*Node)(C.apr_palloc(p, 3*8))
	n.item = item
	if depth > 0 {
		n.left = bottomUpTree(2*item-1, depth-1, p)
		n.right = bottomUpTree(2*item, depth-1, p)
	} else {
		n.left = nil
		n.right = nil
	}
	return n
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

	C.apr_initialize()

	flag.Parse()
	if flag.NArg() > 0 {
		n, _ = strconv.Atoi(flag.Arg(0))
	}

	maxDepth := n
	if minDepth+2 > n {
		maxDepth = minDepth + 2
	}
	stretchDepth := maxDepth + 1

	var lpool *C.apr_pool_t
	C.apr_pool_create_unmanaged_ex(&lpool, nil, nil)

	check_l := bottomUpTree(0, stretchDepth, lpool).itemCheck()
	fmt.Printf("stretch tree of depth %d\t check: %d\n", stretchDepth, check_l)

	C.apr_pool_clear(lpool)
	longLivedTree := bottomUpTree(0, maxDepth, lpool)

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

			var p *C.apr_pool_t
			C.apr_pool_create_unmanaged_ex(&p, nil, nil)
			for i := 1; i <= iterations; i++ {
				check += bottomUpTree(i, depth, p).itemCheck()
				check += bottomUpTree(-i, depth, p).itemCheck()
				C.apr_pool_clear(p)
			}
			C.apr_pool_destroy(p)
			*r = fmt.Sprintf("%d\t trees of depth %d\t check: %d", iterations*2, depth, check)
			<-gate
		}(depth_l, check_l, &result[depth_l])
	}
	wg.Wait()
	for depth := minDepth; depth <= maxDepth; depth += 2 {
		fmt.Println(result[depth])
	}
	fmt.Printf("long lived tree of depth %d\t check: %d\n", maxDepth, longLivedTree.itemCheck())

	fmt.Printf("time spent: %v\n", time.Since(startTime))
	C.apr_pool_destroy(lpool)
	C.apr_terminate()
}
