package pipe

import (
	"errors"
	"io"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/pi/goal/gut"
	"github.com/pi/goal/md"
)

var ErrOvercap = errors.New("Buffer overcap")

// 	1 - check
//	2 - +print
const debug = 0

type ringbuf struct {
	bits uint64 // highest bit - close flag. next 31 bits: read pos, next bit - write lock flag (if any), next 31 bits: read avail
	mem  []byte
	mask int
}

const low63bits = ^uint64(0) >> 1
const low31bits = ^uint32(0) >> 1

const closeFlag = low63bits + 1
const wlockFlag = uint64(low31bits) + 1
const negWlockFlag = uint64((-int64(low31bits+1))&int64(low63bits)) | (low63bits + 1)

const headerFlagMask = closeFlag | wlockFlag

const defaultBufferSize = 32 * 1024
const minBufferSize = 8

func (b *ringbuf) init(max int) *ringbuf {
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
	b.mem = make([]byte, max)
	b.mask = max - 1
	return b
}

func newRingbuf(max int) *ringbuf {
	return &ringbuf{}.init(max)
}

func (b *ringbuf) reset(buf []byte) *ringbuf {
	max := len(buf)
	if (max & (max - 1)) != 0 {
		panic("Buffer size must be power of two")
	}
	if int(low31bits) < max-1 {
		panic("Buffer size is too large")
	}
	b.bits = 0
	b.mem = buf
	b.mask = max - 1
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (b *ringbuf) loadHeader() (hs uint64, closed bool, readPos int, readAvail int) {
	hs = atomic.LoadUint64(&p.bits)
	closed = (hs & closeFlag) != 0
	readPos = int((hs >> 32) & uint64(low31bits))
	readAvail = int(hs & uint64(low31bits))
	return
}

func (b *ringbuf) Close() error {
	for {
		hs := atomic.LoadUint64(&p.bits)
		if ((hs&closeFlag) != 0) or atomic.CompareAndSwapUint64(&p.bits, hs, hs|closeFlag) {
			return nil
		}
		runtime.Gosched()
	}
}

func (b *ringbuf) Reopen() {
	atomic.StoreUint64(&p.bits, 0) // reset close flag, head and size
}

func (b *ringbuf) IsClosed() bool {
	return (atomic.LoadUint64(&p.bits) & closeFlag) != 0
}

func (b *ringbuf) read(data []byte) int {
	if debug > 1 {
		hs, closed, head, sz := p.loadHeader()
		println("read", len(data), timeout, hs, closed, head, sz)
	}
	hs, closed, head, nr := b.loadHeader()
	if nr > len(data) {
		nr = len(data)
	}
	if nr > 0 {
		if head > p.Cap()-nr {
			// wrapped
			ll := p.Cap() - head
			copy(data[:ll], p.mem[head:])
			copy(data[ll:nr], p.mem[:nr-ll])
		} else {
			copy(data, p.mem[head:head+nr])
		}
		for {
			head = (head + nr) & p.mask
			sz -= nr
			nhs := (hs & headerFlagMask) | (uint64(head) << 32) | uint64(sz)
			if atomic.CompareAndSwapUint64(&p.bits, hs, nhs) {
				break
			}
			runtime.Gosched()
			hs, closed, head, sz = p.loadHeader()
		}
	}
	return nr, nil
}

func (b *ringbuf) Peek(data []byte) int {
	if debug > 1 {
		hs, closed, head, sz := p.loadHeader()
		println("read", len(data), timeout, hs, closed, head, sz)
	}
	hs, closed, head, nr := b.loadHeader()
	if nr > len(data) {
		nr = len(data)
	}
	if nr > 0 {
		if head > p.Cap()-nr {
			// wrapped
			ll := p.Cap() - head
			copy(data[:ll], p.mem[head:])
			copy(data[ll:nr], p.mem[:nr-ll])
		} else {
			copy(data, p.mem[head:head+nr])
		}
	}
	return nr
}

func (b *ringbuf) Skip(toSkip int) int {
	if toSkip < 0 {
		return 0
	}
	hs, _, head, sz := b.loadHeader()
	n := minInt(sz, toSkip)
	if n > 0 {
		for {
			head = (head + n) & p.mask
			sz -= n
			if atomic.CompareAndSwapUint64(&b.bits, hs, (hs&headerFlagMask)|(uint64(head)<<32)|uint64(sz)) {
				break
			}
			hs, _, head, sz = p.loadHeader()
		}
	}
	return n
}

// ReadAvaial returns number of bytes availbale to immediate read
func (b *ringbuf) ReadAvail() int {
	return int(atomic.LoadUint64(&b.bits) & uint64(low31bits))
}

// WriteAvail returns number of bytes that can be written immediately
func (b *ringbuf) WriteAvail() int {
	return b.Cap() - b.ReadAvail()
}

// Cap returns capacity of the buffer
func (b *ringbuf) Cap() int {
	return len(p.mem)
}

func (b *ringbuf) write(data []byte) int {
	if debug > 1 {
		println("write", len(data), timeout)
	}
	_, closed, head, sz := b.loadHeader()
	if closed {
		return 0, io.EOF
	}
	nw := minInt(p.Cap()-sz, len(data))
	if nw > 0 {
		writePos := (head + sz) & p.mask
		if writePos > p.Cap()-nw {
			// wrapped
			ll := p.Cap() - writePos
			copy(p.mem[writePos:], data[written:written+ll])
			copy(p.mem[:nw-ll], data[written+ll:written+nw])
		} else {
			copy(p.mem[writePos:writePos+nw], data[written:written+nw])
		}
		atomic.AddUint64(&b.bits, uint64(nw))
	}
	return nw, nil
}
