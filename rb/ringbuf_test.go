package rb

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func psum(a []byte) (sum int) {
	for _, v := range a {
		sum += int(v)
	}
	return
}

func TestThroughput(t *testing.T) {
	const N = 1000000
	const S = 32
	const BS = S * 1000
	wg := sync.WaitGroup{}
	b := NewRingBuf(BS)
	b.ReadTimeout = time.Hour
	b.WriteTimeout = time.Hour
	m := make([]byte, S)
	for i, _ := range m {
		m[i] = byte(i)
	}
	rm := make([]byte, S)
	st := time.Now()
	rsum := 0
	wsum := 0
	wg.Add(2)
	go func() {
		for i := 0; i < N; i++ {
			b.Write(m)
			wsum += psum(m)
		}
		wg.Done()
	}()
	go func() {
		for nr := 0; nr < N; {
			/*n, _ :=*/ b.Read(rm)

			nr++
			rsum += psum(rm)
		}
		wg.Done()
	}()
	wg.Wait()
	require.EqualValues(t, wsum, rsum)
	timeSpent := time.Since(st)
	fmt.Printf("time spent: %v, %d MPS, rs: %v, ws: %v rwc:%d wwc:%d\n", timeSpent, int(N/timeSpent.Seconds()), b.rs, b.ws, b.rwc, b.wwc)

	rsum = 0
	wsum = 0
	st = time.Now()
	tb := make([]byte, S)
	for i := 0; i < N; i++ {
		copy(tb, m)
		//wsum += psum(m)
		copy(rm, tb)
		//rsum += psum(rm)
	}
	require.EqualValues(t, wsum, rsum)
	fmt.Printf("copy time: %v\n", time.Since(st))

	res := make(chan int, 2)
	c := make(chan []byte, 1000)
	wg = sync.WaitGroup{}
	wg.Add(2)
	go func() {
		sum := 0
		for i := 0; i < N; i++ {
			sum += psum(m)
			c <- m
		}
		res <- sum
		wg.Done()
	}()
	go func() {
		sum := 0
		r := make([]byte, S)
		for i := 0; i < N; i++ {
			r = <-c
			sum += psum(r)
		}
		res <- sum
		wg.Done()
	}()
	wg.Wait()
	require.EqualValues(t, <-res, <-res)
	fmt.Printf("channels time: %v\n", time.Since(st))
}

func TestRingBufferStrings(t *testing.T) {
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
