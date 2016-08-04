package rb

import (
	"io"
	"runtime"
	"sync/atomic"
	"time"
)

const infinite = time.Duration(0x7FFFFFFFFFFFFFFF)
const pollPeriod = 10 * time.Nanosecond

type RingBuf struct {
	headAndSize uint64
	wwc         int64
	rwc         int64

	mem          []byte
	max          uint32
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	wt time.Timer
	rt time.Timer

	ws, rs time.Duration
}

func NewRingBuf(max uint32) *RingBuf {
	return &RingBuf{
		mem: make([]byte, int(max), int(max)),
		max: max,
	}
}

func badRead() {
	panic("inconsistent RingBuf.read")
}

func minU32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

func (b *RingBuf) loadHS() (hs uint64, head uint32, readAvail uint32) {
	hs = atomic.LoadUint64(&b.headAndSize)
	head = uint32(hs >> 32)
	readAvail = uint32(hs)
	return
}

func (b *RingBuf) read(data []byte, moveReadPosition bool) uint32 {
	toRead := uint32(len(data))
	hs, head, sz := b.loadHS()

	if toRead > sz {
		toRead = sz
	}
	if head+toRead > b.max {
		// wrapped
		ll := b.max - head
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
				head = (head + toRead) % b.max
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

func (b *RingBuf) ReadWithTimeout(data []byte, timeout time.Duration) (int, error) {
	toRead := uint32(len(data))
	ra := b.ReadAvail()
	if ra >= toRead {
		// easy case: have enough data to read
		nr := b.read(data, true)
		if nr != toRead {
			badRead()
		}
		return len(data), nil
	}

	// hard case: no enough data. read till timeout
	/*
		atomic.AddInt64(&b.rwc, 1)
		periods := int64(timeout / pollPeriod)
		readed := uint32(0)
		for i := int64(0); i <= periods; i++ { // <= periods to do at least one read
			ra = minU32(b.ReadAvail(), toRead-readed)
			if ra > 0 {
				nr := b.read(data[readed:readed+ra], true)
				if nr != ra {
					badRead()
				}
				readed += nr
				if readed == toRead {
					return int(readed), nil
				}
			}
			if i != periods {
				time.Sleep(pollPeriod)
				b.rs += pollPeriod
			}
		}
	*/
	tmr := time.NewTimer(timeout)
	readed := uint32(0)
	for readed < toRead {
		ra = minU32(b.ReadAvail(), toRead-readed)
		if ra > 0 {
			nr := b.read(data[readed:readed+ra], true)
			if nr != ra {
				badRead()
			}
			readed += nr
			if readed == toRead {
				return int(readed), nil
			}
		}
		select {
		case <-tmr.C:
			return int(readed), io.EOF
		}
		runtime.Gosched()
	}
	return int(readed), io.EOF
}

func (b *RingBuf) Read(data []byte) (int, error) {
	return b.ReadWithTimeout(data, b.ReadTimeout)
}

// ReadAll read all or nothing from RingBuf. wait for ReadTimeout for data if there is no enough data
func (b *RingBuf) ReadAll(data []byte) bool {
	ra := b.ReadAvail()
	if ra < uint32(len(data)) {
		if b.ReadTimeout == 0 || !b.ReadWait(uint32(len(data)), b.ReadTimeout) {
			return false
		}
	}
	nr := b.read(data, true)
	if nr != uint32(len(data)) {
		badRead()
	}
	return true
}

// ReadAtLeast reads available data. if min == 0 then read timeout is not used else wait up to ReadTimeout for min bytes available to read
// returns number bytes readed. if there is no min bytes readed then no read performed at all
func (b *RingBuf) ReadAtLeast(data []byte, min uint32) uint32 {
	ra := b.ReadAvail()
	if ra == 0 && min == 0 {
		return 0 // instant return if there is no data to read and min read == 0
	}
	if (ra >= min) || (b.ReadTimeout == 0) || b.ReadWait(min, b.ReadTimeout) {
		return b.read(data, true)
	}
	return 0
}

// Peek for avialable data. ReadTimeout is not used
func (b *RingBuf) Peek(data []byte) uint32 {
	return b.read(data, false)
}

// ReadWait waits for available data for specified timeout. return true if there is enough data or false otherwise
func (b *RingBuf) ReadWait(min uint32, timeout time.Duration) bool {
	if min == 0 {
		min = 1
	}
	if b.ReadAvail() >= min {
		return true
	}
	if timeout == 0 {
		return false
	}

	periods := int64(timeout / pollPeriod)

	for i := int64(0); i <= periods; i++ {
		if b.ReadAvail() >= min {
			return true
		}
		if i != periods {
			time.Sleep(pollPeriod)
			b.rs += pollPeriod
		}
	}
	return false
}

func (b *RingBuf) WriteWait(min uint32, timeout time.Duration) bool {
	if min == 0 {
		min = 1
	}
	if b.WriteAvail() >= min {
		return true
	}

	periods := int64(timeout / pollPeriod)
	for i := int64(0); i <= periods; i++ {
		if b.WriteAvail() >= min {
			return true
		}
		if i != periods {
			time.Sleep(pollPeriod)
			b.ws += pollPeriod
		}
	}
	return false
}

func (b *RingBuf) Skip(n uint32) uint32 {
	if n == 0 {
		return 0
	}
	hs, head, sz := b.loadHS()
	if n > sz {
		n = sz
	}
	for {
		sz -= n
		if sz == 0 {
			head = 0
		} else {
			head = (head + n) % b.max
		}
		nhs := (uint64(head) << 32) | uint64(sz)
		if atomic.CompareAndSwapUint64(&b.headAndSize, hs, nhs) {
			break
		}
		runtime.Gosched()
		hs, head, sz = b.loadHS()
	}
	return n
}

func (b *RingBuf) ReadAvail() uint32 {
	hs := atomic.LoadUint64(&b.headAndSize)
	return uint32(hs)
}

func (b *RingBuf) WriteAvail() uint32 {
	return b.max - b.ReadAvail()
}

func (b *RingBuf) Cap() uint32 {
	return b.max
}

func (b *RingBuf) WriteString(s string) (int, error) {
	return b.Write([]byte(s))
}

func (b *RingBuf) ReadString(maxSize int) (string, error) {
	if maxSize == 0 {
		return "", nil
	}
	data := make([]byte, maxSize, maxSize)
	nr, err := b.Read(data)
	return string(data[:nr]), err
}

func (b *RingBuf) write(data []byte) uint32 {
	hs, head, sz := b.loadHS()
	writePos := (head + sz) % b.max
	toWrite := minU32(b.max-sz, uint32(len(data)))
	if writePos+toWrite > b.max {
		// wrapped
		ll := b.max - writePos
		if ll > 0 {
			copy(b.mem[writePos:b.max], data[0:ll])
			copy(b.mem[0:toWrite-ll], data[ll:toWrite])
		} else {
			copy(b.mem[0:toWrite], data)
		}
	} else {
		copy(b.mem[writePos:writePos+toWrite], data)
	}
	for {
		sz += toWrite
		nhs := (uint64(head) << 32) | uint64(sz)
		if atomic.CompareAndSwapUint64(&b.headAndSize, hs, nhs) {
			break
		}
		runtime.Gosched()
		hs, head, sz = b.loadHS()
	}
	return toWrite
}

// Write data to ringbuf. Returns number of bytes written
func (b *RingBuf) WriteWithTimeout(data []byte, timeout time.Duration) (int, error) {
	toWrite := uint32(len(data))
	if b.WriteAvail() >= toWrite {
		// easy case: there is enough space in buffer
		b.write(data)
		return int(toWrite), nil
	}

	atomic.AddInt64(&b.wwc, 1)
	written := uint32(0)
	periods := int64(timeout / pollPeriod)
	for i := int64(0); i <= periods; i++ {
		wa := b.WriteAvail()
		if wa > 0 {
			written += b.write(data[written:toWrite])
			if written == toWrite {
				break
			}
		}
		if i != periods {
			time.Sleep(pollPeriod)
			b.ws += pollPeriod
		}
	}

	return int(written), nil
}

func (b *RingBuf) Write(data []byte) (int, error) {
	return b.WriteWithTimeout(data, b.WriteTimeout)
}
