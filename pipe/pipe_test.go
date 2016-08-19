package pipe

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/pi/goal/th"
	"github.com/stretchr/testify/require"
)

const kN = 100000
const kS = 32
const kBS = kS * 1000
const kNPIPES = 1000

func TestPipeCap(t *testing.T) {
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

func TestPipeRing(t *testing.T) {
	p := New(kBS)
	m := make([]byte, kS)
	rand.Read(m)
	const N = kN
	rm := make([]byte, kS)
	for i := 0; i < N; i++ {
		for (p.Cap() - p.ReadAvail()) < kS {
			recv(p, rm)
		}
		send(p, m)
	}
	for p.ReadAvail() > 0 {
		recv(p, rm)
	}
}

func TestPipeReadWrite(t *testing.T) {
	wg := sync.WaitGroup{}
	p := New(1024)
	const N = 2000
	wg.Add(2)
	go func() {
		buf := make([]byte, 300)
		for i := 0; i < N; i++ {
			send(p, genMsg(buf))
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

func TestPipeParallelWrite(t *testing.T) {
	const kNPIPES = 100
	wg := sync.WaitGroup{}
	p := New(kBS * kNPIPES)

	st := time.Now()
	sm := th.TotalAlloc()
	wg.Add(1)
	go func() {
		buf := make([]byte, kS)
		for i := 0; i < kN*kNPIPES; i++ {
			//recv(p, buf)
			p.Read(buf)
		}
		wg.Done()
	}()

	for i := 0; i < kNPIPES; i++ {
		wg.Add(1)
		go func() {
			//buf := make([]byte, 300)
			m := genMsg(nil)
			for i := 0; i < kN; i++ {
				//send(p, m) //genMsg(buf))
				p.Write(m)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, xferSpeed(kN*kNPIPES, elapsed), th.MemSince(sm))
}

func TestPipeOvercap(t *testing.T) {
	p := New(16)
	m := make([]byte, 32)
	_, err := rand.Read(m)
	if err != nil {
		panic("rand.Read error " + err.Error())
	}
	rm := make([]byte, 32)
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		readAll(p, rm)
		wg.Done()
	}()
	go func() {
		p.Write(m)
		wg.Done()
	}()
	wg.Wait()
	if crc32.ChecksumIEEE(m) != crc32.ChecksumIEEE(rm) {
		t.Fatal("crc mismatch")
	}
}

func TestPipeClose(t *testing.T) {
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
		var err error
		var s []byte
		var nw int

		s = make([]byte, 2)
		n, err := p.Read(s)
		require.Equal(t, string(s[:n]), "t")
		require.Equal(t, io.EOF, err)
		nw, err = p.Write(s)
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
	pkt := make([]byte, 1+len(data)+4)
	pkt[0] = byte(len(data))
	copy(pkt[1:1+len(data)], data)
	binary.LittleEndian.PutUint32(pkt[1+len(data):], crc32.ChecksumIEEE(data))
	writeAll(w, pkt)
	return len(pkt)
}

func genMsg(buf []byte) []byte {
	if buf == nil {
		buf = make([]byte, 32)
		rand.Read(buf)
		return buf
	}
	max := len(buf)
	if max > 200 {
		max = 200
	}
	l := rand.Int63()%int64(max) + 1
	rand.Read(buf[:l])
	return buf[:l]
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
	var cs [4]byte
	readAll(r, cs[:])
	if crc32.ChecksumIEEE(p) != binary.LittleEndian.Uint32(cs[:]) {
		panic("checksum error")
	}
	return p
}

func sendRaw(w io.Writer, buf []byte) {
	w.Write(buf)
}
func recvRaw(r io.Reader, buf []byte) {
	r.Read(buf)
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
		sendRaw(wb, m)

		rb.Seek(0, 0)
		recvRaw(rb, rm)
		if rm[0] != m[0] {
			panic("ouch")
		}
	}
}

func _TestBufferThroughput(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(kNPIPES)
	st := time.Now()
	sm := th.TotalAlloc()
	for i := 0; i < kNPIPES; i++ {
		go func() {
			bufferTestHelper()
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("%v, %s, mem:%s\n", elapsed, xferSpeed(kNPIPES*kN, elapsed), th.MemSince(sm))
}

func TestPipeParallelThroughput(t *testing.T) {
	st := time.Now()
	wg := &sync.WaitGroup{}
	wg.Add(kNPIPES * 2)
	m := make([]byte, kS)
	rand.Read(m)

	sm := th.TotalAlloc()
	for i := 0; i < kNPIPES; i++ {
		b := New(kBS)
		go func() {
			for i := 0; i < kN; i++ {
				b.Write(m)
			}
			wg.Done()
		}()
		go func() {
			rm := make([]byte, kS)
			for nr := 0; nr < kN; nr++ {
				b.Read(rm)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, xferSpeed(kN*uint64(kNPIPES), elapsed), th.MemSince(sm))
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
	wg.Add(kNPIPES)
	st := time.Now()
	sm := th.TotalAlloc()
	for i := 0; i < kNPIPES; i++ {
		testParallelChannelsHelper(wg)
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("parallel channels time: %v, %s, mem:%s\n", elapsed, xferSpeed(kN*kNPIPES, elapsed), th.MemSince(sm))
}
