package rb

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"hash/crc32"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/pi/goal/th"
	"github.com/stretchr/testify/require"
)

func TestOvercap(t *testing.T) {
	b := NewRingBuf(16)
	m := make([]byte, 32)
	_, err := rand.Read(m)
	if err != nil {
		panic("rand.Read error " + err.Error())
	}
	rm := make([]byte, 32)
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		readAll(b, rm)
		wg.Done()
	}()
	go func() {
		b.Write(m)
		wg.Done()
	}()
	wg.Wait()
	if crc32.ChecksumIEEE(m) != crc32.ChecksumIEEE(rm) {
		t.Fatal("crc mismatch")
	}
}

func TestRingBufferCap(t *testing.T) {
	ck := func(n uint32, must uint32) {
		b := NewRingBuf(n)
		require.EqualValues(t, must, b.Cap())
	}
	ck(0, 2)
	ck(1, 2)
	ck(2, 2)
	ck(3, 4)
	ck(4, 4)
	ck(5, 8)
	ck(16, 16)
	ck(31, 32)
	ck(32, 32)
}

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

var _ = crc32.ChecksumIEEE

func psum(data []byte) (sum int) {
	for _, b := range data {
		sum += int(b)
	}
	return
}

const kN = 10000
const kS = 32
const kBS = kS * 1000
const kN_PIPES = 10000

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

func rbsend(w *RingBuf, data []byte) {
	if len(data) > 255 {
		panicf("packet too long: %d", len(data))
	}
	var l, cs [1]byte
	l[0] = byte(len(data))
	cs[0] = byte(psum(data))
	nw, err := w.WriteChunks(l[:], data, cs[:])
	if nw != len(data)+2 {
		panic("RingBuf.WriteChunks fail")
	}
	if err != nil {
		panic(err)
	}
}

func readAll(r io.Reader, buf []byte) {
	n, err := r.Read(buf)
	if n != len(buf) {
		panicf("short read (%d of %d): %s", n, len(buf), errstr(err))
	}
	if err != nil {
		panic(err)
	}
}

func recv(r io.Reader, pkt []byte) []byte {
	var a [1]byte
	t := a[:]
	readAll(r, t)
	pktLen := int(uint(t[0]))
	if pktLen > len(pkt) {
		panicf("pkt buffer size %d too small for packet len %d", len(pkt), pktLen)
	}
	p := pkt[:pktLen]
	readAll(r, p)
	readAll(r, t)
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
	tb := make([]byte, kBS)
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

func TestBufferThroughput(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(kN_PIPES)
	st := time.Now()
	sm := th.TotalAlloc()
	for i := 0; i < kN_PIPES; i++ {
		go func() {
			bufferTestHelper()
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("%v, %s, mem:%s\n", elapsed, xferSpeed(kN_PIPES*kN, elapsed), th.MemSince(sm))
}

func TestRing(t *testing.T) {
	b := NewRingBuf(kBS)
	m := make([]byte, kS)
	rand.Read(m)
	const N = kN * 100
	rm := make([]byte, kS)
	for i := 0; i < N; i++ {
		for b.WriteAvail() < kS {
			recv(b, rm)
		}
		send(b, m)
	}
	for b.ReadAvail() > 0 {
		recv(b, rm)
	}
}

func TestClosed(t *testing.T) {
	b := NewRingBuf(10)
	require.False(t, b.IsClosed())
	b.Close()
	require.True(t, b.IsClosed())
	_, err := b.WriteString("t")
	require.Equal(t, err, io.ErrClosedPipe)
	_, err = b.ReadString(10)
	require.Equal(t, err, io.EOF)

	b.Reopen()
	require.False(t, b.IsClosed())

	wg := sync.WaitGroup{}
	wg.Add(1)
	b.WriteString("t")
	b.Close()
	{
		var err error
		var s string
		var nw int

		s, err = b.ReadString(2)
		require.Equal(t, s, "t")
		require.Equal(t, io.EOF, err)
		nw, err = b.WriteString("t")
		require.EqualValues(t, nw, 0)
		require.Equal(t, io.ErrClosedPipe, err)
	}

	b.Reopen()
	type res struct {
		m   []byte
		n   int
		err error
	}
	c := make(chan res)
	go func() {
		m := make([]byte, 10)
		n, err := b.Read(m)
		c <- res{m, n, err}
	}()
	go func() {
		b.Write([]byte{0xFE})
		b.Close()
	}()
	r := <-c
	require.EqualValues(t, r.n, 1)
	require.EqualValues(t, r.m[0], 0xFE)
	require.Equal(t, io.EOF, r.err)
}

func TestParallelThroughput(t *testing.T) {
	st := time.Now()
	wg := &sync.WaitGroup{}
	wg.Add(kN_PIPES * 2)
	m := make([]byte, kS)
	rand.Read(m)

	sm := th.TotalAlloc()
	for i := 0; i < kN_PIPES; i++ {
		b := NewRingBuf(kBS)
		go func() {
			for i := 0; i < kN; i++ {
				//b.Write(m) //
				rbsend(b, m)
			}
			wg.Done()
		}()
		go func() {
			rm := make([]byte, kS)
			for nr := 0; nr < kN; nr++ {
				//b.Read(rm)
				recv(b, rm)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, xferSpeed(kN*uint64(kN_PIPES), elapsed), th.MemSince(sm))
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
	wg.Add(kN_PIPES)
	st := time.Now()
	sm := th.TotalAlloc()
	for i := 0; i < kN_PIPES; i++ {
		testParallelChannelsHelper(wg)
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("parallel channels time: %v, %s, mem:%s\n", elapsed, xferSpeed(kN*kN_PIPES, elapsed), th.MemSince(sm))
}
