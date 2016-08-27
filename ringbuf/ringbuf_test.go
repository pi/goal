package ringbuf

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/pi/goal/th"
	"github.com/stretchr/testify/require"
)

func TestCap(t *testing.T) {
	ck := func(n int, must int) {
		p := New(n)
		require.EqualValues(t, must, p.Cap())
	}
	ck(0, defaultBufferSize)
	ck(1, minBufferSize)
	ck(2, minBufferSize)
	ck(3, minBufferSize)
	ck(4, minBufferSize)
	ck(5, minBufferSize)
	ck(16, 16)
	ck(31, 32)
	ck(32, 32)
}

func TestRing(t *testing.T) {
	p := New(kBS)
	m := make([]byte, kS)
	rand.Read(m)
	const N = kN
	rm := make([]byte, kS)
	for i := 0; i < N; i++ {
		for p.WriteAvail() < kS {
			p.Read(rm)
		}
		p.Write(m)
	}
	for p.ReadAvail() > 0 {
		p.Read(rm)
	}
}

func _TestReadWrite(t *testing.T) {
	wg := sync.WaitGroup{}
	p := New(1024)
	const N = 2000
	wg.Add(2)
	go func() {
		buf := make([]byte, 300)
		for i := 0; i < N; i++ {
			m := genMsg(buf)
			for p.WriteAvail() < len(m)+5 {
				time.Sleep(time.Millisecond)
			}
			send(p, m)
		}
		wg.Done()
	}()
	go func() {
		rm := make([]byte, 300)
		for i := 0; i < N; i++ {
			recv(p, rm)
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestClose(t *testing.T) {
	p := New(10)
	require.False(t, p.IsClosed())
	p.Close()
	require.True(t, p.IsClosed())
	_, err := p.Write([]byte("t"))
	require.Equal(t, err, io.EOF)
	_, err = p.Read(make([]byte, 10))
	require.Equal(t, err, io.EOF)

	p.Reopen()
	require.False(t, p.IsClosed())

	wg := sync.WaitGroup{}
	wg.Add(1)
	p.Write([]byte("t"))
	p.Close()
	{
		var (
			err error
			s   []byte
			nw  int
			nr  int
		)

		s = make([]byte, 2)
		nr, err = p.Read(s)
		require.Equal(t, string(s[:nr]), "t")
		require.Equal(t, io.EOF, err)
		nw, err = p.Write([]byte("t"))
		require.EqualValues(t, nw, 0)
		require.Equal(t, io.EOF, err)
	}

	p.Reopen()
	type res struct {
		m   []byte
		n   int
		err error
	}
	c := make(chan res)
	go func() {
		m := make([]byte, 10)
		n, err := p.Read(m)
		c <- res{m, n, err}
	}()
	go func() {
		p.Write([]byte{0xFE})
		p.Close()
	}()
	r := <-c
	require.EqualValues(t, r.n, 1)
	require.EqualValues(t, r.m[0], 0xFE)
	require.Equal(t, io.EOF, r.err)
}

func psum(data []byte) (sum int) {
	for _, p := range data {
		sum += int(p)
	}
	return
}

const kN = 100000
const kS = 32
const kBS = kS * 1000
const kN_PIPES = 1000

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

func send(w io.Writer, data []byte) int {
	if len(data) > 255-1-4 {
		panicf("packet too long: %d", len(data))
	}
	var l [1]byte
	l[0] = byte(len(data))
	writeAll(w, l[:])
	writeAll(w, data)
	var cs [4]byte
	binary.LittleEndian.PutUint32(cs[:], crc32.ChecksumIEEE(data))
	writeAll(w, cs[:])
	return 1 + len(data) + 4
}

func genMsg(buf []byte) []byte {
	max := len(buf)
	if max > 200 {
		max = 200
	}
	l := rand.Int63()%int64(max) + 1
	rand.Read(buf[:l])
	return buf[:l]
}

func rbsend(w *RingBuf, data []byte) int {
	if len(data) > 255-1-4 {
		panicf("packet too long: %d", len(data))
	}
	var (
		l  [1]byte
		cs [4]byte
	)
	l[0] = byte(len(data))
	binary.LittleEndian.PutUint32(cs[:], crc32.ChecksumIEEE(data))
	nw, err := w.WriteAll(l[:], data, cs[:])
	if nw != int64(len(data)+len(l)+len(cs)) {
		panic("RingBuf.WriteChunks fail")
	}
	if err != nil {
		panic(err)
	}
	return int(nw)
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

func recv(r *RingBuf, pkt []byte) []byte {
	var a [1]byte

	t := a[:]
	readAll(r, t)
	pktLen := int(uint(t[0]))
	if pktLen > len(pkt) {
		panicf("pkt buffer size %d too small for packet len %d", len(pkt), pktLen)
	}
	for r.ReadAvail() < pktLen+4 {
		time.Sleep(time.Millisecond)
	}
	p := pkt[:pktLen]
	readAll(r, p)
	var cs [4]byte
	readAll(r, cs[:])
	if crc32.ChecksumIEEE(p) != binary.LittleEndian.Uint32(cs[:]) {
		panic("checksum error")
	}
	return p
}

func _TestMultiWriteMux(t *testing.T) {
	const kN_PIPES = kN_PIPES / 10
	wl := sync.Mutex{}
	st := time.Now()
	wg := &sync.WaitGroup{}
	m := make([]byte, kS)
	rand.Read(m)
	b := New(kBS * kN_PIPES)
	println(kN * kN_PIPES)
	sm := th.TotalAlloc()
	wg.Add(1)
	go func() {
		rm := make([]byte, kS)
		for i := 0; i < kN*kN_PIPES; i++ {
			b.Read(rm)
		}
		wg.Done()
	}()
	for i := 0; i < kN_PIPES; i++ {
		wg.Add(1)
		go func() {
			for i := 0; i < kN; i++ {
				wl.Lock()
				b.Write(m)
				wl.Unlock()
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, xferSpeed(kN*uint64(kN_PIPES), elapsed), th.MemSince(sm))
}

func multiWriteHelper(t *testing.T, kN_PIPES int, kN int) {
	st := time.Now()
	wg := &sync.WaitGroup{}
	m := make([]byte, kS)
	rand.Read(m)
	b := New(kBS * kN_PIPES)
	sm := th.TotalAlloc()
	wg.Add(1)
	go func() {
		rm := make([]byte, kS)
		for i := 0; i < kN*kN_PIPES; i++ {
			b.Read(rm)
		}
		wg.Done()
	}()
	for i := 0; i < kN_PIPES; i++ {
		wg.Add(1)
		go func() {
			for i := 0; i < kN; i++ {
				b.WriteLock()
				b.Write(m)
				b.WriteUnlock()
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, xferSpeed(uint64(kN)*uint64(kN_PIPES), elapsed), th.MemSince(sm))
}
func TestMultiWrite(t *testing.T) {
	multiWriteHelper(t, kN_PIPES/10, kN)
}

func TestMultiWrite2(t *testing.T) {
	multiWriteHelper(t, runtime.GOMAXPROCS(0), kN*100)
}

func TestParallelThroughput(t *testing.T) {
	st := time.Now()
	wg := &sync.WaitGroup{}
	wg.Add(kN_PIPES * 2)
	m := make([]byte, kS)
	rand.Read(m)

	sm := th.TotalAlloc()
	for i := 0; i < kN_PIPES; i++ {
		b := New(kBS)
		go func() {
			for i := 0; i < kN; i++ {
				b.Write(m)
			}
			wg.Done()
		}()
		go func() {
			rm := make([]byte, kS)
			for i := 0; i < kN; i++ {
				b.Read(rm)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, xferSpeed(kN*uint64(kN_PIPES), elapsed), th.MemSince(sm))
}

func TestParallelThroughputContext(t *testing.T) {
	st := time.Now()
	wg := &sync.WaitGroup{}
	wg.Add(kN_PIPES * 2)
	m := make([]byte, kS)
	rand.Read(m)
	ctx, _ := context.WithTimeout(context.Background(), time.Minute)

	sm := th.TotalAlloc()
	for i := 0; i < kN_PIPES; i++ {
		b := New(kBS)
		go func() {
			for i := 0; i < kN; i++ {
				b.WriteContext(ctx, m)
			}
			wg.Done()
		}()
		go func() {
			rm := make([]byte, kS)
			for i := 0; i < kN; i++ {
				b.ReadContext(ctx, rm)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, xferSpeed(kN*uint64(kN_PIPES), elapsed), th.MemSince(sm))
}

func TestParallelThroughputWithWriteLock(t *testing.T) {
	st := time.Now()
	wg := &sync.WaitGroup{}
	wg.Add(kN_PIPES * 2)
	m := make([]byte, kS)
	rand.Read(m)

	sm := th.TotalAlloc()
	for i := 0; i < kN_PIPES; i++ {
		b := New(kBS)
		go func() {
			for i := 0; i < kN; i++ {
				b.WriteLock()
				b.Write(m)
				b.WriteUnlock()
			}
			wg.Done()
		}()
		go func() {
			rm := make([]byte, kS)
			for i := 0; i < kN; i++ {
				b.Read(rm)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, xferSpeed(kN*uint64(kN_PIPES), elapsed), th.MemSince(sm))
}
