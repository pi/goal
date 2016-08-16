package rb

import (
	"errors"
	"io"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/pi/goal/gut"
	"github.com/pi/goal/md"
)

const low63bits = (^uint64(0)) >> 1
const low31bits = (^uint32(0)) >> 1

var ErrOvercap = errors.New("Buffer overcap")

type timeoutError struct{}

// Timeout error conforms to net.Error
func (timeoutError) Error() string     { return "i/o timeout" }
func (timeoutError) IsTimeout() bool   { return true }
func (timeoutError) IsTemporary() bool { return true }

var ErrTimeout = timeoutError{}

const infinite = time.Duration(int64(low63bits))
const initialSleepTime = time.Microsecond
const maxSleepTime = 100 * time.Millisecond

const NoTimeout = time.Duration(-1)

const closeFlag = low63bits + 1

const defaultBufferSize = 32 * 1024
const minBufferSize = 8

func calcDeadline(timeout time.Duration) time.Duration {
	if timeout == 0 {
		return infinite
	} else if timeout < 0 {
		return 0 // nowait
	}
	ct := md.Monotime()
	result := ct + timeout
	if result < timeout {
		// overflow
		result = infinite
	}
	return result
}

func minDuration(d1, d2, d3 time.Duration) time.Duration {
	if d2 < d3 {
		if d1 < d2 {
			return d1
		}
		return d2
	}
	if d1 < d3 {
		return d1
	}
	return d3
}

type RingBuf struct {
	headAndSize uint64 // highest bit - close flag. next 31 bits: read pos, next lower 32 bits: read avail (only lower 31 bits is used)
	mem         []byte
	mask        uint32
}

func New(max uint32) *RingBuf {
	if max == 0 {
		max = defaultBufferSize
	} else if max < minBufferSize {
		max = minBufferSize
	} else if (max & (max - 1)) != 0 {
		// round up to power of two
		max = 1 << gut.BitLen(uint(max))
	}
	if max > (low31bits + 1) {
		panic("RingBuffer size too large")
	}
	return &RingBuf{
		mem:  make([]byte, int(max)),
		mask: max - 1,
	}
}

func With(buf []byte) *RingBuf {
	max := uint(len(buf))
	if (max & (max - 1)) != 0 {
		panic("buffer size must be power of two")
	}
	if max > uint(low31bits)+1 {
		panic("Buffer size too large")
	}
	return &RingBuf{
		mem:  buf,
		mask: uint32(max) - 1,
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

func (b *RingBuf) loadHeader() (hs uint64, closed bool, readPos uint32, readAvail uint32) {
	hs = atomic.LoadUint64(&b.headAndSize)
	closed = (hs & closeFlag) != 0
	readPos = uint32(hs>>32) & low31bits
	readAvail = uint32(hs) & low31bits
	return
}

func (b *RingBuf) Close() {
	for {
		hs := atomic.LoadUint64(&b.headAndSize)
		if atomic.CompareAndSwapUint64(&b.headAndSize, hs, hs|closeFlag) {
			return
		}
		runtime.Gosched()
	}
}

func (b *RingBuf) Reopen() {
	atomic.StoreUint64(&b.headAndSize, 0) // reset close flag, head and size
}

func (b *RingBuf) IsClosed() bool {
	return (atomic.LoadUint64(&b.headAndSize) & closeFlag) != 0
}

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

// ReadWithTimeout read bytes with the specified timeout
// Returns number of bytes readed and error
//	Errors:
//		io.EOF if buffer was closed
//		ErrTimeout if timeout expired
func (b *RingBuf) ReadWithTimeout(data []byte, timeout time.Duration) (int, error) {
	readed := uint32(0)
	readLimit := uint32(len(data))
	sleepTime := initialSleepTime
	deadline := calcDeadline(timeout)
	for readed < readLimit { // <= periods to do at least one read
		hs, closed, head, sz := b.loadHeader()
		if sz > 0 {
			sleepTime = initialSleepTime // reset sleep time
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
			//b.updateHeaderAfterRead(hs, nr)
			for {
				head = (head + nr) & b.mask
				sz -= nr
				nhs := (hs & closeFlag) | (uint64(head) << 32) | uint64(sz)
				if atomic.CompareAndSwapUint64(&b.headAndSize, hs, nhs) {
					break
				}
				runtime.Gosched()
				hs, closed, head, sz = b.loadHeader()
			}
		} else {
			if closed {
				return int(readed), io.EOF
			}
			remainingTime := deadline - md.Monotime()
			if remainingTime <= 0 {
				return int(readed), ErrTimeout
			}
			sleepTime = minDuration(sleepTime*2, remainingTime, maxSleepTime)
			time.Sleep(sleepTime)
		}
	}
	return int(readed), nil
}

func (b *RingBuf) Read(data []byte) (int, error) {
	return b.ReadWithTimeout(data, 0)
}

// Peek for avialable data. ReadTimeout is not used, close flag is not handled
func (b *RingBuf) Peek(data []byte) uint32 {
	_, _, head, sz := b.loadHeader()
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
//	true, io.EOF if pipe is closed and reamining number of bytes is equal to min
//	false, ErrTimeout on timeout
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
	if timeout < 0 {
		return false, ErrTimeout
	}

	deadline := calcDeadline(timeout)
	sleepTime := initialSleepTime
	for {
		_, closed, _, sz := b.loadHeader()
		if sz >= min {
			if sz == min && closed {
				return true, io.EOF
			} else {
				return true, nil
			}
		}
		if closed {
			return false, io.EOF
		}
		remainingTime := deadline - md.Monotime()
		if remainingTime <= 0 {
			return false, ErrTimeout
		}
		sleepTime = minDuration(sleepTime*2, remainingTime, maxSleepTime)
		time.Sleep(sleepTime)
	}
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
	if timeout < 0 {
		return false, ErrTimeout
	}

	deadline := calcDeadline(timeout)
	sleepTime := initialSleepTime

	for {
		_, closed, _, sz := b.loadHeader()
		if closed {
			return false, io.ErrClosedPipe
		}
		if b.Cap()-sz >= min {
			return true, nil
		}
		remainingTime := deadline - md.Monotime()
		if remainingTime <= 0 {
			return false, ErrTimeout
		}
		sleepTime = minDuration(sleepTime*2, remainingTime, maxSleepTime)
		time.Sleep(sleepTime)
	}
}

// SkipWithTimeout similar to Read but discards readed data
//	returns number of bytes skipped
// Possible errors
//	io.EOF if pipe was closed
//	ErrTimeout if timeout is expired
func (b *RingBuf) SkipWithTimeout(toSkip uint32, timeout time.Duration) (uint32, error) {
	if toSkip == 0 {
		return 0, nil
	}
	skipped := uint32(0)
	deadline := calcDeadline(timeout)
	sleepTime := initialSleepTime
	for skipped < toSkip {
		hs, closed, head, sz := b.loadHeader()
		if sz > 0 {
			n := minU32(toSkip-skipped, sz)
			skipped += n
			for {
				head = (head + n) & b.mask
				sz -= n
				if atomic.CompareAndSwapUint64(&b.headAndSize, hs, (hs&closeFlag)|(uint64(head)<<32)|uint64(sz)) {
					break
				}
				hs, closed, head, sz = b.loadHeader()
			}
			//b.updateHeaderAfterRead(hs, n)
			sleepTime = initialSleepTime // reset sleep time
		} else {
			if closed {
				return skipped, io.EOF
			}
			remainingTime := deadline - md.Monotime()
			if remainingTime <= 0 {
				return skipped, ErrTimeout
			}
			sleepTime = minDuration(sleepTime*2, remainingTime, maxSleepTime)
			time.Sleep(sleepTime)
		}
	}
	return skipped, nil
}

// Skip skip specified number of bytes
//	Returns number of skipped bytes
//	Possible errors:
//		see SkipWithTimeout
func (b *RingBuf) Skip(toSkip uint32) (uint32, error) {
	return b.SkipWithTimeout(toSkip, 0)
}

// Avail returns number of bytes available to read and write
func (b *RingBuf) Avail() (readAvail uint32, writeAvail uint32) {
	_, _, _, readAvail = b.loadHeader()
	writeAvail = b.Cap() - readAvail
	return
}

// ReadAvaial returns number of bytes availbale to immediate read
func (b *RingBuf) ReadAvail() uint32 {
	return uint32(atomic.LoadUint64(&b.headAndSize) & uint64(low31bits))
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
// possible errors: io.EOF if buffer was closed
func (b *RingBuf) Write(data []byte) (int, error) {
	written := uint32(0)
	toWrite := uint32(len(data))
	sleepTime := initialSleepTime
	for written < toWrite {
		_, closed, head, sz := b.loadHeader()
		if closed {
			return int(written), io.EOF
		}
		nw := minU32(b.Cap()-sz, toWrite-written)
		if nw == 0 {
			sleepTime *= 2
			if sleepTime > maxSleepTime {
				sleepTime = maxSleepTime
			}
			time.Sleep(sleepTime)
			continue
		}
		sleepTime = initialSleepTime // reset sleep time

		writePos := (head + sz) & b.mask
		if writePos+nw > b.Cap() {
			// wrapped
			ll := b.Cap() - writePos
			copy(b.mem[writePos:], data[written:written+ll])
			copy(b.mem[:nw-ll], data[written+ll:written+nw])
		} else {
			copy(b.mem[writePos:writePos+nw], data[written:written+nw])
		}
		written += nw
		atomic.AddUint64(&b.headAndSize, uint64(nw))
		/*for {
			if atomic.CompareAndSwapUint64(&b.headAndSize, hs, hs+uint64(nw)) {
				break
			}
			runtime.Gosched()
			hs = atomic.LoadUint64(&b.headAndSize)
		}*/
	}
	return int(written), nil
}

// WriteChunks is optimized Write for multiple byte chunks
//	possible errors: io.EOF if buffer was closed
func (b *RingBuf) WriteChunks(chunks ...[]byte) (int, error) {
	var totalWritten int

	sleepTime := initialSleepTime
	for _, data := range chunks {
		written := uint32(0)
		toWrite := uint32(len(data))
		for written < toWrite {
			hs, closed, head, sz := b.loadHeader()
			if closed {
				return totalWritten, io.EOF
			}
			nw := minU32(b.Cap()-sz, toWrite-written)
			if nw == 0 {
				sleepTime *= 2
				if sleepTime > maxSleepTime {
					sleepTime = maxSleepTime
				}
				time.Sleep(sleepTime)
				continue
			}
			sleepTime = initialSleepTime // reset sleep time
			writePos := (head + sz) & b.mask
			if writePos+nw > b.Cap() {
				// wrapped
				ll := b.Cap() - writePos
				copy(b.mem[writePos:], data[written:written+ll])
				copy(b.mem[:nw-ll], data[written+ll:written+nw])
			} else {
				copy(b.mem[writePos:writePos+nw], data[written:written+nw])
			}
			written += nw
			for {
				if atomic.CompareAndSwapUint64(&b.headAndSize, hs, hs+uint64(nw)) {
					break
				}
				hs = atomic.LoadUint64(&b.headAndSize)
			}
		}
		totalWritten += int(written)
	}
	return totalWritten, nil
}

// Write all data in one operation. Equal to WriteWait(len(data)); Write(data)
func (b *RingBuf) WriteAll(data []byte) (int, error) {
	toWrite := uint32(len(data))
	if toWrite > b.Cap() {
		return 0, ErrOvercap
	}

	sleepTime := initialSleepTime
	for {
		hs, closed, head, sz := b.loadHeader()
		if closed {
			return 0, io.EOF
		}
		if b.Cap()-sz >= toWrite {
			writePos := (head + sz) & b.mask
			if writePos+toWrite > b.Cap() {
				// wrapped
				ll := b.Cap() - writePos
				copy(b.mem[writePos:], data[:ll])
				copy(b.mem[:toWrite-ll], data[ll:toWrite])
			} else {
				copy(b.mem[writePos:writePos+toWrite], data)
			}
			for {
				if atomic.CompareAndSwapUint64(&b.headAndSize, hs, hs+uint64(toWrite)) {
					return int(toWrite), nil
				}
				runtime.Gosched()
				hs = atomic.LoadUint64(&b.headAndSize)
			}
		}
		sleepTime *= 2
		if sleepTime > maxSleepTime {
			sleepTime = maxSleepTime
		}
		time.Sleep(sleepTime)
	}
}

// Read all data in one operation. Equal to ReadWait(len(data)); Read(data)
func (b *RingBuf) ReadAll(data []byte, timeout time.Duration) (int, error) {
	nr := uint32(len(data))
	if nr > b.Cap() {
		return 0, ErrOvercap
	}
	sleepTime := initialSleepTime
	deadline := calcDeadline(timeout)
	for {
		hs, closed, head, sz := b.loadHeader()
		if sz >= nr {
			sleepTime = initialSleepTime // reset sleep time
			if head+nr > b.Cap() {
				// wrapped
				ll := b.Cap() - head
				copy(data[:ll], b.mem[head:])
				copy(data[ll:], b.mem[:nr-ll])
			} else {
				copy(data, b.mem[head:head+nr])
			}
			for {
				head = (head + nr) & b.mask
				sz -= nr
				if atomic.CompareAndSwapUint64(&b.headAndSize, hs, (hs&closeFlag)|(uint64(head)<<32)|uint64(sz)) {
					return int(nr), nil
				}
				runtime.Gosched()
				hs, closed, head, sz = b.loadHeader()
			}
			//b.updateHeaderAfterRead(hs, toRead)
			//return int(toRead), nil
		}
		if closed {
			return 0, io.EOF
		}
		remainingTime := deadline - md.Monotime()
		if remainingTime <= 0 {
			return 0, ErrTimeout
		}
		sleepTime = minDuration(sleepTime*2, remainingTime, maxSleepTime)
		time.Sleep(sleepTime)
	}
}
