package pipe

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pi/goal/th"
)

func TestPipeConn(t *testing.T) {
	/*const kNPIPES = 1000
	const kN = 100000
	const kS = 32
	const kBS = kS * 1000*/

	var rst, wst uint64

	wg := sync.WaitGroup{}

	st := time.Now()
	sm := th.TotalAlloc()
	for i := 0; i < kNPIPES; i++ {
		srvConn, cliConn := Conn(kBS)

		wg.Add(2)

		go func(c net.Conn) {
			m := make([]byte, kS)
			rm := make([]byte, kS)
			for i := 0; i < kN; i++ {
				c.Write(m)
				c.Read(rm)
			}
			p := c.(*pipeConn)
			atomic.AddUint64(&rst, uint64(p.rp.rst))
			atomic.AddUint64(&wst, uint64(p.wp.wst))
			wg.Done()
		}(cliConn)

		go func(c net.Conn) {
			m := make([]byte, kS)
			for i := 0; i < kN; i++ {
				c.Read(m)
				c.Write(m)
			}
			p := c.(*pipeConn)
			atomic.AddUint64(&rst, uint64(p.rp.rst))
			atomic.AddUint64(&wst, uint64(p.wp.wst))
			wg.Done()
		}(srvConn)
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v %s, mem: %s\n", elapsed, xferSpeed(kNPIPES*kN*2, elapsed), th.MemSince(sm))
	fmt.Printf("rst:%v wst:%v\n", (time.Duration)(rst), (time.Duration)(wst))
}

func TestPipeConnProducerConsumer(t *testing.T) {
	/*const kNPIPES = 1000
	const kNMSG = 10000
	const kS = 32
	const kBS = kS * 1000*/

	wg := sync.WaitGroup{}

	st := time.Now()
	sm := th.TotalAlloc()
	for i := 0; i < kNPIPES; i++ {
		consumer, producer := Conn(kBS)
		wg.Add(2)

		go func(c net.Conn) {
			m := make([]byte, kS)
			for i := 0; i < kN; i++ {
				c.Read(m)
			}
			wg.Done()
		}(consumer)

		go func(c net.Conn) {
			m := make([]byte, kS)
			for i := 0; i < kN; i++ {
				c.Write(m)
			}
			wg.Done()
		}(producer)
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v %s, mem: %s\n", elapsed, xferSpeed(kNPIPES*kN, elapsed), th.MemSince(sm))
}

func TestPipeConnParallelWrite(t *testing.T) {
	wg := sync.WaitGroup{}

	st := time.Now()
	sm := th.TotalAlloc()
	wg.Add(1)
	rc, wc := Conn(kBS * kNPIPES)
	go func() {
		buf := make([]byte, kS)
		for i := 0; i < kN*kNPIPES; i++ {
			rc.Read(buf)
		}
		wg.Done()
	}()

	for i := 0; i < kNPIPES; i++ {
		wg.Add(1)
		go func() {
			m := make([]byte, kS)
			for i := 0; i < kN; i++ {
				wc.Write(m)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, xferSpeed(kN*kNPIPES, elapsed), th.MemSince(sm))
}
