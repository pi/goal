package ringbuf

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync/atomic"

	"github.com/pi/goal/gut"
)

var ErrOvercap = errors.New("Buffer overcap")

type RingBuf struct {
	bits uint64 // highest bit - close flag. next 31 bits: read pos, next bit - unused, next 31 bits: read avail
	mem  []byte
	mask int
	wsig chan struct{}
	rsig chan struct{}
	lsig chan struct{}
	wlck int32
	wq   int32
}

const low63bits = ^uint64(0) >> 1
const low31bits = ^uint32(0) >> 1

const closeFlag = low63bits + 1

const headerFlagMask = closeFlag

const defaultBufferSize = 32 * 1024
const minBufferSize = 8

func With(mem []byte) *RingBuf {
	if mem == nil {
		mem = make([]byte, defaultBufferSize)
	}
	max := len(mem)
	if (max & (max - 1)) != 0 {
		panic("Buffer size should be power of two")
	}
	if int(low31bits) < max-1 {
		panic("ringbuffer size is too large")
	}
	return &RingBuf{
		mem:  mem,
		mask: max - 1,
		rsig: make(chan struct{}, 1),
		wsig: make(chan struct{}, 1),
		lsig: make(chan struct{}, 1),
	}
}

func New(max int) *RingBuf {
	if max == 0 {
		max = defaultBufferSize
	} else if max < minBufferSize {
		max = minBufferSize
	} else if (max & (max - 1)) != 0 {
		// round up to power of two
		max = 1 << gut.BitLen(uint(max))
	}
	if int(low31bits) < max-1 {
		panic("ringbuffer size is too large")
	}

	return With(make([]byte, max))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (b *RingBuf) loadHeader() (hs uint64, closed bool, readPos int, readAvail int) {
	hs = atomic.LoadUint64(&b.bits)
	closed = (hs & closeFlag) != 0
	readPos = int((hs >> 32) & uint64(low31bits))
	readAvail = int(hs & uint64(low31bits))
	return
}

func (b *RingBuf) Close() error {
	for {
		hs := atomic.LoadUint64(&b.bits)
		if ((hs & closeFlag) != 0) || atomic.CompareAndSwapUint64(&b.bits, hs, hs|closeFlag) {
			if (hs & closeFlag) == 0 {
				notify(b.rsig)
				notify(b.wsig)
				notify(b.lsig)
			}
			return nil
		}
		runtime.Gosched()
	}
}

func (b *RingBuf) Reopen() {
	b.bits = 0 // reset head and size
	b.wlck = 0
	b.wq = 0
}

func (b *RingBuf) IsClosed() bool {
	return (atomic.LoadUint64(&b.bits) & closeFlag) != 0
}

func notify(c chan struct{}) {
	select {
	case c <- struct{}{}:
	default:
	}
}

func (b *RingBuf) Read(data []byte) (int, error) {
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
				if atomic.CompareAndSwapUint64(&b.bits, hs, (hs&headerFlagMask)|(uint64(head)<<32)|uint64(sz)) {
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
				notify(b.wsig) // wake other readers
				return readed, io.EOF
			}
			<-b.wsig
		}
	}
	return readed, nil
}

func (b *RingBuf) Write(data []byte) (int, error) {
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
			atomic.AddUint64(&b.bits, uint64(nw))
			written += nw
			notify(b.wsig)
		} else {
			if closed {
				notify(b.rsig) // wake other writers
				return written, io.EOF
			}
			<-b.rsig
		}
	}
	return toWrite, nil
}

func (b *RingBuf) ReadContext(ctx context.Context, data []byte) (int, error) {
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
				nhs := (hs & headerFlagMask) | (uint64(head) << 32) | uint64(sz)
				if atomic.CompareAndSwapUint64(&b.bits, hs, nhs) {
					break
				}
				runtime.Gosched()
				hs, closed, head, sz = b.loadHeader()
			}
			readed += nr
			notify(b.rsig)
			if closed {
				return readed, io.EOF
			}
		} else {
			if closed {
				notify(b.wsig)
				return readed, io.EOF
			}
			select {
			case <-b.wsig:
			case <-ctx.Done():
				return readed, ctx.Err()
			}
		}
	}
	return readed, nil
}

func (b *RingBuf) WriteContext(ctx context.Context, data []byte) (int, error) {
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
			atomic.AddUint64(&b.bits, uint64(nw))
			written += nw
			notify(b.wsig)
		} else {
			if closed {
				notify(b.rsig)
				return written, io.EOF
			}
			select {
			case <-b.rsig:
			case <-ctx.Done():
				return written, ctx.Err()
			}
		}
	}
	return toWrite, nil
}

func (b *RingBuf) Peek(data []byte) (int, error) {
	_, closed, head, nr := b.loadHeader()
	if nr > len(data) {
		nr = len(data)
	}
	if nr > 0 {
		if head > b.Cap()-nr {
			// wrapped
			ll := b.Cap() - head
			copy(data[:ll], b.mem[head:])
			copy(data[ll:nr], b.mem[:nr-ll])
		} else {
			copy(data, b.mem[head:head+nr])
		}
	}
	if closed {
		return nr, io.EOF
	}
	return nr, nil
}

func (b *RingBuf) Skip(toSkip int) (int, error) {
	if toSkip <= 0 {
		if b.IsClosed() {
			return 0, io.EOF
		}
		return 0, nil
	}
	skipped := 0
	for skipped < toSkip {
		hs, closed, head, sz := b.loadHeader()
		n := minInt(sz, toSkip-skipped)
		if n > 0 {
			for {
				head = (head + n) & b.mask
				sz -= n
				if atomic.CompareAndSwapUint64(&b.bits, hs, (hs&headerFlagMask)|(uint64(head)<<32)|uint64(sz)) {
					break
				}
				runtime.Gosched()
				hs, closed, head, sz = b.loadHeader()
			}
			skipped += n
			notify(b.rsig)
		} else {
			if closed {
				notify(b.wsig)
				return skipped, io.EOF
			}
			<-b.wsig
		}
	}
	if b.IsClosed() {
		return skipped, io.EOF
	}
	return skipped, nil
}

func (b *RingBuf) SkipContext(ctx context.Context, toSkip int) (int, error) {
	if toSkip <= 0 {
		if b.IsClosed() {
			return 0, io.EOF
		}
		return 0, nil
	}
	skipped := 0
	for skipped < toSkip {
		hs, closed, head, sz := b.loadHeader()
		n := minInt(sz, toSkip-skipped)
		if n > 0 {
			for {
				head = (head + n) & b.mask
				sz -= n
				if atomic.CompareAndSwapUint64(&b.bits, hs, (hs&headerFlagMask)|(uint64(head)<<32)|uint64(sz)) {
					break
				}
				runtime.Gosched()
				hs, closed, head, sz = b.loadHeader()
			}
			skipped += n
			notify(b.rsig)
		} else {
			if closed {
				notify(b.wsig)
				return skipped, io.EOF
			}
			select {
			case <-b.wsig:
			case <-ctx.Done():
				return skipped, ctx.Err()
			}
		}
	}
	if b.IsClosed() {
		return skipped, io.EOF
	}
	return skipped, nil
}

// ReadAvaial returns number of bytes availbale to immediate read
func (b *RingBuf) ReadAvail() int {
	return int(atomic.LoadUint64(&b.bits) & uint64(low31bits))
}

// WriteAvail returns number of bytes that can be written immediately
func (b *RingBuf) WriteAvail() int {
	return b.Cap() - b.ReadAvail()
}

// Cap returns capacity of the buffer
func (b *RingBuf) Cap() int {
	return len(b.mem)
}

func (b *RingBuf) WriteAll(chunks ...[]byte) (int64, error) {
	var written int64
	for _, data := range chunks {
		n, err := b.Write(data)
		written += int64(n)
		if err != nil {
			return written, err
		}
	}
	return written, nil
}

func (b *RingBuf) WriteUnlock() {
	atomic.StoreInt32(&b.wlck, 0)
	if atomic.LoadInt32(&b.wq) != 0 && !b.IsClosed() {
		notify(b.lsig)
	}
}

func (b *RingBuf) WriteLock() error {
	// fast path
	wlck := atomic.LoadInt32(&b.wlck)
	if (wlck == 0) && atomic.CompareAndSwapInt32(&b.wlck, 0, 1) {
		return nil
	}
	// slow path
	atomic.AddInt32(&b.wq, 1)
	for {
		// first spin some
		for i := 0; i < 100; i++ {
			wlck = atomic.LoadInt32(&b.wlck)
			if (wlck == 0) && atomic.CompareAndSwapInt32(&b.wlck, 0, 1) {
				atomic.AddInt32(&b.wq, -1)
				return nil
			}
			runtime.Gosched()
		}
		// then wait notification
		<-b.lsig
		if b.IsClosed() {
			atomic.AddInt32(&b.wq, 1)
			notify(b.lsig)
			return io.EOF
		}
	}
}

func (b *RingBuf) WriteLockContext(ctx context.Context) error {
	// fast path
	wlck := atomic.LoadInt32(&b.wlck)
	if (wlck == 0) && atomic.CompareAndSwapInt32(&b.wlck, 0, 1) {
		return nil
	}
	// slow path
	atomic.AddInt32(&b.wq, 1)
	for {
		// first spin some
		for i := 0; i < 100; i++ {
			wlck = atomic.LoadInt32(&b.wlck)
			if (wlck == 0) && atomic.CompareAndSwapInt32(&b.wlck, 0, 1) {
				atomic.AddInt32(&b.wq, -1)
				return nil
			}
			runtime.Gosched()
		}
		// then wait notification
		select {
		case <-b.lsig:
		case <-ctx.Done():
			atomic.AddInt32(&b.wq, 1)
			return ctx.Err()
		}
		if b.IsClosed() {
			notify(b.lsig)
			atomic.AddInt32(&b.wq, 1)
			return io.EOF
		}
	}
}
