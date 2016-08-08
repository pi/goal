package rb

import (
	"errors"
	"io"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/pi/goal/gut"
)

var ErrOvercap = errors.New("Buffer overcap")

const infinite = time.Duration(0x7FFFFFFFFFFFFFFF)
const sleepTime = time.Microsecond

const closeFlag = 0x8000000000000000

const defaultBufferSize = 64 * 1024

type RingBuf struct {
	headAndSize uint64 // highest bit - close flag
	mem         []byte
	mask        uint32
	ReadTimeout time.Duration
}

func NewRingBuf(max uint32) *RingBuf {
	if max == 0 {
		max = defaultBufferSize
	} else if max < 2 {
		max = 2
	} else if (max & (max - 1)) != 0 {
		// round up to power of two
		max = 1 << gut.BitLen(uint(max))
	}
	if max >= 0x80000000 {
		panic("RingBuffer size too large")
	}
	return &RingBuf{
		mem:  make([]byte, int(max)),
		mask: max - 1,
	}
}

func badRead() {
	panic("inconsistent RingBuf.Read")
}

func minU32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

func (b *RingBuf) loadHS() (closed bool, hs uint64, head uint32, readAvail uint32) {
	hs = atomic.LoadUint64(&b.headAndSize)
	closed = (hs & closeFlag) != 0
	head = uint32(hs>>32) & 0x7FFFFFFF
	readAvail = uint32(hs)
	return
}

func (b *RingBuf) Close() {
	for {
		oldHS := atomic.LoadUint64(&b.headAndSize)
		newHS := oldHS | closeFlag
		if atomic.CompareAndSwapUint64(&b.headAndSize, oldHS, newHS) {
			return
		}
		runtime.Gosched()
	}
}

func (b *RingBuf) Reopen() {
	b.headAndSize = 0 // reset close flag, head and size
}

func (b *RingBuf) IsClosed() bool {
	closed, _, _, _ := b.loadHS()
	return closed
}

/*
func (b *RingBuf) read(data []byte, moveReadPosition bool) uint32 {
	toRead := uint32(len(data))
	hs, head, sz := b.loadHS()

	if toRead > sz {
		toRead = sz
	}
	if head+toRead > b.Cap() {
		// wrapped
		ll := b.Cap() - head
		copy(data[0:ll], b.mem[head:head+ll])
		copy(data[ll:toRead], b.mem[0:toRead-ll])
	} else {
		copy(data[0:toRead], b.mem[head:head+toRead])
	}
	if moveReadPosition {
		for {
			sz -= toRead
			if sz == 0 {
				head = 0
			} else {
				head = (head + toRead) & b.mask
			}
			nhs := (uint64(head) << 32) | uint64(sz)
			if atomic.CompareAndSwapUint64(&b.headAndSize, hs, nhs) {
				break
			}
			runtime.Gosched()
			hs, head, sz = b.loadHS()
		}
	}
	return toRead
}
*/

func (b *RingBuf) Clear() {
	atomic.StoreUint64(&b.headAndSize, 0)
}

func (b *RingBuf) ReadByte() (byte, error) {
	var buf [1]byte
	_, err := b.Read(buf[:])
	return buf[0], err
}

func (b *RingBuf) WriteByte(c byte) error {
	var buf [1]byte
	buf[0] = c
	_, err := b.Write(buf[:])
	return err
}

func (b *RingBuf) ReadWithTimeout(data []byte, timeout time.Duration) (int, error) {
	readed := uint32(0)
	readLimit := uint32(len(data))
	periods := int64(timeout / sleepTime)
	for readed < readLimit { // <= periods to do at least one read
		closed, hs, head, sz := b.loadHS()
		if sz > 0 {
			nr := minU32(sz, readLimit-readed)
			if head+nr > b.Cap() {
				// wrapped
				ll := b.Cap() - head
				copy(data[readed:readed+ll], b.mem[head:])
				copy(data[readed+ll:readed+nr], b.mem[:nr-ll])
			} else {
				copy(data[readed:readed+nr], b.mem[head:head+nr])
			}
			readed += nr
			for {
				sz -= nr
				head = (head + nr) & b.mask
				nhs := (uint64(head) << 32) | uint64(sz)
				if closed {
					nhs |= closeFlag
				}
				if atomic.CompareAndSwapUint64(&b.headAndSize, hs, nhs) {
					break
				}
				runtime.Gosched()
				closed, hs, head, sz = b.loadHS()
			}
		} else {
			if closed {
				return int(readed), io.EOF
			}
			periods--
			if periods < 0 {
				break
			}
			time.Sleep(sleepTime)
		}
	}
	return int(readed), nil
}

func (b *RingBuf) Read(data []byte) (int, error) {
	if b.ReadTimeout == 0 {
		return b.ReadWithTimeout(data, infinite)
	} else {
		return b.ReadWithTimeout(data, b.ReadTimeout)
	}
}

// Peek for avialable data. ReadTimeout is not used, close flag is not handled
func (b *RingBuf) Peek(data []byte) uint32 {
	_, _, head, sz := b.loadHS()
	l := minU32(sz, uint32(len(data)))
	if l == 0 {
		return 0
	}
	if head+l > b.Cap() {
		// wrap
		ll := b.Cap() - head
		copy(data[:ll], b.mem[head:])
		copy(data[ll:l], b.mem[:l-ll])
	} else {
		copy(data, b.mem[head:head+l])
	}
	return l
}

// ReadWait waits for sepcified number of data bytes for specified timeout.
// Returns:
//	true, nil if the wait was successfull
//	false, nil on timeout
//	false, io.EOF if pipe is closed
//  false, ErrOvercap if the specified number of bytes is greater than the buffer capacity
func (b *RingBuf) ReadWait(min uint32, timeout time.Duration) (bool, error) {
	if min > b.Cap() {
		return false, ErrOvercap
	}
	if min == 0 {
		min = 1
	}
	if b.ReadAvail() >= min {
		return true, nil
	}
	if timeout == 0 {
		return false, nil
	}

	periods := int64(timeout / sleepTime)

	for i := int64(0); i <= periods; i++ {
		closed, _, _, ra := b.loadHS()
		if ra >= min {
			return true, nil
		}
		if closed {
			if ra > 0 {
				return false, io.ErrUnexpectedEOF
			} else {
				return false, io.EOF
			}
		}
		if i != periods {
			time.Sleep(sleepTime)
		}
	}
	return false, nil
}

// WriteWait waits till buffer has specified avialable to write space
// Returns:
//	true, nil - there is space
//	false, nil - timeout expired and there is no space
//	false, io.ErrClosedPipe - pipe is closed
//	false, ErrOvercap if the specified number of bytes is greater than the buffer capacity
func (b *RingBuf) WriteWait(min uint32, timeout time.Duration) (bool, error) {
	if min > b.Cap() {
		return false, ErrOvercap
	}
	if min == 0 {
		min = 1
	}
	if b.WriteAvail() >= min {
		return true, nil
	}

	periods := int64(timeout / sleepTime)
	for i := int64(0); i <= periods; i++ {
		closed, _, _, sz := b.loadHS()
		if b.Cap()-sz >= min {
			return true, nil
		}
		if closed {
			return false, io.ErrClosedPipe
		}
		if i != periods {
			time.Sleep(sleepTime)
		}
	}
	return false, nil
}

// SkipWithTimeout similar to Read but discards readed data
//	returns number of bytes skipped
// possible errors: io.EOF if pipe was closed
func (b *RingBuf) SkipWithTimeout(toSkip uint32, timeout time.Duration) (uint32, error) {
	if toSkip == 0 {
		return 0, nil
	}
	skipped := uint32(0)
	periods := int64(timeout / sleepTime)
	for i := int64(0); i <= periods; i++ {
		closed, hs, head, sz := b.loadHS()
		if sz > 0 {
			n := minU32(toSkip-skipped, sz)
			for {
				sz -= n
				head = (head + n) & b.mask
				if closed {
					head |= closeFlag >> 32
				}
				nhs := (uint64(head) << 32) | uint64(sz)
				if atomic.CompareAndSwapUint64(&b.headAndSize, hs, nhs) {
					break
				}
				runtime.Gosched()
				closed, hs, head, sz = b.loadHS()
			}
			skipped += n
			if skipped == toSkip {
				return toSkip, nil
			}
		} else {
			if closed {
				return skipped, io.EOF
			}
			if i != periods {
				time.Sleep(sleepTime)
			}
		}
	}
	return skipped, nil
}

// Skip with default timeout (blocking if b.ReadTimeout is not set)
func (b *RingBuf) Skip(toSkip uint32) (uint32, error) {
	if b.ReadTimeout == 0 {
		return b.SkipWithTimeout(toSkip, infinite)
	} else {
		return b.SkipWithTimeout(toSkip, b.ReadTimeout)
	}
}

// Avail returns number of bytes available to read and write
func (b *RingBuf) Avail() (readAvail uint32, writeAvail uint32) {
	_, _, _, readAvail = b.loadHS()
	writeAvail = b.Cap() - readAvail
	return
}

// ReadAvaial returns number of bytes availbale to immediate read
func (b *RingBuf) ReadAvail() (readAvail uint32) {
	_, _, _, readAvail = b.loadHS()
	return
}

// WriteAvail returns number of bytes that can be written immediately
func (b *RingBuf) WriteAvail() uint32 {
	return b.Cap() - b.ReadAvail()
}

// Cap returns capacity of the buffer
func (b *RingBuf) Cap() uint32 {
	return uint32(len(b.mem))
}

// WriteString is sugar for Write(string(bytes))
func (b *RingBuf) WriteString(s string) (int, error) {
	return b.Write([]byte(s))
}

// ReadString reads string from buffer. For errors see RingBuf.Read
func (b *RingBuf) ReadString(maxSize int) (string, error) {
	if maxSize == 0 {
		return "", nil
	}
	data := make([]byte, maxSize, maxSize)
	nr, err := b.Read(data)
	return string(data[:nr]), err
}

// Write writes bytes to buffer. Function will not return till all bytes written
// possible errors: io.ErrClosedPipe if pipe was closed
func (b *RingBuf) Write(data []byte) (int, error) {
	written := uint32(0)
	toWrite := uint32(len(data))
	for written < toWrite {
		closed, hs, head, sz := b.loadHS()
		if closed {
			return int(written), io.ErrClosedPipe
		}
		nw := minU32(b.Cap()-sz, toWrite-written)
		if nw == 0 {
			time.Sleep(sleepTime)
			continue
		}

		writePos := (head + sz) & b.mask
		if writePos+nw > b.Cap() {
			// wrapped
			ll := b.Cap() - writePos
			copy(b.mem[writePos:], data[written:written+ll])
			copy(b.mem[:nw-ll], data[written+ll:written+nw])
		} else {
			copy(b.mem[writePos:writePos+nw], data[written:written+nw])
		}
		for {
			sz += nw
			nhs := (uint64(head) << 32) | uint64(sz)
			if atomic.CompareAndSwapUint64(&b.headAndSize, hs, nhs) {
				break
			}
			runtime.Gosched()
			closed, hs, head, sz = b.loadHS()
			if closed {
				return int(written), io.ErrClosedPipe
			}
		}
		written += nw
	}
	return int(written), nil
}

// WriteChunks is optimized Write for multiple byte chunks
//	possible errors:
func (b *RingBuf) WriteChunks(chunks ...[]byte) (int, error) {
	var totalWritten int

	for _, data := range chunks {
		written := uint32(0)
		toWrite := uint32(len(data))
		for written < toWrite {
			closed, hs, head, sz := b.loadHS()
			if closed {
				return totalWritten, io.ErrClosedPipe
			}
			nw := minU32(b.Cap()-sz, toWrite-written)
			if nw == 0 {
				//runtime.Gosched()
				time.Sleep(sleepTime)
				continue
			}
			writePos := (head + sz) & b.mask
			if writePos+nw > b.Cap() {
				// wrapped
				ll := b.Cap() - writePos
				copy(b.mem[writePos:], data[written:written+ll])
				copy(b.mem[:nw-ll], data[written+ll:written+nw])
			} else {
				copy(b.mem[writePos:writePos+nw], data[written:written+nw])
			}
			for {
				sz += nw
				nhs := (uint64(head) << 32) | uint64(sz)
				if atomic.CompareAndSwapUint64(&b.headAndSize, hs, nhs) {
					break
				}
				runtime.Gosched()
				closed, hs, head, sz = b.loadHS()
				if closed {
					return totalWritten, io.ErrClosedPipe
				}
			}
			written += nw
		}
		totalWritten += int(written)
	}
	return totalWritten, nil
}
