// The Computer Language Benchmarks Game
// http://benchmarksgame.alioth.debian.org/
//
// Go adaptation of binary-trees Rust #4 program
// Use semaphores to match the number of workers with the CPU count
//
// contributed by Marcel Ibes
//
// modified by Sergey Philippov to use unmanaged memory pools

package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"runtime"
	"sort"
	"strconv"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/pi/goal/gut"
)

type Tree struct {
	Left  *Tree
	Right *Tree
}

type Message struct {
	Pos  uint32
	Text string
}

type ByPos []Message

func (m ByPos) Len() int           { return len(m) }
func (m ByPos) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }
func (m ByPos) Less(i, j int) bool { return m[i].Pos < m[j].Pos }

func itemCheck(tree *Tree) uint32 {
	if tree.Left != nil && tree.Right != nil {
		return uint32(1) + itemCheck(tree.Right) + itemCheck(tree.Left)
	}

	return 1
}

func treeSize(depth uint32) uint {
	return ((1<<uint(depth))*2 - 1) * 16
}

func bottomUpTree(pool *gut.UnsafeMemoryPool, depth uint32) *Tree {
	tree := (*Tree)(pool.Alloc(16))
	if depth > uint32(0) {
		tree.Right = bottomUpTree(pool, depth-1)
		tree.Left = bottomUpTree(pool, depth-1)
	}
	return tree
}

func inner(depth, iterations uint32) string {
	chk := uint32(0)
	pool, _ := gut.NewUnsafeMemoryPool(treeSize(depth))
	for i := uint32(0); i < iterations; i++ {
		pool.Reset()
		a := bottomUpTree(pool, depth)
		chk += itemCheck(a)
	}
	return fmt.Sprintf("%d\t trees of depth %d\t check: %d", iterations, depth, chk)
}

const minDepth = uint32(4)

func main() {
	n := 21
	flag.Parse()
	if flag.NArg() > 0 {
		n, _ = strconv.Atoi(flag.Arg(0))
	}

	run(uint32(n))
}

func run(n uint32) {
	startTime := time.Now()

	cpuCount := runtime.NumCPU()
	sem := semaphore.NewWeighted(int64(cpuCount))

	maxDepth := n
	if minDepth+2 > n {
		maxDepth = minDepth + 2
	}

	depth := maxDepth + 1

	messages := make(chan Message, cpuCount)
	expected := uint32(2) // initialize with the 2 summary messages we're always outputting

	go func() {
		for halfDepth := minDepth / 2; halfDepth < maxDepth/2+1; halfDepth++ {
			depth := halfDepth * 2
			iterations := uint32(1 << (maxDepth - depth + minDepth))
			expected++

			func(d, i uint32) {
				if err := sem.Acquire(context.TODO(), 1); err == nil {
					go func() {
						defer sem.Release(1)
						messages <- Message{d, inner(d, i)}
					}()
				} else {
					panic(err)
				}
			}(depth, iterations)
		}

		if err := sem.Acquire(context.TODO(), 1); err == nil {
			go func() {
				defer sem.Release(1)
				pool, _ := gut.NewUnsafeMemoryPool(treeSize(depth))
				tree := bottomUpTree(pool, depth)
				messages <- Message{0,
					fmt.Sprintf("stretch tree of depth %d\t check: %d", depth, itemCheck(tree))}
				pool.Done()
			}()
		} else {
			panic(err)
		}

		if err := sem.Acquire(context.TODO(), 1); err == nil {
			go func() {
				defer sem.Release(1)
				pool, _ := gut.NewUnsafeMemoryPool(treeSize(maxDepth))
				longLivedTree := bottomUpTree(pool, maxDepth)
				messages <- Message{math.MaxUint32,
					fmt.Sprintf("long lived tree of depth %d\t check: %d", maxDepth, itemCheck(longLivedTree))}
				pool.Done()
			}()
		} else {
			panic(err)
		}
	}()

	var sortedMsg []Message
	for m := range messages {
		sortedMsg = append(sortedMsg, m)
		expected--
		if expected == 0 {
			close(messages)
		}
	}

	sort.Sort(ByPos(sortedMsg))
	for _, m := range sortedMsg {
		fmt.Println(m.Text)
	}

	fmt.Printf("time spent: %v\n", time.Since(startTime))
}
