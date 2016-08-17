package gut

import (
	"errors"
	"io"
	"runtime"
	"sync/atomic"
	"time"
)

var ErrOvercap = errors.New("Buffer overcap")

type timeoutError struct{}

// Timeout error conforms to net.Error
func (timeoutError) Error() string     { return "i/o timeout" }
func (timeoutError) IsTimeout() bool   { return true }
func (timeoutError) IsTemporary() bool { return true }

var ErrTimeout = timeoutError{}

const low63bits = (^uint64(0)) >> 1
const low31bits = (^uint32(0)) >> 1

const infinite = time.Duration(low63bits)
const sleepTime = time.Microsecond

const closeFlag = 1 << 63

const defaultBufferSize = 64 * 1024

const NoTimeout = time.Duration(-1)

const kCapturedHeader = ^uint64(0)

const MinRingbufSize = 8

type RingBuf struct {
	header uint64 // 1bit closed flag, 31bit readPos, 1bit capture flag, 31bit readAvail
	mem    []byte
	mask   uint32
}

func NewRingBuf(max uint32) *RingBuf {
	if max == 0 {
		max = defaultBufferSize
	} else if max < MinRingbufSize {
		max = MinRingbufSize
	} else if (max & (max - 1)) != 0 {
		// round up to power of two
		max = 1 << BitLen(uint(max))
	}
	if max >= 0x80000000 {
		panic("RingBuf size is too large")
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

func unpackHeader(header uint64) (closed bool, readPos uint32, readAvail uint32) {
	closed = (header & closeFlag) != 0
	readPos = uint32(header>>32) & low31bits
	readAvail = uint32(header)
	return
}

func packHeader(closed bool, readPos uint32, readAvail uint32) (header uint64) {
	header = uint64(readPos)<<32 | uint64(readAvail)
	if closed {
		header |= closeFlag
	}
	return header
}

func (b *RingBuf) loadHeader() (closed bool, readPos uint32, readAvail uint32) {
	header := atomic.LoadUint64(&b.header)
	for header == kCapturedHeader {
		runtime.Gosched()
		header = atomic.LoadUint64(&b.header)
	}
	return unpackHeader(header)
}

func (b *RingBuf) capture() (closed bool, readPos uint32, readAvail uint32) {
	for {
		header := atomic.LoadUint64(&b.header)
		if (header != kCapturedHeader) && atomic.CompareAndSwapUint64(&b.header, header, kCapturedHeader) {
			return unpackHeader(header)
		}
		runtime.Gosched()
	}
}

func (b *RingBuf) release(closed bool, readPos uint32, readAvail uint32) {
	newHeader := packHeader(closed, readPos, readAvail)
	if closed {
		newHeader |= closeFlag
	}
	if !atomic.CompareAndSwapUint64(&b.header, kCapturedHeader, newHeader) {
		panic("inconsistent RingBuf.release")
	}
}

func (b *RingBuf) Close() {
	// first try fast without locking buffer
	_, readPos, readAvail := b.capture()
	b.release(true, readPos, readAvail)
}

func (b *RingBuf) Reopen() {
	b.capture()
	b.release(false, 0, 0) // reset close flag, readPos and readAvail
}

// IsClosed return true if the ring buffer is closed atm
func (b *RingBuf) IsClosed() bool {
	closed, _, _ := b.loadHeader()
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
	closed, _, _ := b.capture()
	b.release(closed, 0, 0)
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
	for readed < readLimit {
		closed, readPos, readAvail := b.capture()
		if readAvail > 0 {
			nr := minU32(readAvail, readLimit-readed)
			if readPos+nr > b.Cap() {
				// wrapped
				ll := b.Cap() - readPos
				copy(data[readed:readed+ll], b.mem[readPos:])
				copy(data[readed+ll:readed+nr], b.mem[:nr-ll])
			} else {
				copy(data[readed:readed+nr], b.mem[readPos:readPos+nr])
			}
			readed += nr
			readAvail -= nr
			if readAvail == 0 {
				readPos = 0 // minimize wraps
			} else {
				readPos = (readPos + nr) & b.mask
			}
			b.release(closed, readPos, readAvail)
		} else {
			b.release(closed, readPos, readAvail)
			if closed {
				return int(readed), io.EOF
			}
			if periods == 0 {
				break
			}
			time.Sleep(sleepTime)
			periods--
		}
	}
	if readed < readLimit {
		return int(readed), ErrTimeout
	}
	return int(readed), nil
}

func (b *RingBuf) Read(data []byte) (int, error) {
	return b.ReadWithTimeout(data, infinite)
}

// Peek for immediately avialable data
func (b *RingBuf) Peek(data []byte) uint32 {
	closed, readPos, readAvail := b.capture()
	l := minU32(readAvail, uint32(len(data)))
	if l == 0 {
		b.release(closed, readPos, readAvail)
		return 0
	}
	if readPos+l > b.Cap() {
		// wrap
		ll := b.Cap() - readPos
		copy(data[:ll], b.mem[readPos:])
		copy(data[ll:l], b.mem[:l-ll])
	} else {
		copy(data, b.mem[readPos:readPos+l])
	}
	b.release(closed, readPos, readPos)
	return l
}

// ReadWait waits for sepcified number of data bytes for specified timeout. Specify timeout 0 for blocking wait
// Returns number of bytes available to read atm or number of bytes available and error
// Possible errors:
//	io.EOF if the buffer is closed
//  ErrOvercap if the specified number of bytes is greater than the buffer capacity
func (b *RingBuf) ReadWait(min uint32, timeout time.Duration) (int, error) {
	readAvail := b.ReadAvail()
	if min > b.Cap() {
		return int(readAvail), ErrOvercap
	}
	if min == 0 {
		min = 1
	}
	if readAvail >= min {
		return int(readAvail), nil
	}
	var periods int64
	if timeout == 0 {
		periods = int64(low63bits)
	} else {
		periods = int64(timeout / sleepTime)
	}

	for i := int64(0); i <= periods; i++ {
		closed, _, readAvail := b.loadHeader()
		if readAvail >= min {
			return int(readAvail), nil
		}
		if closed {
			return int(readAvail), io.EOF
		}
		if i != periods {
			time.Sleep(sleepTime)
		}
	}
	return int(b.ReadAvail()), ErrTimeout
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
		closed, _, readAvail := b.loadHeader()
		if b.Cap()-readAvail >= min {
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
	for skipped < toSkip {
		closed, readPos, readAvail := b.capture()
		if readAvail > 0 {
			n := minU32(toSkip-skipped, readAvail)
			readAvail -= n
			if readAvail == 0 {
				readPos = 0
			} else {
				readPos = (readPos + n) & b.mask
			}
			b.release(closed, readPos, readAvail)
			skipped += n
			if skipped == toSkip {
				return toSkip, nil
			}
		} else {
			b.release(closed, readPos, readAvail)
			if closed {
				return skipped, io.EOF
			}
			if periods == 0 {
				return skipped, nil
			} else {
				periods--
				time.Sleep(sleepTime)
			}
		}
	}
	if skipped == toSkip {
		return skipped, nil
	}
	return skipped, ErrTimeout
}

// Skip with default timeout (blocking if b.ReadTimeout is not set)
func (b *RingBuf) Skip(toSkip uint32) (uint32, error) {
	return b.SkipWithTimeout(toSkip, infinite)
}

// Avail returns number of bytes available to read and write
func (b *RingBuf) Avail() (readAvail uint32, writeAvail uint32) {
	var closed bool
	closed, _, readAvail = b.loadHeader()
	if closed {
		writeAvail = 0
	} else {
		writeAvail = b.Cap() - readAvail
	}
	return
}

// ReadAvaial returns number of bytes availbale to immediate read
func (b *RingBuf) ReadAvail() (readAvail uint32) {
	_, _, readAvail = b.loadHeader()
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
		closed, readPos, readAvail := b.capture()
		if closed {
			b.release(closed, readPos, readAvail)
			return int(written), io.ErrClosedPipe
		}
		nw := minU32(b.Cap()-readAvail, toWrite-written)
		if nw == 0 {
			b.release(closed, readPos, readAvail)
			time.Sleep(sleepTime)
			continue
		}

		writePos := (readPos + readAvail) & b.mask
		if writePos+nw > b.Cap() {
			// wrapped
			ll := b.Cap() - writePos
			copy(b.mem[writePos:], data[written:written+ll])
			copy(b.mem[:nw-ll], data[written+ll:written+nw])
		} else {
			copy(b.mem[writePos:writePos+nw], data[written:written+nw])
		}
		readAvail += nw
		b.release(closed, readPos, readAvail)
		written += nw
	}
	return int(written), nil
}

// WriteChunks is optimized Write for multiple byte chunks
func (b *RingBuf) WriteChunks(chunks ...[]byte) (int, error) {
	var totalWritten int

	for _, data := range chunks {
		written := uint32(0)
		toWrite := uint32(len(data))
		for written < toWrite {
			closed, readPos, readAvail := b.capture()
			if closed {
				b.release(closed, readAvail, readPos)
				return totalWritten, io.ErrClosedPipe
			}
			nw := minU32(b.Cap()-readAvail, toWrite-written)
			if nw == 0 {
				b.release(closed, readPos, readAvail)
				time.Sleep(sleepTime)
				continue
			}
			writePos := (readPos + readAvail) & b.mask
			if writePos+nw > b.Cap() {
				// wrapped
				ll := b.Cap() - writePos
				copy(b.mem[writePos:], data[written:written+ll])
				copy(b.mem[:nw-ll], data[written+ll:written+nw])
			} else {
				copy(b.mem[writePos:writePos+nw], data[written:written+nw])
			}
			readAvail += nw
			b.release(closed, readPos, readAvail)
			written += nw
		}
		totalWritten += int(written)
	}
	return totalWritten, nil
}
