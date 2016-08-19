// Pipe is a buffered async half duplex byte channel.
// Multiple writes are serialized.
// Multiple reads are NOT serialized (and won't be because I don't understand reads from multiple goroutines at all)
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
	ErrOvercap = errors.New("Pipe buffer overcap")
	ErrTimeout = timeoutError{}
)

//  0 - no debug checks and info
// 	1 - check
//	2 - check + print extra info
const debug = 0

type Pipe struct {
	bits          uint64 // highest bit - close flag. next 31 bits: read pos, next bit - write lock flag (if any), next 31 bits: read avail
	mem           []byte
	mask          int
	readDeadline  time.Duration
	writeDeadline time.Duration

	rst, wst time.Duration
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
const maxSleepTime = 64 * initialSleepTime

const NoTimeout = time.Duration(-1)

const closeFlag = low63bits + 1
const wlockFlag = uint64(low31bits) + 1
const negWlockFlag = uint64((-int64(low31bits+1))&int64(low63bits)) | (low63bits + 1)

const headerFlagMask = closeFlag | wlockFlag

const defaultBufferSize = 32 * 1024
const minBufferSize = 8

const spinCycles = 10000

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
	return With(make([]byte, max))
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

func (p *Pipe) SetReadDeadline(deadline time.Time) {
	if deadline.IsZero() {
		p.readDeadline = 0
	} else {
		p.readDeadline = deadline.Sub(time.Now())
	}
}

func (p *Pipe) SetWriteDeadline(deadline time.Time) {
	if deadline.IsZero() {
		p.writeDeadline = 0
	} else {
		p.writeDeadline = deadline.Sub(time.Now())
	}
}

func (p *Pipe) SetDeadline(deadline time.Time) {
	p.SetReadDeadline(deadline)
	p.writeDeadline = p.readDeadline
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (p *Pipe) loadHeader() (hs uint64, closed bool, readPos int, readAvail int) {
	hs = atomic.LoadUint64(&p.bits)
	closed = (hs & closeFlag) != 0
	readPos = int((hs >> 32) & uint64(low31bits))
	readAvail = int(hs & uint64(low31bits))
	return
}

func (p *Pipe) Close() error {
	for {
		hs := atomic.LoadUint64(&p.bits)
		if atomic.CompareAndSwapUint64(&p.bits, hs, hs|closeFlag) {
			return nil
		}
		runtime.Gosched()
	}
	return nil
}

func (p *Pipe) Reopen() {
	atomic.StoreUint64(&p.bits, 0) // reset close flag, head and size
}

func (p *Pipe) IsClosed() bool {
	return (atomic.LoadUint64(&p.bits) & closeFlag) != 0
}

func (p *Pipe) Clear() {
	atomic.StoreUint64(&p.bits, 0)
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

// Read bytes into buffer, return number of bytes readed and error (if any)
// Errors:
//	io.EOF if pipe was closed (but data still can be there)
//	ErrTimeout if read deadline is reached
func (p *Pipe) Read(data []byte) (int, error) {
	readed := 0
	toRead := len(data)
	sleepTime := initialSleepTime
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
				if atomic.CompareAndSwapUint64(&p.bits, hs, nhs) {
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
			if p.readDeadline == 0 {
				if sleepTime < maxSleepTime {
					sleepTime *= 2
				}
			} else {
				remainingTime := p.readDeadline - md.Monotime()
				if remainingTime < time.Microsecond {
					return readed, ErrTimeout
				}
				sleepTime = minDuration(sleepTime*2, remainingTime, maxSleepTime)
			}
			time.Sleep(sleepTime)
			p.rst += sleepTime
		}
	}
	return readed, nil
}

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

// ReadWait waits for sepcified number of data bytes.
// Returns:
//	true, nil if the wait was successfull
//	true, io.EOF if pipe is closed and reamining number of bytes is equal to min
//	false, ErrTimeout if read deadline is reached
//	false, io.EOF if pipe is closed
//  false, ErrOvercap if the specified number of bytes is greater than the buffer capacity
func (p *Pipe) ReadWait(min int) (bool, error) {
	if min > p.Cap() {
		return false, ErrOvercap
	}
	if min < 1 {
		min = 1
	}
	if p.ReadAvail() >= min {
		return true, nil
	}
	if p.readDeadline < 0 {
		return false, ErrTimeout
	}

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
		if p.readDeadline == 0 {
			if sleepTime < maxSleepTime {
				sleepTime *= 2
			}
		} else {
			remainingTime := p.readDeadline - md.Monotime()
			if remainingTime < time.Microsecond {
				return false, ErrTimeout
			}
			sleepTime = minDuration(sleepTime*2, remainingTime, maxSleepTime)
		}
		time.Sleep(sleepTime)
	}
}

// Skip similar to Read but discards readed data.
// Return number of bytes skipped and error
// Possible errors
//	io.EOF if pipe was closed
//	ErrTimeout if read deadline is reached
func (p *Pipe) Skip(toSkip int) (int, error) {
	if toSkip == 0 {
		return 0, nil
	}
	skipped := 0
	sleepTime := initialSleepTime
	for skipped < toSkip {
		hs, closed, head, sz := p.loadHeader()
		if sz > 0 {
			n := minInt(toSkip-skipped, sz)
			skipped += n
			for {
				head = (head + n) & p.mask
				sz -= n
				if atomic.CompareAndSwapUint64(&p.bits, hs, (hs&headerFlagMask)|(uint64(head)<<32)|uint64(sz)) {
					break
				}
				runtime.Gosched()
				hs, closed, head, sz = p.loadHeader()
			}
			sleepTime = initialSleepTime // reset sleep time
		} else {
			if closed {
				return skipped, io.EOF
			}
			if p.readDeadline == 0 {
				if sleepTime < maxSleepTime {
					sleepTime *= 2
				}
			} else {
				remainingTime := p.readDeadline - md.Monotime()
				if remainingTime < time.Microsecond {
					return skipped, ErrTimeout
				}
				sleepTime = minDuration(sleepTime*2, remainingTime, maxSleepTime)
			}
			time.Sleep(sleepTime)
		}
	}
	return skipped, nil
}

// SkipAll discards all pending input
func (p *Pipe) SkipAll() (int, error) {
	for {
		hs, closed, head, sz := p.loadHeader()
		head = (head + sz) & p.mask
		if atomic.CompareAndSwapUint64(&p.bits, hs, (hs&headerFlagMask)|(uint64(head)<<32)) {
			if closed {
				return sz, io.EOF
			}
			return sz, nil
		}
		runtime.Gosched()
	}
}

// ReadAvaial returns number of bytes availbale to immediate read
func (p *Pipe) ReadAvail() int {
	return int(atomic.LoadUint64(&p.bits) & uint64(low31bits))
}

// Cap returns capacity of the buffer
func (p *Pipe) Cap() int {
	return len(p.mem)
}

func (p *Pipe) writeLock() {
	// first spin some
	for i := 0; i < spinCycles; i++ {
		hs := atomic.LoadUint64(&p.bits)
		if ((hs & wlockFlag) == 0) && atomic.CompareAndSwapUint64(&p.bits, hs, hs|wlockFlag) {
			return
		}
		time.Sleep(time.Microsecond)
	}
	sleepTime := initialSleepTime
	for {
		hs := atomic.LoadUint64(&p.bits)
		if ((hs & wlockFlag) == 0) && atomic.CompareAndSwapUint64(&p.bits, hs, hs|wlockFlag) {
			return
		}
		if sleepTime < maxSleepTime {
			sleepTime *= 2
		}
		time.Sleep(sleepTime)
	}
}

func (p *Pipe) writeUnlock() {
	if debug > 0 {
		if (atomic.LoadUint64(&p.bits) & wlockFlag) == 0 {
			panic("unlocking not locked pipe")
		}
	}
	atomic.AddUint64(&p.bits, negWlockFlag)
}

// Write writes bytes to buffer. Function return when all bytes written or timeout expired
// Errors:
//	io.EOF if buffer was closed
//	ErrTimeout if write deadline is reached.
func (p *Pipe) write(data []byte, alreadyLocked bool) (int, error) {
	// we don't use defer to unlock because it slows write x3 times
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
	locked := alreadyLocked
	for {
		hs, closed, head, sz := p.loadHeader()
		if closed {
			if locked && !alreadyLocked {
				atomic.AddUint64(&p.bits, negWlockFlag)
			}
			return written, io.EOF
		}

		if !locked && ((hs & wlockFlag) == 0) {
			locked = atomic.CompareAndSwapUint64(&p.bits, hs, hs|wlockFlag)
		}

		if locked {
			nw := minInt(p.Cap()-sz, toWrite-written)
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
				written += nw
				atomic.AddUint64(&p.bits, uint64(nw))
				if written == toWrite {
					atomic.AddUint64(&p.bits, negWlockFlag)
					return written, nil
				}
				sleepTime = initialSleepTime // reset sleep time
				// fallback to sleep because there is no enough space to write
			}
		}

		if p.writeDeadline == 0 {
			if sleepTime < maxSleepTime {
				sleepTime *= 2
			}
		} else {
			remainTime := p.writeDeadline - md.Monotime()
			if remainTime < time.Microsecond {
				if locked && !alreadyLocked {
					atomic.AddUint64(&p.bits, negWlockFlag)
				}
				return written, ErrTimeout
			}
			sleepTime = minDuration(sleepTime*2, maxSleepTime, remainTime)
		}
		time.Sleep(sleepTime)
		p.wst += sleepTime
	}
}

// Write writes bytes to buffer. Function will not return till all bytes is written or error occured.
// Errors:
//	io.EOF if pipe was closed
//	ErrTimeout if write deadline is reached
func (p *Pipe) Write(data []byte) (int, error) {
	return p.write(data, false)
}

// WriteAll sequentially writes all passed data.
//	Return number of written bytes and error
//	Errors:
//		io.EOF if pipe was closed
//		ErrTimeout if write deadline is reached
func (p *Pipe) WriteAll(chunks ...[]byte) (int64, error) {
	var totalWritten int64

	sleepTime := initialSleepTime
	locked := false
	for _, data := range chunks {
		written := 0
		toWrite := len(data)
		for written < toWrite {
			hs, closed, head, sz := p.loadHeader()
			if closed {
				if locked {
					atomic.AddUint64(&p.bits, negWlockFlag)
				}
				return totalWritten, io.EOF
			}
			if !locked && ((hs & wlockFlag) == 0) {
				locked = atomic.CompareAndSwapUint64(&p.bits, hs, hs|wlockFlag)
			}
			if locked {
				nw := minInt(p.Cap()-sz, toWrite-written)
				if nw > 0 {
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
					atomic.AddUint64(&p.bits, uint64(nw))
				}
			}
			if written < toWrite {
				if p.writeDeadline == 0 {
					if sleepTime < maxSleepTime {
						sleepTime *= 2
					}
				} else {
					remainTime := p.writeDeadline - md.Monotime()
					if remainTime < time.Microsecond {
						if locked {
							atomic.AddUint64(&p.bits, negWlockFlag)
						}
						return totalWritten + int64(written), ErrTimeout
					}
					sleepTime = minDuration(sleepTime*2, maxSleepTime, remainTime)
				}
				time.Sleep(sleepTime)
			}
		}
		totalWritten += int64(written)
	}
	if locked {
		atomic.AddUint64(&p.bits, negWlockFlag)
	}
	return totalWritten, nil
}

// ReadFrom see io.ReaderFrom
func (p *Pipe) ReadFrom(r io.Reader) (n int64, err error) {
	chunk := make([]byte, 8192)
	p.writeLock()
	defer p.writeUnlock()
	for {
		readed, err := r.Read(chunk)
		if readed > 0 {
			written, werr := p.write(chunk[:readed], true)
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
