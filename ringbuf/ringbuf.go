package ringbuf

import (
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
			return nil
		}
		runtime.Gosched()
	}
}

func (b *RingBuf) Reopen() {
	atomic.StoreUint64(&b.bits, 0) // reset close flag, head and size
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
			if closed {
				return readed, io.EOF
			}
		} else {
			if closed {
				return readed, io.EOF
			}
			notify(b.rsig)
			<-b.wsig
		}
	}
	if readed > 0 {
		notify(b.rsig)
	}
	return readed, nil
}

func (b *RingBuf) Write(data []byte) (int, error) {
	toWrite := len(data)
	if toWrite == 0 {
		return 0, nil
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
		} else {
			if closed {
				return written, io.EOF
			}
			notify(b.wsig)
			<-b.rsig
		}
	}
	notify(b.wsig)
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
	if toSkip < 0 {
		return 0, nil
	}
	hs, closed, head, sz := b.loadHeader()
	n := minInt(sz, toSkip)
	if n > 0 {
		for {
			head = (head + n) & b.mask
			sz -= n
			if atomic.CompareAndSwapUint64(&b.bits, hs, (hs&headerFlagMask)|(uint64(head)<<32)|uint64(sz)) {
				break
			}
			hs, closed, head, sz = b.loadHeader()
		}
	}
	if closed {
		return n, io.EOF
	}
	return n, nil
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
		_, closed, head, sz := b.loadHeader()
		if closed {
			return written, io.EOF
		}
		nw := minInt(b.Cap()-sz, len(data))
		if nw == 0 {
			return written, nil
		}
		writePos := (head + sz) & b.mask
		if writePos > b.Cap()-nw {
			// wrapped
			ll := b.Cap() - writePos
			copy(b.mem[writePos:], data[:ll])
			copy(b.mem[:nw-ll], data[ll:nw])
		} else {
			copy(b.mem[writePos:writePos+nw], data[:nw])
		}
		atomic.AddUint64(&b.bits, uint64(nw))
		written += int64(nw)
	}
	return written, nil
}
