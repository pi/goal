package rb

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/pi/goal/th"
	"github.com/stretchr/testify/require"
)

func TestRingBufferStrings(t *testing.T) {
	const N = 11
	b := NewRingBuf(N)
	bcap := b.Cap()
	ws := func(str string) {
		ra := int(b.ReadAvail())
		b.WriteString(str)
		require.EqualValues(t, ra+len(str), int(b.ReadAvail()))
	}
	trs := func(str string) {
		rs, err := b.ReadString(len(str))
		require.NoError(t, err)
		require.EqualValues(t, len(str), len(rs))
		require.EqualValues(t, str, rs)
	}
	ws("hello")
	ws("olleh")
	require.EqualValues(t, bcap-10, b.WriteAvail())
	trs("hello")
	require.EqualValues(t, 5, b.ReadAvail())
	ws("tst")
	require.EqualValues(t, 8, b.ReadAvail())
	trs("olleh")
	trs("tst")
	require.EqualValues(t, 0, b.ReadAvail())
}

func psum(a []byte) (sum int) {
	for _, v := range a {
		sum += int(v)
	}
	return
}

func pcheck(a []byte) int {
	return int(a[0]) + int(a[len(a)-1])
}

const kN = 10000
const kS = 32
const kBS = kS * 1000
const kN_GORO = 10000

func xferSpeed(nmsg uint64, elapsed time.Duration) string {
	return fmt.Sprintf("mps:%.2fM, %s/s", float64(nmsg)*1000.0/float64(elapsed.Nanoseconds()), th.MemString(uint64(float64(nmsg)*kS/elapsed.Seconds())))
}

func errstr(err error) string {
	if err == nil {
		return "<no error>"
	} else {
		return err.Error()
	}
}

func panicf(fmtStr string, args ...interface{}) {
	s := fmt.Sprintf(fmtStr, args...)
	panic(s)
}

func writeAll(w io.Writer, data []byte) {
	n, err := w.Write(data)
	if n != len(data) {
		panicf("can't write: %s", errstr(err))
	}
	if err != nil {
		panic(err)
	}
}

func send(w io.Writer, data []byte) {
	if len(data) > 255 {
		panicf("packet too long: %d", len(data))
	}
	var t [1]byte
	t[0] = byte(len(data))
	writeAll(w, t[:])
	writeAll(w, data)
	t[0] = byte(psum(data))
	writeAll(w, t[:])
}

func readAll(r io.Reader, buf []byte) {
	n, err := r.Read(buf)
	if n != len(buf) {
		panicf("unable to read data: %s", errstr(err))
	}
	if err != nil {
		panic(err)
	}
}

func recv(r io.Reader, pkt []byte) []byte {
	var t [1]byte
	readAll(r, t[:])
	pktLen := int(uint(t[0]))
	if pktLen > len(pkt) {
		panicf("pkt buffer size %d too small for packet len %d", len(pkt), pktLen)
	}
	p := pkt[0:pktLen]
	readAll(r, p)
	readAll(r, t[:])
	if byte(psum(p)) != t[0] {
		panic("checksum error")
	}
	return p
}

func bufferTestHelper() {
	m := make([]byte, kS)
	for i := 0; i < kS; i++ {
		m[i] = byte(i + 2)
	}
	tb := make([]byte, kS*5)
	rm := make([]byte, kS)
	wb := bytes.NewBuffer(tb)
	rb := bytes.NewReader(tb)
	for i := 0; i < kN; i++ {
		wb.Reset()
		send(wb, m)

		rb.Seek(0, 0)
		recv(rb, rm)
	}
}

func TestParallelBuffers(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(kN_GORO)
	st := time.Now()
	sm := th.TotalAlloc()
	for i := 0; i < kN_GORO; i++ {
		go func() {
			bufferTestHelper()
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("%v, %s, mem:%s\n", elapsed, xferSpeed(kN_GORO*kN, elapsed), th.MemSince(sm))
}

func TestSeqBufferThroughput(t *testing.T) {
	st := time.Now()
	sm := th.TotalAlloc()
	for i := 0; i < 100; i++ {
		bufferTestHelper()
	}
	elapsed := time.Since(st)
	fmt.Printf("%v, %s, mem:%s\n", elapsed, xferSpeed(kN*100, elapsed), th.MemSince(sm))
}

func TestSeqThroughput(t *testing.T) {
	b := NewRingBuf(kBS)
	m := make([]byte, kS)
	rand.Read(m)
	const N = kN * 100
	rm := make([]byte, kS)
	st := time.Now()
	sm := th.TotalAlloc()
	for i := 0; i < N; i++ {
		send(b, m)
		recv(b, rm)
	}
	elapsed := time.Since(st)
	fmt.Printf("%v, %s mem:%s\n", elapsed, xferSpeed(N, elapsed), th.MemSince(sm))
}

func TestRing(t *testing.T) {
	b := NewRingBuf(kBS)
	m := make([]byte, kS)
	rand.Read(m)
	const N = kN * 100
	rm := make([]byte, kS)
	st := time.Now()
	sm := th.TotalAlloc()
	for i := 0; i < N; i++ {
		for b.WriteAvail() < kS {
			recv(b, rm)
		}
		send(b, m)
	}
	for b.ReadAvail() > 0 {
		recv(b, rm)
	}
	elapsed := time.Since(st)
	fmt.Printf("%v, %s mem:%s\n", elapsed, xferSpeed(N, elapsed), th.MemSince(sm))
}

func testParallelThroughputHelper(t *testing.T, n_pipes int) {
	st := time.Now()
	wg := &sync.WaitGroup{}
	wg.Add(n_pipes * 2)
	m := make([]byte, kS)
	rand.Read(m)

	sm := th.TotalAlloc()
	for i := 0; i < n_pipes; i++ {
		b := NewRingBuf(kBS)
		go func() {
			for i := 0; i < kN; i++ {
				send(b, m)
			}
			wg.Done()
		}()
		go func() {
			rm := make([]byte, kS)
			for nr := 0; nr < kN; nr++ {
				recv(b, rm)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, xferSpeed(kN*uint64(n_pipes), elapsed), th.MemSince(sm))
}

func TestThroughput(t *testing.T) {
	testParallelThroughputHelper(t, 1)
}

func TestParallelThroughput(t *testing.T) {
	testParallelThroughputHelper(t, kN_GORO)
}

func testParallelChannelsHelper(dwg *sync.WaitGroup) {
	wg := sync.WaitGroup{}
	wg.Add(2)
	c := make(chan []byte, 10)
	m := make([]byte, kS)
	go func() {
		for i := 0; i < kN; i++ {
			c <- m
		}
		wg.Done()
	}()
	go func() {
		r := make([]byte, kS)
		for i := 0; i < kN; i++ {
			copy(r, <-c)
		}
		wg.Done()
	}()
	wg.Wait()
	dwg.Done()
}
func _TestParallelChannels(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(kN_GORO)
	st := time.Now()
	sm := th.TotalAlloc()
	for i := 0; i < kN_GORO; i++ {
		testParallelChannelsHelper(wg)
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("parallel channels time: %v, %s, mem:%s\n", elapsed, xferSpeed(kN*kN_GORO, elapsed), th.MemSince(sm))
}
