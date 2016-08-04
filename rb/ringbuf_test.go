package rb

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pi/goal/th"
	"github.com/stretchr/testify/require"
)

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
const kBS = kS * 10000
const kN_GORO = 10000

func testThroughputHelper(mwg *sync.WaitGroup) {
	wg := sync.WaitGroup{}
	b := NewRingBuf(kBS)
	m := make([]byte, kS)
	for i, _ := range m {
		m[i] = byte(i)
	}
	rm := make([]byte, kS)
	wg.Add(2)
	go func() {
		for i := 0; i < kN; i++ {
			b.Write(m)
		}
		wg.Done()
	}()
	go func() {
		for nr := 0; nr < kN; {
			b.ReadS(rm)
			nr++
		}
		wg.Done()
	}()
	wg.Wait()
	mwg.Done()
}

func xferSpeed(nmsg uint64, elapsed time.Duration) string {
	return fmt.Sprintf("%dMPS, %s/s", int64(float64(nmsg)/elapsed.Seconds()), th.MemString(uint64(float64(nmsg)*kS/elapsed.Seconds())))
}

func TestParallelThroughput(t *testing.T) {
	st := time.Now()
	sm := th.TotalAlloc()
	wg := &sync.WaitGroup{}
	wg.Add(kN_GORO)
	for i := 0; i < kN_GORO; i++ {
		testThroughputHelper(wg)
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, xferSpeed(kN*kN_GORO, elapsed), th.MemSince(sm))
}

func testParallelBuffersHelper(wg *sync.WaitGroup) {
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
		wb.Write(m)

		rb.Seek(0, 0)
		rb.Read(rm)
		if rm[0] != m[0] {
			panic("mismatch")
		}
	}
	wg.Done()
}
func TestParallelBuffers(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(kN_GORO)
	st := time.Now()
	sm := th.CurAlloc()
	for i := 0; i < kN_GORO; i++ {
		go testParallelBuffersHelper(wg)
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("parallel buffers time: %v, %s, mem:%s\n", elapsed, xferSpeed(kN_GORO*kN, elapsed), th.MemSince(sm))
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
func TestParallelChannels(t *testing.T) {
	wg := &sync.WaitGroup{}
	wg.Add(kN_GORO)
	st := time.Now()
	sm := th.CurAlloc()
	for i := 0; i < kN_GORO; i++ {
		testParallelChannelsHelper(wg)
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("parallel channels time: %v, %s, mem:%s\n", elapsed, xferSpeed(kN*kN_GORO, elapsed), th.MemSince(sm))
}

func TestRwSpeed(t *testing.T) {
	b := NewRingBuf(kBS)
	m := make([]byte, kS)
	const kN = kN * 100
	rm := make([]byte, kS)
	st := time.Now()
	for i := 0; i < kN; i++ {
		b.Write(m)
		b.ReadS(rm)
	}
	elapsed := time.Since(st)
	fmt.Printf("rw time: %v, %s\n", elapsed, xferSpeed(kN, elapsed))
}

func _TestRingBufferStrings(t *testing.T) {
	const N = 11
	b := NewRingBuf(N)
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
	require.EqualValues(t, N-10, b.WriteAvail())
	trs("hello")
	require.EqualValues(t, 5, b.ReadAvail())
	ws("tst")
	require.EqualValues(t, 8, b.ReadAvail())
	trs("olleh")
	trs("tst")
	require.EqualValues(t, 0, b.ReadAvail())
}
