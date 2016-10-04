package pipe

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	. "github.com/pi/goal/pipe/_testing"

	"github.com/pi/goal/th"
	"github.com/stretchr/testify/require"
)

func TestConnDeadlines(t *testing.T) {
	c1, c2 := Conn(BS)
	c1.SetReadDeadline(time.Now().Add(-time.Second))
	c1.SetWriteDeadline(time.Now().Add(-time.Second))

	_, err := c1.Read(make([]byte, 1))
	checkTimeoutErr(t, err)
	_, err = c1.Write(make([]byte, 1))
	checkTimeoutErr(t, err)

	c2.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
	_, err = c2.Read(make([]byte, 1))
	checkTimeoutErr(t, err)

	c1.SetDeadline(time.Time{})
	c2.SetReadDeadline(time.Now().Add(time.Millisecond))
	n, err := c1.Write(make([]byte, 1))
	require.NoError(t, err)
	require.Equal(t, 1, n)
	n, err = c2.Read(make([]byte, 1))
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

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

func TestUnsyncPipeConnClientServer(t *testing.T) {
	clientServerTestHelper(t, UnsyncConn)
}

func TestSyncWritePipeConnClientServer(t *testing.T) {
	clientServerTestHelper(t, SyncWriteConn)
}

func TestFullSyncPipeConnClientServer(t *testing.T) {
	clientServerTestHelper(t, Conn)
}

func TestUnsyncPipeConnProducerConsumer(t *testing.T) {
	wg := sync.WaitGroup{}

	st := time.Now()
	sm := th.TotalAlloc()
	for i := 0; i < NPIPES; i++ {
		consumer, producer := UnsyncConn(BS)
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

func checkTimeoutErr(t *testing.T, err interface{}) {
	require.NotNil(t, err)
	ne, ok := err.(net.Error)
	require.True(t, ok)
	require.True(t, ne.Timeout())
}
