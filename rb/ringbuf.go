package rb

import (
	"io"
	"runtime"
	"sync/atomic"
	"time"
)

const infinite = time.Duration(0x7FFFFFFFFFFFFFFF)
const pollPeriod = 100 * time.Nanosecond

type RingBuf struct {
	headAndSize uint64
	mem         []byte
	max         uint32
	ReadTimeout time.Duration
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

func (b *RingBuf) Clear() {
	atomic.StoreUint64(&b.headAndSize, 0)
}

func (b *RingBuf) ReadWithTimeout(data []byte, timeout time.Duration) uint32 {
	toRead := uint32(len(data))
	ra := b.ReadAvail()
	if ra >= toRead {
		// easy case: have enough data to read
		nr := b.read(data, true)
		if nr != toRead {
			badRead()
		}
		return toRead
	}

	// hard case: no enough data. read till timeout
	readed := uint32(0)
	periods := int64(timeout / pollPeriod)
	for i := int64(0); i <= periods; i++ { // <= periods to do at least one read
		ra = minU32(b.ReadAvail(), toRead-readed)
		if ra > 0 {
			nr := b.read(data[readed:readed+ra], true)
			if nr != ra {
				badRead()
			}
			readed += nr
			if readed == toRead {
				return readed
			}
		}
		if i != periods {
			time.Sleep(pollPeriod)
		}
	}
	return readed
}

func (b *RingBuf) Read(data []byte) (int, error) {
	var readed uint32
	if b.ReadTimeout == 0 {
		readed = b.ReadWithTimeout(data, infinite)
	} else {
		readed = b.ReadWithTimeout(data, b.ReadTimeout)
	}
	if readed < uint32(len(data)) {
		return int(readed), io.EOF
	} else {
		return int(readed), nil
	}
}

func (b *RingBuf) ReadS(data []byte) uint32 {
	readed := uint32(0)
	toRead := uint32(len(data))
	hs, head, sz := b.loadHS()
	for readed < toRead {
		if sz > 0 {
			nr := minU32(sz, toRead-readed)
			if head+toRead > b.max {
				// wrapped
				ll := b.max - head
				copy(data[readed:readed+ll], b.mem[head:head+ll])
				copy(data[readed+ll:readed+nr], b.mem[0:nr-ll])
			} else {
				copy(data[readed:readed+nr], b.mem[head:head+nr])
			}
			for {
				sz -= nr
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
			readed += nr
		} else {
			runtime.Gosched()
			hs, head, sz = b.loadHS()
		}
	}
	return readed
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
		}
	}
	return false
}

func (b *RingBuf) SkipWithTimeout(toSkip uint32, timeout time.Duration) uint32 {
	if toSkip == 0 {
		return 0
	}
	hs, head, sz := b.loadHS()
	skipped := uint32(0)
	periods := int64(timeout / pollPeriod)
	for i := int64(0); i <= periods; i++ {
		if sz > 0 {
			n := minU32(toSkip-skipped, sz)
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
			skipped += n
			if skipped == toSkip {
				return toSkip
			}
		} else {
			if i != periods {
				time.Sleep(pollPeriod)
			}
		}
		hs, head, sz = b.loadHS()
	}
	return skipped
}

// Skip with default timeout (blocking if b.ReadTimeout is not set)
func (b *RingBuf) Skip(toSkip uint32) uint32 {
	if b.ReadTimeout == 0 {
		return b.SkipWithTimeout(toSkip, infinite)
	} else {
		return b.SkipWithTimeout(toSkip, b.ReadTimeout)
	}
}

func (b *RingBuf) Avail() (readAvail uint32, writeAvail uint32) {
	hs := atomic.LoadUint64(&b.headAndSize)
	readAvail = uint32(hs)
	writeAvail = b.max - readAvail
	return
}

func (b *RingBuf) ReadAvail() uint32 {
	return uint32(atomic.LoadUint64(&b.headAndSize))
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

func (b *RingBuf) Write(data []byte) (int, error) {
	written := uint32(0)
	toWrite := uint32(len(data))
	for written < toWrite {
		hs, head, sz := b.loadHS()
		writePos := (head + sz) % b.max
		nw := minU32(b.max-sz, toWrite-written)
		if nw == 0 {
			runtime.Gosched()
			continue
		}
		if writePos+nw > b.max {
			// wrapped
			ll := b.max - writePos
			if ll > 0 {
				copy(b.mem[writePos:b.max], data[written:written+ll])
				copy(b.mem[0:nw-ll], data[written+ll:written+nw])
			} else {
				copy(b.mem[0:nw], data[written:written+nw])
			}
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
			hs, head, sz = b.loadHS()
		}
		written += nw
	}
	return int(written), nil
}
