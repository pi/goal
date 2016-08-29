package pipe

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	. "./_testing"

	"github.com/pi/goal/th"
)

type connConstructor func(bufSize int) (net.Conn, net.Conn)

func clientServerTestHelper(t *testing.T, ctr connConstructor) {
	wg := sync.WaitGroup{}

	st := time.Now()
	sm := th.TotalAlloc()
	for i := 0; i < NPIPES; i++ {
		srvConn, cliConn := ctr(BS)

		wg.Add(2)

		go func(c net.Conn) {
			m := make([]byte, S)
			rm := make([]byte, S)
			for i := 0; i < N; i++ {
				c.Write(m)
				c.Read(rm)
			}
			wg.Done()
		}(cliConn)

		go func(c net.Conn) {
			m := make([]byte, S)
			for i := 0; i < N; i++ {
				c.Read(m)
				c.Write(m)
			}
			wg.Done()
		}(srvConn)
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v %s, mem: %s\n", elapsed, XferSpeed(NPIPES*N*2, elapsed), th.MemSince(sm))
}

func TestP2PPipeConnClientServer(t *testing.T) {
	clientServerTestHelper(t, Conn)
}

func TestMWPipeConnClientServer(t *testing.T) {
	clientServerTestHelper(t, SyncWriteConn)
}

func TestFullSyncPipeConnClientServer(t *testing.T) {
	clientServerTestHelper(t, SyncConn)
}

func TestP2PPipeConnProducerConsumer(t *testing.T) {
	wg := sync.WaitGroup{}

	st := time.Now()
	sm := th.TotalAlloc()
	for i := 0; i < NPIPES; i++ {
		consumer, producer := Conn(BS)
		wg.Add(2)

		go func(c net.Conn) {
			m := make([]byte, S)
			for i := 0; i < N; i++ {
				c.Read(m)
			}
			wg.Done()
		}(consumer)

		go func(c net.Conn) {
			m := make([]byte, S)
			for i := 0; i < N; i++ {
				c.Write(m)
			}
			wg.Done()
		}(producer)
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v %s, mem: %s\n", elapsed, XferSpeed(NPIPES*N, elapsed), th.MemSince(sm))
}

func TestMultiWritePipeConn(t *testing.T) {
	wg := sync.WaitGroup{}

	st := time.Now()
	sm := th.TotalAlloc()
	wg.Add(1)
	rc, wc := SyncWriteConn(BS * NPIPES)
	go func() {
		buf := make([]byte, S)
		for i := 0; i < N*NPIPES; i++ {
			rc.Read(buf)
		}
		wg.Done()
	}()

	for i := 0; i < NPIPES; i++ {
		wg.Add(1)
		go func() {
			m := make([]byte, S)
			for i := 0; i < N; i++ {
				wc.Write(m)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, XferSpeed(N*NPIPES, elapsed), th.MemSince(sm))
}
