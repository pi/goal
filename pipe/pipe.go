// Pipe is a buffered async half duplex byte channel.
// Multiple writers are serialized.
// Multiple readers are NOT serialized.
// It is safe to read/write in parallel.
// It is safe to close in parallel with read/write.
// Supported interfaces:
//	io.Closer
//	io.Reader
//	io.ReadCloser
//	io.Writer
//	io.WriteCloser
//	io.ByteReader
//	io.ByteWriter
//	io.ReaderFrom
//	io.WriterTo
//
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

var (
	ErrOvercap = errors.New("Buffer overcap")
	ErrTimeout = timeoutError{}
)

// 	1 - check
//	2 - +print
const debug = 0

type Pipe struct {
	headAndSize uint64 // highest bit - close flag. next 31 bits: read pos, next bit - write lock flag (if any), next 31 bits: read avail
	mem         []byte
	mask        int
}

const low63bits = ^uint64(0) >> 1
const low31bits = ^uint32(0) >> 1

type timeoutError struct{}

// Timeout error conforms to net.Error
func (timeoutError) Error() string     { return "i/o timeout" }
func (timeoutError) IsTimeout() bool   { return true }
func (timeoutError) IsTemporary() bool { return true }

const infinite = time.Duration(int64(low63bits))
const initialSleepTime = time.Microsecond
const maxSleepTime = 100 * time.Millisecond

const NoTimeout = time.Duration(-1)

const closeFlag = low63bits + 1
const wlockFlag = uint64(low31bits) + 1
const negWlockFlag = uint64((-int64(low31bits+1))&int64(low63bits)) | (low63bits + 1)

const headerFlagMask = closeFlag | wlockFlag

const defaultBufferSize = 32 * 1024
const minBufferSize = 8

func calcDeadline(timeout time.Duration) (deadline time.Duration) {
	if timeout == 0 {
		return infinite
	} else if timeout < 0 {
		return 0 // nowait
	}
	deadline = md.Monotime() + timeout
	if deadline < timeout {
		// overflow
		deadline = infinite
	}
	return
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

func New(max int) *Pipe {
	if max == 0 {
		max = defaultBufferSize
	} else if max < minBufferSize {
		max = minBufferSize
	} else if (max & (max - 1)) != 0 {
		// round up to power of two
		max = 1 << gut.BitLen(uint(max))
	}
	if int(low31bits) < max-1 {
		panic("Pipe size is too large")
	}
	return &Pipe{
		mem:  make([]byte, max),
		mask: max - 1,
	}
}

func With(buf []byte) *Pipe {
	max := len(buf)
	if (max & (max - 1)) != 0 {
		panic("Buffer size must be power of two")
	}
	if int(low31bits) < max-1 {
		panic("Buffer size is too large")
	}
	return &Pipe{
		mem:  buf,
		mask: max - 1,
	}
}

func badRead() {
	panic("inconsistent Pipe.Read")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (p *Pipe) loadHeader() (hs uint64, closed bool, readPos int, readAvail int) {
	hs = atomic.LoadUint64(&p.headAndSize)
	closed = (hs & closeFlag) != 0
	readPos = int((hs >> 32) & uint64(low31bits))
	readAvail = int(hs & uint64(low31bits))
	return
}

func (p *Pipe) Close() error {
	for {
		hs := atomic.LoadUint64(&p.headAndSize)
		if atomic.CompareAndSwapUint64(&p.headAndSize, hs, hs|closeFlag) {
			return nil
		}
		runtime.Gosched()
	}
	return nil
}

func (p *Pipe) Reopen() {
	atomic.StoreUint64(&p.headAndSize, 0) // reset close flag, head and size
}

func (p *Pipe) IsClosed() bool {
	return (atomic.LoadUint64(&p.headAndSize) & closeFlag) != 0
}

func (p *Pipe) Clear() {
	atomic.StoreUint64(&p.headAndSize, 0)
}

func (p *Pipe) ReadByte() (byte, error) {
	var buf [1]byte
	_, err := p.Read(buf[:])
	return buf[0], err
}

func (p *Pipe) WriteByte(c byte) error {
	var buf [1]byte
	buf[0] = c
	_, err := p.Write(buf[:])
	return err
}

// basic read function
func (p *Pipe) read(data []byte, timeout time.Duration) (int, error) {
	if debug > 1 {
		hs, closed, head, sz := p.loadHeader()
		println("read", len(data), timeout, hs, closed, head, sz)
	}
	readed := 0
	toRead := len(data)
	sleepTime := initialSleepTime
	deadline := calcDeadline(timeout)
	noDeadline := deadline == infinite
	for {
		hs, closed, head, sz := p.loadHeader()
		if sz > 0 {
			nr := minInt(sz, toRead-readed)
			if head > p.Cap()-nr {
				// wrapped
				ll := p.Cap() - head
				copy(data[readed:readed+ll], p.mem[head:])
				copy(data[readed+ll:readed+nr], p.mem[:nr-ll])
			} else {
				copy(data[readed:readed+nr], p.mem[head:head+nr])
			}
			for {
				head = (head + nr) & p.mask
				sz -= nr
				nhs := (hs & headerFlagMask) | (uint64(head) << 32) | uint64(sz)
				if atomic.CompareAndSwapUint64(&p.headAndSize, hs, nhs) {
					break
				}
				runtime.Gosched()
				hs, closed, head, sz = p.loadHeader()
			}
			readed += nr
			if readed == toRead {
				return readed, nil
			}
			sleepTime = initialSleepTime // reset sleep time
		} else {
			if closed {
				return readed, io.EOF
			}
			if noDeadline {
				if sleepTime < maxSleepTime {
					sleepTime *= 2
				}
			} else {
				remainingTime := deadline - md.Monotime()
				if remainingTime <= 0 {
					return readed, ErrTimeout
				}
				sleepTime = minDuration(sleepTime*2, remainingTime, maxSleepTime)
			}
			time.Sleep(sleepTime)
		}
	}
	return readed, nil
}

// ReadWithTimeout read bytes with the specified timeout
// Returns number of bytes readed and error (if any)
//	Errors:
//		io.EOF if pipe was closed (there still can be data)
//		ErrTimeout if timeout expired
func (p *Pipe) ReadWithTimeout(data []byte, timeout time.Duration) (int, error) {
	return p.read(data, timeout)
}

// Read bytes into buffer, return number of bytes readed and error (if any)
// Errors:
//	io.EOF if pipe was closed (but data still can be there)
func (p *Pipe) Read(buf []byte) (int, error) {
	return p.read(buf, 0)
}

/*func (p *Pipe) Read(buf []byte) (int, error) {
	readed := 0
	toRead := len(buf)
	sleepTime := initialSleepTime
	for {
		hs, closed, head, sz := p.loadHeader()
		if sz > 0 {
			sleepTime = initialSleepTime // reset sleep time
			nr := minInt(sz, toRead-readed)
			if head > p.Cap()-nr {
				// wrapped
				ll := p.Cap() - head
				copy(buf[readed:readed+ll], p.mem[head:])
				copy(buf[readed+ll:readed+nr], p.mem[:nr-ll])
			} else {
				copy(buf[readed:readed+nr], p.mem[head:head+nr])
			}
			readed += nr
			for {
				head = (head + nr) & p.mask
				sz -= nr
				nhs := (hs & headerFlagMask) | (uint64(head) << 32) | uint64(sz)
				if atomic.CompareAndSwapUint64(&p.headAndSize, hs, nhs) {
					break
				}
				runtime.Gosched()
				hs, closed, head, sz = p.loadHeader()
			}
			if readed == toRead {
				if closed {
					return readed, io.EOF
				} else {
					return readed, nil
				}
			}
		} else {
			if closed {
				return readed, io.EOF
			}
			if sleepTime >= maxSleepTime {
				sleepTime = maxSleepTime
			} else {
				sleepTime *= 2
			}
			time.Sleep(sleepTime)
		}
	}
	return readed, nil
}*/

// Peek for avialable data. ReadTimeout is not used, close flag is not handled
func (p *Pipe) Peek(data []byte) int {
	_, _, head, sz := p.loadHeader()
	l := minInt(sz, len(data))
	if l == 0 {
		return 0
	}
	if head > p.Cap()-l {
		// wrap
		ll := p.Cap() - head
		copy(data[:ll], p.mem[head:])
		copy(data[ll:l], p.mem[:l-ll])
	} else {
		copy(data, p.mem[head:head+l])
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
func (p *Pipe) ReadWait(min int, timeout time.Duration) (bool, error) {
	if min > p.Cap() {
		return false, ErrOvercap
	}
	if min < 1 {
		min = 1
	}
	if p.ReadAvail() >= min {
		return true, nil
	}
	if timeout < 0 {
		return false, ErrTimeout
	}

	deadline := calcDeadline(timeout)
	sleepTime := initialSleepTime
	for {
		_, closed, _, sz := p.loadHeader()
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
func (p *Pipe) WriteWait(min int, timeout time.Duration) (bool, error) {
	if min > p.Cap() {
		return false, ErrOvercap
	}
	if min < 1 {
		min = 1
	}
	if p.WriteAvail() >= min {
		return true, nil
	}
	if timeout < 0 {
		return false, ErrTimeout
	}

	deadline := calcDeadline(timeout)
	sleepTime := initialSleepTime

	for {
		_, closed, _, sz := p.loadHeader()
		if closed {
			return false, io.ErrClosedPipe
		}
		if min <= p.Cap()-sz {
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

// Skip similar to Read but discards readed data. Uses deadline specified by SetReadDeadline
//	returns number of bytes skipped
// Possible errors
//	io.EOF if pipe was closed
//	ErrTimeout if timeout is expired
func (p *Pipe) Skip(toSkip int, timeout time.Duration) (int, error) {
	if toSkip == 0 {
		return 0, nil
	}
	skipped := 0
	deadline := calcDeadline(timeout)
	noDeadline := deadline == infinite
	sleepTime := initialSleepTime
	for skipped < toSkip {
		hs, closed, head, sz := p.loadHeader()
		if sz > 0 {
			n := minInt(toSkip-skipped, sz)
			skipped += n
			for {
				head = (head + n) & p.mask
				sz -= n
				if atomic.CompareAndSwapUint64(&p.headAndSize, hs, (hs&headerFlagMask)|(uint64(head)<<32)|uint64(sz)) {
					break
				}
				hs, closed, head, sz = p.loadHeader()
			}
			sleepTime = initialSleepTime // reset sleep time
		} else {
			if closed {
				return skipped, io.EOF
			}
			if noDeadline {
				sleepTime = minDuration(sleepTime, maxSleepTime, infinite)
			} else {
				remainingTime := deadline - md.Monotime()
				if remainingTime <= 0 {
					return skipped, ErrTimeout
				}
				sleepTime = minDuration(sleepTime*2, remainingTime, maxSleepTime)
			}
			time.Sleep(sleepTime)
		}
	}
	return skipped, nil
}

// Avail returns number of bytes available to read and write
func (p *Pipe) Avail() (readAvail int, writeAvail int) {
	_, _, _, readAvail = p.loadHeader()
	writeAvail = p.Cap() - readAvail
	return
}

// ReadAvaial returns number of bytes availbale to immediate read
func (p *Pipe) ReadAvail() int {
	return int(atomic.LoadUint64(&p.headAndSize) & uint64(low31bits))
}

// WriteAvail returns number of bytes that can be written immediately
func (p *Pipe) WriteAvail() int {
	return p.Cap() - p.ReadAvail()
}

// Cap returns capacity of the buffer
func (p *Pipe) Cap() int {
	return len(p.mem)
}

// WriteString is sugar for Write(string(bytes))
func (p *Pipe) WriteString(s string) (int, error) {
	return p.Write([]byte(s))
}

// ReadString reads string from buffer. For errors see Pipe.Read
func (p *Pipe) ReadString(maxSize int) (string, error) {
	if maxSize == 0 {
		return "", nil
	}
	data := make([]byte, maxSize, maxSize)
	nr, err := p.Read(data)
	return string(data[:nr]), err
}

// basic write func
func (p *Pipe) _write(data []byte, timeout time.Duration) (int, error) {
	if debug > 1 {
		println("write", len(data), timeout)
	}
	written := 0
	toWrite := len(data)
	sleepTime := initialSleepTime
	deadline := calcDeadline(timeout)
	noDeadline := deadline == infinite
	for written < toWrite {
		_, closed, head, sz := p.loadHeader()
		if closed {
			return written, io.EOF
		}
		nw := minInt(p.Cap()-sz, toWrite-written)
		if nw == 0 {
			if noDeadline {
				sleepTime = minDuration(sleepTime, maxSleepTime, infinite)
			} else {
				remainTime := deadline - md.Monotime()
				if remainTime <= 0 {
					return written, ErrTimeout
				}
				sleepTime = minDuration(sleepTime*2, maxSleepTime, remainTime)
			}
			time.Sleep(sleepTime)
			continue
		}
		sleepTime = initialSleepTime // reset sleep time

		writePos := (head + sz) & p.mask
		if writePos > p.Cap()-nw {
			// wrapped
			ll := p.Cap() - writePos
			copy(p.mem[writePos:], data[written:written+ll])
			copy(p.mem[:nw-ll], data[written+ll:written+nw])
		} else {
			copy(p.mem[writePos:writePos+nw], data[written:written+nw])
		}
		written += nw
		atomic.AddUint64(&p.headAndSize, uint64(nw))
	}
	return written, nil
}

// basic write func
func (p *Pipe) write(data []byte, timeout time.Duration) (int, error) {
	// we don't use defer because it slows write x3 times
	toWrite := len(data)
	if toWrite == 0 {
		if p.IsClosed() {
			return 0, io.EOF
		} else {
			return 0, nil
		}
	}
	written := 0
	sleepTime := initialSleepTime
	deadline := calcDeadline(timeout)
	noDeadline := deadline == infinite
	locked := false
	/*hs := atomic.LoadUint64(&p.headAndSize)
	if (hs & wlockFlag) == 0 {
		locked = atomic.CompareAndSwapUint64(&p.headAndSize, hs, hs|wlockFlag)
	}*/

	for written < toWrite {
		hs, closed, head, sz := p.loadHeader()
		if closed {
			if locked {
				atomic.AddUint64(&p.headAndSize, negWlockFlag)
			}
			return written, io.EOF
		}
		if !locked && ((hs & wlockFlag) == 0) {
			// spin some
			for i := 0; i < 10000 && !locked; i++ {
				if (hs & wlockFlag) == 0 {
					locked = atomic.CompareAndSwapUint64(&p.headAndSize, hs, hs|wlockFlag)
					if locked {
						break
					}
				}
				runtime.Gosched()
				hs, closed, head, sz = p.loadHeader()
				if closed {
					return 0, io.EOF
				}
			}
		}
		var nw int
		if locked {
			nw = minInt(p.Cap()-sz, toWrite-written)
		} else {
			nw = 0
		}
		if nw == 0 {
			if noDeadline {
				if sleepTime < maxSleepTime {
					sleepTime *= 2
				}
			} else {
				remainTime := deadline - md.Monotime()
				if remainTime <= 0 {
					if locked {
						atomic.AddUint64(&p.headAndSize, negWlockFlag)
					}
					return written, ErrTimeout
				}
				sleepTime = minDuration(sleepTime*2, maxSleepTime, remainTime)
			}
			time.Sleep(sleepTime)
			continue
		}
		sleepTime = initialSleepTime // reset sleep time

		writePos := (head + sz) & p.mask
		if writePos > p.Cap()-nw {
			// wrapped
			ll := p.Cap() - writePos
			copy(p.mem[writePos:], data[written:written+ll])
			copy(p.mem[:nw-ll], data[written+ll:written+nw])
		} else {
			copy(p.mem[writePos:writePos+nw], data[written:written+nw])
		}
		written += nw
		atomic.AddUint64(&p.headAndSize, uint64(nw))
	}
	if locked {
		atomic.AddUint64(&p.headAndSize, negWlockFlag)
	}
	return written, nil
}

// Write writes bytes to buffer. Function will not return till all bytes is written or error occured.
// Errors:
//	io.EOF if pipe was closed
func (p *Pipe) Write(data []byte) (int, error) {
	return p.write(data, 0)
}

/*
func (p *Pipe) Write(data []byte) (int, error) {
	written := 0
	toWrite := len(data)
	if toWrite == 0 {
		return 0, nil
	}
	sleepTime := initialSleepTime
	for {
		_, closed, head, sz := p.loadHeader()
		if closed {
			return written, io.EOF
		}
		nw := p.Cap() - sz
		if nw > toWrite-written {
			nw = toWrite - written
		}
		if nw == 0 {
			if sleepTime < maxSleepTime {
				sleepTime *= 2
			}
			time.Sleep(sleepTime)
			continue
		}

		writePos := (head + sz) & p.mask
		if writePos > p.Cap()-nw {
			// wrapped
			ll := p.Cap() - writePos
			copy(p.mem[writePos:], data[written:written+ll])
			copy(p.mem[:nw-ll], data[written+ll:written+nw])
		} else {
			copy(p.mem[writePos:writePos+nw], data[written:written+nw])
		}
		written += nw
		atomic.AddUint64(&p.headAndSize, uint64(nw))
		if written == toWrite {
			return written, nil
		}
		sleepTime = initialSleepTime // reset sleep time
	}
}
*/
// Write writes bytes to buffer. Function return when all bytes written or timeout expired
// Errors:
//	io.EOF if buffer was closed
//	ErrTimeout if write deadline is reached.
func (p *Pipe) WriteWithTimeout(data []byte, timeout time.Duration) (int, error) {
	return p.write(data, timeout)
}

// WriteChunks is optimized Write for multiple byte chunks
//	possible errors: io.EOF if buffer was closed
func (p *Pipe) WriteChunks(chunks ...[]byte) (int, error) {
	var totalWritten int

	sleepTime := initialSleepTime
	for _, data := range chunks {
		written := 0
		toWrite := len(data)
		for written < toWrite {
			hs, closed, head, sz := p.loadHeader()
			if closed {
				return totalWritten, io.EOF
			}
			nw := minInt(p.Cap()-sz, toWrite-written)
			if nw == 0 {
				sleepTime = minDuration(sleepTime*2, maxSleepTime, infinite)
				time.Sleep(sleepTime)
				continue
			}
			sleepTime = initialSleepTime // reset sleep time
			writePos := (head + sz) & p.mask
			if writePos > p.Cap()-nw {
				// wrapped
				ll := p.Cap() - writePos
				copy(p.mem[writePos:], data[written:written+ll])
				copy(p.mem[:nw-ll], data[written+ll:written+nw])
			} else {
				copy(p.mem[writePos:writePos+nw], data[written:written+nw])
			}
			written += nw
			for {
				if atomic.CompareAndSwapUint64(&p.headAndSize, hs, hs+uint64(nw)) {
					break
				}
				hs = atomic.LoadUint64(&p.headAndSize)
			}
		}
		totalWritten += written
	}
	return totalWritten, nil
}

// Write all data in one operation. Equal to WriteWait(len(data)); Write(data)
func (p *Pipe) WriteAll(data []byte) (int, error) {
	if len(data) > p.Cap() {
		return 0, ErrOvercap
	}
	toWrite := len(data)
	sleepTime := initialSleepTime
	for {
		hs, closed, head, sz := p.loadHeader()
		if closed {
			return 0, io.EOF
		}
		if p.Cap()-sz >= toWrite {
			writePos := (head + sz) & p.mask
			if writePos > p.Cap()-toWrite {
				// wrapped
				ll := p.Cap() - writePos
				copy(p.mem[writePos:], data[:ll])
				copy(p.mem[:toWrite-ll], data[ll:toWrite])
			} else {
				copy(p.mem[writePos:writePos+toWrite], data)
			}
			for {
				if atomic.CompareAndSwapUint64(&p.headAndSize, hs, hs+uint64(toWrite)) {
					return toWrite, nil
				}
				runtime.Gosched()
				hs = atomic.LoadUint64(&p.headAndSize)
			}
		}
		sleepTime *= 2
		if sleepTime > maxSleepTime {
			sleepTime = maxSleepTime
		}
		time.Sleep(sleepTime)
	}
}

// Read all data in one transaction. Equal to ReadWait(len(data)); Read(data)
func (p *Pipe) ReadAll(data []byte, timeout time.Duration) (int, error) {
	if len(data) > p.Cap() {
		return 0, ErrOvercap
	}
	toRead := len(data)
	sleepTime := initialSleepTime
	deadline := calcDeadline(timeout)
	for {
		hs, closed, head, sz := p.loadHeader()
		if sz >= toRead {
			sleepTime = initialSleepTime // reset sleep time
			if head > p.Cap()-toRead {
				// wrapped
				ll := p.Cap() - head
				copy(data[:ll], p.mem[head:])
				copy(data[ll:], p.mem[:toRead-ll])
			} else {
				copy(data, p.mem[head:head+toRead])
			}
			for {
				head = (head + toRead) & p.mask
				sz -= toRead
				if atomic.CompareAndSwapUint64(&p.headAndSize, hs, (hs&headerFlagMask)|(uint64(head)<<32)|uint64(sz)) {
					return toRead, nil
				}
				runtime.Gosched()
				hs, closed, head, sz = p.loadHeader()
			}
			return toRead, nil
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

// ReadFrom see io.ReaderFrom
func (p *Pipe) ReadFrom(r io.Reader) (n int64, err error) {
	chunk := make([]byte, 8192)
	for {
		readed, err := r.Read(chunk)
		if readed > 0 {
			_, werr := p.Write(chunk[:readed])
			n += int64(readed)
			if werr != nil {
				return n, werr
			}
		}
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return n, err
		}
	}
}

// WriteTo see io.WriterTo
func (p *Pipe) WriteTo(w io.Writer) (n int64, err error) {
	chunk := make([]byte, 8192)
	for {
		readed, err := p.Read(chunk)
		if readed > 0 {
			written, werr := w.Write(chunk[:readed])
			n += int64(written)
			if werr != nil {
				return n, werr
			}
		}
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return n, err
		}
	}
}
