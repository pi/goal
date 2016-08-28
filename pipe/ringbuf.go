package pipe

import (
	"context"
	"errors"
	"io"
	"runtime"
	"sync/atomic"

	"github.com/pi/goal/gut"
)

var ErrOvercap = errors.New("Buffer overcap")

type ringbuf struct {
	pbits *uint64 // highest bit - close flag. next 31 bits: read pos, next bit - unused, next 31 bits: read avail
	mem   []byte
	mask  int
	wsig  chan struct{}
	rsig  chan struct{}
	lsig  chan struct{}
	lck   int32
	lq    int32
}

const low63bits = ^uint64(0) >> 1
const low31bits = ^uint32(0) >> 1

const closeFlag = low63bits + 1

const headerFlagMask = closeFlag

const defaultBufferSize = 32 * 1024
const minBufferSize = 8

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (b *ringbuf) loadHeader() (hs uint64, closed bool, readPos int, readAvail int) {
	hs = atomic.LoadUint64(b.pbits)
	closed = (hs & closeFlag) != 0
	readPos = int((hs >> 32) & uint64(low31bits))
	readAvail = int(hs & uint64(low31bits))
	return
}

func (b *ringbuf) Close() error {
	for {
		hs := atomic.LoadUint64(b.pbits)
		if ((hs & closeFlag) != 0) || atomic.CompareAndSwapUint64(b.pbits, hs, hs|closeFlag) {
			if (hs & closeFlag) == 0 {
				close(b.rsig)
				close(b.wsig)
				close(b.lsig)
			}
			return nil
		}
		runtime.Gosched()
	}
}

/*
func (b *ringbuf) Reopen() {
	b.rsig = make(chan struct{}, 1)
	b.wsig = make(chan struct{}, 1)
	b.lsig = make(chan struct{}, 1)
	*b.pbits = 0 // reset head and size
	b.lck = 0
	b.lq = 0
}*/

func (b *ringbuf) IsClosed() bool {
	return (atomic.LoadUint64(b.pbits) & closeFlag) != 0
}

func notify(c chan struct{}) {
	select {
	case c <- struct{}{}:
	default:
	}
}

func (b *ringbuf) read(data []byte) (int, error) {
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

func (b *ringbuf) readWithContext(ctx context.Context, data []byte) (int, error) {
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
				if atomic.CompareAndSwapUint64(b.pbits, hs, nhs) {
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

func (b *ringbuf) writeWithContext(ctx context.Context, data []byte) (int, error) {
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
			select {
			case <-b.rsig:
			case <-ctx.Done():
				return written, ctx.Err()
			}
		}
	}
	return toWrite, nil
}

func (b *ringbuf) peek(data []byte) (int, error) {
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

func (b *ringbuf) skip(toSkip int) (int, error) {
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
				if atomic.CompareAndSwapUint64(b.pbits, hs, (hs&headerFlagMask)|(uint64(head)<<32)|uint64(sz)) {
					break
				}
				runtime.Gosched()
				hs, closed, head, sz = b.loadHeader()
			}
			skipped += n
			notify(b.rsig)
		} else {
			if closed {
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

func (b *ringbuf) skipWithContext(ctx context.Context, toSkip int) (int, error) {
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
				if atomic.CompareAndSwapUint64(b.pbits, hs, (hs&headerFlagMask)|(uint64(head)<<32)|uint64(sz)) {
					break
				}
				runtime.Gosched()
				hs, closed, head, sz = b.loadHeader()
			}
			skipped += n
			notify(b.rsig)
		} else {
			if closed {
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
func (b *ringbuf) readAvail() int {
	return int(atomic.LoadUint64(b.pbits) & uint64(low31bits))
}

// Cap returns capacity of the buffer
func (b *ringbuf) Cap() int {
	return len(b.mem)
}

func (b *ringbuf) writeAll(chunks ...[]byte) (int64, error) {
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

func (b *ringbuf) unlock() {
	atomic.StoreInt32(&b.lck, 0)
	if atomic.LoadInt32(&b.lq) != 0 && !b.IsClosed() {
		notify(b.lsig)
	}
}

func (b *ringbuf) lock() error {
	// fast path
	lck := atomic.LoadInt32(&b.lck)
	if (lck == 0) && atomic.CompareAndSwapInt32(&b.lck, 0, 1) {
		return nil
	}
	// slow path
	atomic.AddInt32(&b.lq, 1)
	for {
		// first spin some
		for i := 0; i < 100; i++ {
			lck = atomic.LoadInt32(&b.lck)
			if (lck == 0) && atomic.CompareAndSwapInt32(&b.lck, 0, 1) {
				atomic.AddInt32(&b.lq, -1)
				return nil
			}
			runtime.Gosched()
		}
		// then wait notification
		<-b.lsig
		if b.IsClosed() {
			atomic.AddInt32(&b.lq, 1)
			return io.EOF
		}
	}
}

func (b *ringbuf) lockWithContext(ctx context.Context) error {
	// fast path
	lck := atomic.LoadInt32(&b.lck)
	if (lck == 0) && atomic.CompareAndSwapInt32(&b.lck, 0, 1) {
		return nil
	}
	// slow path
	atomic.AddInt32(&b.lq, 1)
	for {
		// first spin some
		for i := 0; i < 100; i++ {
			lck = atomic.LoadInt32(&b.lck)
			if (lck == 0) && atomic.CompareAndSwapInt32(&b.lck, 0, 1) {
				atomic.AddInt32(&b.lq, -1)
				return nil
			}
			runtime.Gosched()
		}
		// then wait notification
		select {
		case <-b.lsig:
		case <-ctx.Done():
			atomic.AddInt32(&b.lq, 1)
			return ctx.Err()
		}
		if b.IsClosed() {
			atomic.AddInt32(&b.lq, 1)
			return io.EOF
		}
	}
}
