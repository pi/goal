package pipe

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"

	. "github.com/pi/goal/pipe/_testing"

	"github.com/pi/goal/th"
	"github.com/stretchr/testify/require"
)

type TestWriterInterface interface {
	io.Closer
	io.Writer
	io.ByteWriter
	io.ReaderFrom
	WriteAll(data ...[]byte) (int64, error)
	WriteAllWithContext(ctx context.Context, data ...[]byte) (int64, error)
	WriteWait(n int) error
	WriteWaitWithContext(ctx context.Context, n int) error
}

type TestReaderInterface interface {
	io.Closer
	io.Reader
	io.WriterTo
	io.ByteReader
	ReadWithContext(ctx context.Context, data []byte) (int, error)
	Skip(toSkip int) (int, error)
	SkipWithContext(ctx context.Context, toSkip int) (int, error)
	Peek(buf []byte) (int, error)
	Len() int
	ReadWait(n int) error
	ReadWaitWithContext(ctx context.Context, n int) error
}

func TestCap(t *testing.T) {
	ck := func(n int, must int) {
		p, _ := Pipe(n)
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
	r, w := Pipe(BS)
	m := make([]byte, S)
	rand.Read(m)
	rm := make([]byte, S)
	for i := 0; i < N; i++ {
		for {
			_, _, _, sz := w.loadHeader()
			if w.Cap()-sz >= S {
				break
			}
			r.Read(rm)
		}
		w.Write(m)
	}
	var p = make([]byte, 1)
	for {
		n, _ := r.Peek(p)
		if n > 0 {
			r.Read(rm)
		} else {
			break
		}
	}
}

func _TestReadWrite(t *testing.T) {
	wg := sync.WaitGroup{}
	r, w := Pipe(1024)
	const N = 2000
	wg.Add(2)
	go func() {
		buf := make([]byte, 300)
		for i := 0; i < N; i++ {
			m := genMsg(buf)
			for {
				_, _, _, sz := w.loadHeader()
				if w.Cap()-sz >= len(m)+5 {
					break
				}
				time.Sleep(time.Millisecond)
			}
			send(w, m)
		}
		wg.Done()
	}()
	go func() {
		rm := make([]byte, 300)
		for i := 0; i < N; i++ {
			recv(r, rm)
		}
		wg.Done()
	}()
	wg.Wait()
}

func TestClose(t *testing.T) {
	r, w := Pipe(10)
	require.False(t, r.IsClosed())
	require.False(t, w.IsClosed())
	r.Close()
	require.True(t, r.IsClosed())
	require.True(t, w.IsClosed())
	_, err := w.Write([]byte("t"))
	require.Equal(t, err, io.EOF)
	_, err = r.Read(make([]byte, 10))
	require.Equal(t, err, io.EOF)

	r, w = Pipe(10)
	require.False(t, r.IsClosed())
	require.False(t, w.IsClosed())

	wg := sync.WaitGroup{}
	wg.Add(1)
	w.Write([]byte("t"))
	w.Close()
	{
		var (
			err error
			s   []byte
			nw  int
			nr  int
		)

		s = make([]byte, 2)
		nr, err = r.Read(s)
		require.Equal(t, string(s[:nr]), "t")
		require.Equal(t, io.EOF, err)
		nw, err = w.Write([]byte("t"))
		require.EqualValues(t, nw, 0)
		require.Equal(t, io.EOF, err)
	}

	r, w = Pipe(10)
	type res struct {
		m   []byte
		n   int
		err error
	}
	c := make(chan res)
	go func() {
		m := make([]byte, 10)
		n, err := r.Read(m)
		c <- res{m, n, err}
	}()
	go func() {
		w.Write([]byte{0xFE})
		w.Close()
	}()
	rv := <-c
	require.EqualValues(t, rv.n, 1)
	require.EqualValues(t, rv.m[0], 0xFE)
	require.Equal(t, io.EOF, rv.err)

	r, w = SyncWritePipe(100)
	m := make([]byte, 8)
	for i := 0; i < 1000; i++ {
		go func() {
			for {
				if _, err := w.Write(m); err != nil {
					break
				}
			}
		}()
	}
	go func() {
		rm := make([]byte, 8)
		for {
			if _, err := r.Read(rm); err != nil {
				break
			}
		}
	}()
	time.Sleep(500 * time.Millisecond)
	r.Close()
	w.Close()
}

func TestDeadline(t *testing.T) {
	checkTimeoutError := func(e error) {
		require.Error(t, e)
		te, ok := e.(net.Error)
		require.True(t, ok)
		require.True(t, te.Timeout())
	}
	r, w := Pipe(BS)

	r.setDeadline(time.Now().Add(time.Second))
	w.setDeadline(time.Now().Add(time.Second))
	b, err := r.ReadByte()
	checkTimeoutError(err)
	require.EqualValues(t, 0, b)

	r.setDeadline(time.Time{})
	w.setDeadline(time.Time{})
	err = w.WriteByte(1)
	require.NoError(t, err)
	b, err = r.ReadByte()
	require.NoError(t, err)
	require.EqualValues(t, 1, b)
}

func psum(data []byte) (sum int) {
	for _, p := range data {
		sum += int(p)
	}
	return
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

func rbsend(w *Writer, data []byte) int {
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
		panic("ringbuf.WriteChunks fail")
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

func recv(r *Reader, pkt []byte) []byte {
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

func _TestMultiWriteMux(t *testing.T) {
	kNPIPES := NPIPES / 10
	wl := sync.Mutex{}
	st := time.Now()
	wg := &sync.WaitGroup{}
	m := make([]byte, S)
	rand.Read(m)
	r, w := Pipe(BS * kNPIPES)
	println(N * kNPIPES)
	sm := th.TotalAlloc()
	wg.Add(1)
	go func() {
		rm := make([]byte, S)
		for i := 0; i < N*kNPIPES; i++ {
			r.Read(rm)
		}
		wg.Done()
	}()
	for i := 0; i < kNPIPES; i++ {
		wg.Add(1)
		go func() {
			for i := 0; i < N; i++ {
				wl.Lock()
				w.Write(m)
				wl.Unlock()
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, XferSpeed(N*uint64(kNPIPES), elapsed), th.MemSince(sm))
}

func TestReadFrom(t *testing.T) {
	for i := 0; i < 100; i++ {
		data := make([]byte, 8000)
		rand.Read(data)
		crc := crc32.ChecksumIEEE(data)

		r, w := SyncPipe(256)
		rc := make(chan []byte)
		go func() {
			w.ReadFrom(bytes.NewReader(data))
			w.Close()
		}()
		go func() {
			bw := bytes.NewBuffer(nil)
			bb := make([]byte, 5)
			for {
				n, err := r.Read(bb)
				if n > 0 {
					bw.Write(bb[:n])
				}
				if err == io.EOF {
					break
				} else if err != nil {
					t.Fatal(err)
				}
			}
			rc <- bw.Bytes()
		}()

		rdata := <-rc

		require.Equal(t, crc, crc32.ChecksumIEEE(rdata))
		require.True(t, bytes.Equal(data, rdata))
	}
}

func TestWriteTo(t *testing.T) {
	for i := 0; i < 100; i++ {
		data := make([]byte, 9000)
		rand.Read(data)
		crc := crc32.ChecksumIEEE(data)
		r, w := Pipe(8192)
		w.Write(make([]byte, 1011))
		r.Skip(1011)
		go func() {
			w.Write(data)
			w.Close()
		}()
		rc := make(chan []byte)
		go func() {
			bw := bytes.NewBuffer(nil)
			r.WriteTo(bw)
			rc <- bw.Bytes()
		}()
		rdata := <-rc
		require.Equal(t, crc, crc32.ChecksumIEEE(rdata))
		require.True(t, bytes.Equal(data, rdata))
	}
}

func multiWriteHelper(t *testing.T, NPIPES int, kN int) {
	st := time.Now()
	wg := &sync.WaitGroup{}
	m := make([]byte, S)
	rand.Read(m)
	r, w := SyncWritePipe(BS * NPIPES)
	sm := th.TotalAlloc()
	wg.Add(1)
	go func() {
		rm := make([]byte, S)
		for i := 0; i < N*NPIPES; i++ {
			r.Read(rm)
		}
		wg.Done()
	}()
	for i := 0; i < NPIPES; i++ {
		wg.Add(1)
		go func() {
			for i := 0; i < N; i++ {
				w.Write(m)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, XferSpeed(uint64(N)*uint64(NPIPES), elapsed), th.MemSince(sm))
}
func TestMultiWrite(t *testing.T) {
	multiWriteHelper(t, NPIPES/10, N)
}

func TestMultiWriteContext(t *testing.T) {
	st := time.Now()
	wg := &sync.WaitGroup{}
	m := make([]byte, S)
	rand.Read(m)
	const NPIPES = NPIPES / 10
	r, w := SyncWritePipe(BS * NPIPES)
	sm := th.TotalAlloc()
	wg.Add(1)
	go func() {
		ctx, cf := context.WithCancel(context.Background())
		rm := make([]byte, S)
		for i := 0; i < N*NPIPES; i++ {
			r.ReadWithContext(ctx, rm)
		}
		wg.Done()
		cf()
	}()
	for i := 0; i < NPIPES; i++ {
		wg.Add(1)
		go func() {
			ctx, cf := context.WithCancel(context.Background())
			for i := 0; i < N; i++ {
				w.WriteWithContext(ctx, m)
			}
			wg.Done()
			cf()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, XferSpeed(uint64(N)*uint64(NPIPES), elapsed), th.MemSince(sm))
}

func TestParallelThroughput(t *testing.T) {
	st := time.Now()
	wg := &sync.WaitGroup{}
	wg.Add(NPIPES * 2)
	m := make([]byte, S)
	rand.Read(m)

	sm := th.TotalAlloc()
	for i := 0; i < NPIPES; i++ {
		r, w := Pipe(BS)
		go func() {
			for i := 0; i < N; i++ {
				w.Write(m)
			}
			wg.Done()
		}()
		go func() {
			rm := make([]byte, S)
			for i := 0; i < N; i++ {
				r.Read(rm)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, XferSpeed(N*uint64(NPIPES), elapsed), th.MemSince(sm))
}

func TestParallelThroughputContext(t *testing.T) {
	st := time.Now()
	wg := &sync.WaitGroup{}
	wg.Add(NPIPES * 2)
	m := make([]byte, S)
	rand.Read(m)

	sm := th.TotalAlloc()
	for i := 0; i < NPIPES; i++ {
		r, w := Pipe(BS)
		go func() {
			ctx, cf := context.WithTimeout(context.Background(), time.Minute)
			for i := 0; i < N; i++ {
				w.WriteWithContext(ctx, m)
			}
			cf()
			wg.Done()
		}()
		go func() {
			ctx, cf := context.WithTimeout(context.Background(), time.Minute)
			rm := make([]byte, S)
			for i := 0; i < N; i++ {
				r.ReadWithContext(ctx, rm)
			}
			cf()
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, XferSpeed(N*uint64(NPIPES), elapsed), th.MemSince(sm))
}

func TestParallelThroughputWithWriteLock(t *testing.T) {
	st := time.Now()
	wg := &sync.WaitGroup{}
	wg.Add(NPIPES * 2)
	m := make([]byte, S)
	rand.Read(m)

	sm := th.TotalAlloc()
	for i := 0; i < NPIPES; i++ {
		r, w := SyncWritePipe(BS)
		go func() {
			for i := 0; i < N; i++ {
				w.Write(m)
			}
			wg.Done()
		}()
		go func() {
			rm := make([]byte, S)
			for i := 0; i < N; i++ {
				r.Read(rm)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, XferSpeed(N*uint64(NPIPES), elapsed), th.MemSince(sm))
}

func TestParallelThroughputWithWriteLockContext(t *testing.T) {
	st := time.Now()
	wg := &sync.WaitGroup{}
	wg.Add(NPIPES * 2)
	m := make([]byte, S)
	rand.Read(m)
	ctx := context.Background()

	sm := th.TotalAlloc()
	for i := 0; i < NPIPES; i++ {
		r, w := SyncWritePipe(BS)
		go func() {
			for i := 0; i < N; i++ {
				w.WriteWithContext(ctx, m)
			}
			wg.Done()
		}()
		go func() {
			rm := make([]byte, S)
			for i := 0; i < N; i++ {
				r.ReadWithContext(ctx, rm)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, XferSpeed(N*uint64(NPIPES), elapsed), th.MemSince(sm))
}

func TestInterface(t *testing.T) {
	it := func() (TestReaderInterface, TestWriterInterface) {
		r, w := Pipe(2)
		return r, w
	}
	it()
}
