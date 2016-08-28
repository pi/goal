package pipe

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/pi/goal/th"
)

type connConstructor func(bufSize int) (net.Conn, net.Conn)

func clientServerTestHelper(t *testing.T, ctr connConstructor) {
	wg := sync.WaitGroup{}

	st := time.Now()
	sm := th.TotalAlloc()
	for i := 0; i < kNPIPES; i++ {
		srvConn, cliConn := ctr(kBS)

		wg.Add(2)

		go func(c net.Conn) {
			m := make([]byte, kS)
			rm := make([]byte, kS)
			for i := 0; i < kN; i++ {
				c.Write(m)
				c.Read(rm)
			}
			wg.Done()
		}(cliConn)

		go func(c net.Conn) {
			m := make([]byte, kS)
			for i := 0; i < kN; i++ {
				c.Read(m)
				c.Write(m)
			}
			wg.Done()
		}(srvConn)
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v %s, mem: %s\n", elapsed, xferSpeed(kNPIPES*kN*2, elapsed), th.MemSince(sm))
}

func TestP2PPipeConnClientServer(t *testing.T) {
	clientServerTestHelper(t, P2PConn)
}

func TestMWPipeConnClientServer(t *testing.T) {
	clientServerTestHelper(t, MultiWriteConn)
}

func TestFullSyncPipeConnClientServer(t *testing.T) {
	clientServerTestHelper(t, Conn)
}

func TestP2PPipeConnProducerConsumer(t *testing.T) {
	wg := sync.WaitGroup{}

	st := time.Now()
	sm := th.TotalAlloc()
	for i := 0; i < kNPIPES; i++ {
		consumer, producer := P2PConn(kBS)
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

func TestMultiWritePipeConn(t *testing.T) {
	wg := sync.WaitGroup{}

	st := time.Now()
	sm := th.TotalAlloc()
	wg.Add(1)
	rc, wc := MultiWriteConn(kBS * kNPIPES)
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
