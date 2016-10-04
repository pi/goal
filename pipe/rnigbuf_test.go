package pipe

import (
	"fmt"
	"io"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/pi/goal/pipe/_testing"

	"github.com/pi/goal/th"
)

func (b *ringbuf) write(data []byte) (int, error) {
	toWrite := len(data)
	if toWrite == 0 {
		if b.IsClosed() {
			return 0, io.EOF
		} else {
			return 0, nil
		}
	}
	written := 0
	for written < toWrite {
		_, closed, head, sz := b.loadHeader()
		if closed {
			return written, io.EOF
		}
		nw := minInt(b.Cap()-sz, toWrite-written)
		if nw > 0 {
			writePos := (head + sz) & b.mask
			if writePos > b.Cap()-nw {
				// wrapped
				ll := b.Cap() - writePos
				copy(b.mem[writePos:], data[written:written+ll])
				copy(b.mem[:nw-ll], data[written+ll:written+nw])
			} else {
				copy(b.mem[writePos:writePos+nw], data[written:written+nw])
			}
			atomic.AddUint64(b.pbits, uint64(nw))
			written += nw
			notify(b.wsig)
		} else {
			if closed {
				return written, io.EOF
			}
			<-b.rsig
		}
	}
	return toWrite, nil
}

func (b *ringbuf) _read(data []byte) (int, error) {
	toRead := len(data)
	if toRead == 0 {
		if b.IsClosed() {
			return 0, io.EOF
		} else {
			return 0, nil
		}
	}
	readed := 0
	for readed < toRead {
		hs, closed, head, sz := b.loadHeader()
		nr := minInt(sz, toRead-readed)
		if nr > 0 {
			if head > b.Cap()-nr {
				// wrapped
				ll := b.Cap() - head
				copy(data[readed:readed+ll], b.mem[head:])
				copy(data[readed+ll:readed+nr], b.mem[:nr-ll])
			} else {
				copy(data[readed:readed+nr], b.mem[head:head+nr])
			}
			for {
				head = (head + nr) & b.mask
				sz -= nr
				if atomic.CompareAndSwapUint64(b.pbits, hs, (hs&headerFlagMask)|(uint64(head)<<32)|uint64(sz)) {
					break
				}
				runtime.Gosched()
				hs, closed, head, sz = b.loadHeader()
			}
			readed += nr
			if closed {
				return readed, io.EOF
			}
			notify(b.rsig)
		} else {
			if closed {
				return readed, io.EOF
			}
			<-b.wsig
		}
	}
	return readed, nil
}

func (r *ringbuf) read(data []byte) (int, error) {
	toRead := len(data)
	if toRead == 0 {
		if r.IsClosed() {
			return 0, io.EOF
		} else {
			return 0, nil
		}
	}
	if r.synchronized {
		err := r.lock()
		if err != nil {
			return 0, err
		}
	}
	readed := 0
	for readed < toRead {
		hs, closed, head, sz := r.loadHeader()
		nr := minInt(sz, toRead-readed)
		if nr > 0 {
			if head > r.Cap()-nr {
				// wrapped
				ll := r.Cap() - head
				copy(data[readed:readed+ll], r.mem[head:])
				copy(data[readed+ll:readed+nr], r.mem[:nr-ll])
			} else {
				copy(data[readed:readed+nr], r.mem[head:head+nr])
			}
			for {
				head = (head + nr) & r.mask
				sz -= nr
				if atomic.CompareAndSwapUint64(r.pbits, hs, (hs&headerFlagMask)|(uint64(head)<<32)|uint64(sz)) {
					break
				}
				runtime.Gosched()
				hs, closed, head, sz = r.loadHeader()
			}
			readed += nr
			if closed {
				if r.synchronized {
					r.unlock()
				}
				return readed, io.EOF
			}
			notify(r.rsig)
		} else {
			if closed {
				if r.synchronized {
					r.unlock()
				}
				return readed, io.EOF
			}
			<-r.wsig
		}
	}
	if r.synchronized {
		r.unlock()
	}
	return readed, nil
}

func TestRingbufParallelThroughput(t *testing.T) {
	st := time.Now()
	wg := &sync.WaitGroup{}
	wg.Add(NPIPES * 2)
	m := make([]byte, S)
	rand.Read(m)

	sm := th.TotalAlloc()
	for i := 0; i < NPIPES; i++ {
		r := &ringbuf{}
		r.init(BS, false)
		w := &ringbuf{}
		w.initFrom(r, false)
		go func() {
			for i := 0; i < N; i++ {
				w.write(m)
			}
			wg.Done()
		}()
		go func() {
			rm := make([]byte, S)
			for i := 0; i < N; i++ {
				r.read(rm)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	elapsed := time.Since(st)
	fmt.Printf("time spent: %v, %s, mem: %s\n", elapsed, XferSpeed(N*uint64(NPIPES), elapsed), th.MemSince(sm))
}
