package pipe

import (
	"context"
	"io"
	"runtime"
	"sync/atomic"
)

type Reader struct {
	ringbuf
}

func (r *Reader) Read(data []byte) (int, error) {
	toRead := len(data)
	if toRead == 0 {
		return 0, r.checkDeadline()
	}
	if r.synchronized {
		err := r.lock()
		if err != nil {
			return 0, err
		}
	}
	timeoutChan, exceed := r.timeoutChan()
	if exceed {
		if r.synchronized {
			r.unlock()
		}
		return 0, timeoutError
	}
	readed := 0
	for readed < toRead {
		hs, closed, head, sz := r.loadHeader()
		if closed && sz == 0 {
			if r.synchronized {
				r.unlock()
			}
			notify(r.wsig) // resume other readers (if any)
			return readed, io.EOF
		}
		nr := minInt(sz, toRead-readed)
		if nr > 0 {
			if head > r.Cap()-nr {
				// wrapped
				ll := r.Cap() - head
				copy(data[readed:readed+ll], r.mem[head:])
				copy(data[readed+ll:readed+nr], r.mem[:nr-ll])
			} else {
				copy(data[readed:readed+nr], r.mem[head:head+nr])
			}
			for {
				head = (head + nr) & r.mask
				sz -= nr
				if atomic.CompareAndSwapUint64(r.pbits, hs, (hs&headerFlagMask)|(uint64(head)<<32)|uint64(sz)) {
					break
				}
				runtime.Gosched()
				hs, closed, head, sz = r.loadHeader()
			}
			readed += nr
			notify(r.rsig)
		} else {
			select {
			case <-r.wsig:
			case <-timeoutChan:
				if r.synchronized {
					r.unlock()
				}
				return readed, timeoutError
			}
		}
	}
	if r.synchronized {
		r.unlock()
	}
	return readed, nil
}

func (r *Reader) ReadWithContext(ctx context.Context, data []byte) (int, error) {
	toRead := len(data)
	if toRead == 0 {
		return 0, r.checkDeadline()
	}
	if r.synchronized {
		err := r.lockWithContext(ctx)
		if err != nil {
			return 0, err
		}
	}
	readed := 0
	timeoutChan, exc := r.timeoutChan()
	if exc {
		if r.synchronized {
			r.unlock()
		}
		return 0, timeoutError
	}
	for readed < toRead {
		hs, closed, head, sz := r.loadHeader()
		if closed && sz == 0 {
			if r.synchronized {
				r.unlock()
			}
			notify(r.wsig) // resume other readers (if any)
			return readed, io.EOF
		}
		nr := minInt(sz, toRead-readed)
		if nr > 0 {
			if head > r.Cap()-nr {
				// wrapped
				ll := r.Cap() - head
				copy(data[readed:readed+ll], r.mem[head:])
				copy(data[readed+ll:readed+nr], r.mem[:nr-ll])
			} else {
				copy(data[readed:readed+nr], r.mem[head:head+nr])
			}
			for {
				head = (head + nr) & r.mask
				sz -= nr
				nhs := (hs & headerFlagMask) | (uint64(head) << 32) | uint64(sz)
				if atomic.CompareAndSwapUint64(r.pbits, hs, nhs) {
					break
				}
				runtime.Gosched()
				hs, closed, head, sz = r.loadHeader()
			}
			readed += nr
			notify(r.rsig)
		} else {
			select {
			case <-r.wsig:
			case <-ctx.Done():
				if r.synchronized {
					r.unlock()
				}
				return readed, ctx.Err()
			case <-timeoutChan:
				if r.synchronized {
					r.unlock()
				}
				return readed, timeoutError
			}
		}
	}
	if r.synchronized {
		r.unlock()
	}
	return readed, nil
}

func (r *Reader) Peek(data []byte) (int, error) {
	if r.synchronized {
		err := r.lock()
		if err != nil {
			return 0, err
		}
	}
	_, closed, head, nr := r.loadHeader()
	if nr > len(data) {
		nr = len(data)
	}
	if nr > 0 {
		if head > r.Cap()-nr {
			// wrapped
			ll := r.Cap() - head
			copy(data[:ll], r.mem[head:])
			copy(data[ll:nr], r.mem[:nr-ll])
		} else {
			copy(data, r.mem[head:head+nr])
		}
	}
	if r.synchronized {
		r.unlock()
	}
	if closed {
		return nr, io.EOF
	}
	return nr, nil
}

func (r *Reader) Skip(toSkip int) (int, error) {
	if toSkip <= 0 {
		if r.IsClosed() {
			return 0, io.EOF
		}
		return 0, nil
	}
	if r.synchronized {
		err := r.lock()
		if err != nil {
			return 0, err
		}
	}
	skipped := 0
	timeoutChan, exceed := r.timeoutChan()
	if exceed {
		if r.synchronized {
			r.unlock()
		}
		return 0, timeoutError
	}
	for skipped < toSkip {
		hs, closed, head, sz := r.loadHeader()
		if closed && sz == 0 {
			if r.synchronized {
				r.unlock()
			}
			notify(r.wsig) // resume ohter waiters (if any)
			return skipped, io.EOF
		}
		n := minInt(sz, toSkip-skipped)
		if n > 0 {
			for {
				head = (head + n) & r.mask
				sz -= n
				if atomic.CompareAndSwapUint64(r.pbits, hs, (hs&headerFlagMask)|(uint64(head)<<32)|uint64(sz)) {
					break
				}
				runtime.Gosched()
				hs, closed, head, sz = r.loadHeader()
			}
			skipped += n
			notify(r.rsig)
		} else {
			select {
			case <-r.wsig:
			case <-timeoutChan:
				if r.synchronized {
					r.unlock()
				}
				return skipped, timeoutError
			}
		}
	}
	if r.synchronized {
		r.unlock()
	}
	return skipped, nil
}

func (r *Reader) SkipWithContext(ctx context.Context, toSkip int) (int, error) {
	if toSkip <= 0 {
		if r.IsClosed() {
			return 0, io.EOF
		}
		return 0, nil
	}
	if r.synchronized {
		err := r.lockWithContext(ctx)
		if err != nil {
			return 0, err
		}
	}
	skipped := 0
	timeoutChan, exceed := r.timeoutChan()
	if exceed {
		if r.synchronized {
			r.unlock()
		}
		return 0, timeoutError
	}
	for skipped < toSkip {
		hs, closed, head, sz := r.loadHeader()
		if closed && sz == 0 {
			if r.synchronized {
				r.unlock()
			}
			notify(r.wsig) // resume other readers (if any)
			return skipped, io.EOF
		}
		n := minInt(sz, toSkip-skipped)
		if n > 0 {
			for {
				head = (head + n) & r.mask
				sz -= n
				if atomic.CompareAndSwapUint64(r.pbits, hs, (hs&headerFlagMask)|(uint64(head)<<32)|uint64(sz)) {
					break
				}
				runtime.Gosched()
				hs, closed, head, sz = r.loadHeader()
			}
			skipped += n
			notify(r.rsig)
		} else {
			select {
			case <-r.wsig:
			case <-ctx.Done():
				if r.synchronized {
					r.unlock()
				}
				return skipped, ctx.Err()
			case <-timeoutChan:
				if r.synchronized {
					r.unlock()
				}
				return skipped, timeoutError
			}
		}
	}
	if r.synchronized {
		r.unlock()
	}
	return skipped, nil
}

// Len returns number of buffered bytes availbale to immediate read
func (r *Reader) Len() int {
	return int(atomic.LoadUint64(r.pbits) & uint64(low31bits))
}

func (r *Reader) ReadWait(min int) error {
	if min > r.Cap() {
		return ErrOvercap
	}
	if min < 1 {
		min = 1
	}
	timeoutChan, exceed := r.timeoutChan()
	if exceed {
		return timeoutError
	}
	for {
		_, closed, _, sz := r.loadHeader()
		if sz >= min {
			return nil
		}
		if closed {
			notify(r.wsig) // resume other readers (if any)
			return io.EOF
		}
		select {
		case <-r.wsig:
		case <-timeoutChan:
			return timeoutError
		}
	}
}

func (r *Reader) ReadWaitWithContext(ctx context.Context, min int) error {
	if min > r.Cap() {
		return ErrOvercap
	}
	if min < 1 {
		min = 1
	}
	timeoutChan, exceed := r.timeoutChan()
	if exceed {
		return timeoutError
	}
	for {
		_, closed, _, sz := r.loadHeader()
		if sz >= min {
			return nil
		}
		if closed {
			notify(r.wsig) // resume other readers (if any)
			return io.EOF
		}
		select {
		case <-r.wsig:
		case <-ctx.Done():
			return ctx.Err()
		case <-timeoutChan:
			return timeoutError
		}
	}
}

func (r *Reader) ReadByte() (byte, error) {
	var b [1]byte
	_, err := r.Read(b[:])
	return b[0], err
}

func (r *Reader) WriteTo(w io.Writer) (readed int64, err error) {
	if r.synchronized {
		if err = r.lock(); err != nil {
			return 0, err
		}
	}
	timeoutChan, exceed := r.timeoutChan()
	if exceed {
		if r.synchronized {
			r.unlock()
		}
		return 0, timeoutError
	}
	for {
		hs, closed, head, sz := r.loadHeader()
		if closed && sz == 0 {
			notify(r.wsig) // resume other readers
			if r.synchronized {
				r.unlock()
			}
			return readed, nil
		}
		if sz > 0 {
			var n int
			if head > r.Cap()-sz {
				// wrapped
				var n1, n2 int
				n1, err := w.Write(r.mem[head:])
				if err == nil {
					n2, err = w.Write(r.mem[:sz-(r.Cap()-head)])
				}
				n = n1 + n2
			} else {
				n, err = w.Write(r.mem[head : head+sz])
			}
			readed += int64(n)
			for {
				head = (head + n) & r.mask
				sz -= n
				if atomic.CompareAndSwapUint64(r.pbits, hs, (hs&headerFlagMask)|(uint64(head)<<32)|uint64(sz)) {
					break
				}
				runtime.Gosched()
				hs, closed, head, sz = r.loadHeader()
			}
			notify(r.rsig)
			if err != nil {
				if r.synchronized {
					r.unlock()
				}
				return readed, err
			}
		} else {
			select {
			case <-r.wsig:
			case <-timeoutChan:
				if r.synchronized {
					r.unlock()
				}
				return readed, timeoutError
			}
		}
	}
}
