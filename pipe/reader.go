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
		if r.IsClosed() {
			notify(r.wsig) // resume ohter readers (if any)
			return 0, io.EOF
		} else {
			return 0, nil
		}
	}
	if r.synchronized != 0 {
		err := r.lock()
		if err != nil {
			return 0, err
		}
	}
	readed := 0
	for readed < toRead {
		hs, closed, head, sz := r.loadHeader()
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
			if closed {
				if r.synchronized != 0 {
					r.unlock()
				}
				return readed, io.EOF
			}
			notify(r.rsig)
		} else {
			if closed {
				if r.synchronized != 0 {
					r.unlock()
				}
				notify(r.wsig) // resume other readers (if any)
				return readed, io.EOF
			}
			<-r.wsig
		}
	}
	if r.synchronized != 0 {
		r.unlock()
	}
	return readed, nil
}

func (r *Reader) ReadWithContext(ctx context.Context, data []byte) (int, error) {
	toRead := len(data)
	if toRead == 0 {
		if r.IsClosed() {
			notify(r.wsig) // resume ohter waiters (if any)
			return 0, io.EOF
		} else {
			return 0, nil
		}
	}
	if r.synchronized != 0 {
		err := r.lockWithContext(ctx)
		if err != nil {
			return 0, err
		}
	}
	readed := 0
	for readed < toRead {
		hs, closed, head, sz := r.loadHeader()
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
			if closed {
				if r.synchronized != 0 {
					r.unlock()
				}
				notify(r.wsig) // resume other readers (if any)
				return readed, io.EOF
			}
			select {
			case <-r.wsig:
			case <-ctx.Done():
				if r.synchronized != 0 {
					r.unlock()
				}
				return readed, ctx.Err()
			}
		}
	}
	if r.synchronized != 0 {
		r.unlock()
	}
	return readed, nil
}

func (r *Reader) Peek(data []byte) (int, error) {
	if r.synchronized != 0 {
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
	if r.synchronized != 0 {
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
	if r.synchronized != 0 {
		err := r.lock()
		if err != nil {
			return 0, err
		}
	}
	skipped := 0
	for skipped < toSkip {
		hs, closed, head, sz := r.loadHeader()
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
			if closed {
				if r.synchronized != 0 {
					r.unlock()
				}
				notify(r.wsig) // resume ohter waiters (if any)
				return skipped, io.EOF
			}
			<-r.wsig
		}
	}
	if r.synchronized != 0 {
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
	if r.synchronized != 0 {
		err := r.lockWithContext(ctx)
		if err != nil {
			return 0, err
		}
	}
	skipped := 0
	for skipped < toSkip {
		hs, closed, head, sz := r.loadHeader()
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
			if closed {
				if r.synchronized != 0 {
					r.unlock()
				}
				notify(r.wsig) // resume other readers (if any)
				return skipped, io.EOF
			}
			select {
			case <-r.wsig:
			case <-ctx.Done():
				return skipped, ctx.Err()
			}
		}
	}
	if r.synchronized != 0 {
		r.unlock()
	}
	return skipped, nil
}

// Pending returns number of bytes availbale to immediate read
func (r *Reader) Pending() int {
	return int(atomic.LoadUint64(r.pbits) & uint64(low31bits))
}

func (r *Reader) ReadWait(min int) error {
	if min > r.Cap() {
		return ErrOvercap
	}
	if min < 1 {
		min = 1
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
		<-r.wsig
	}
}

func (r *Reader) ReadWaitWithContext(ctx context.Context, min int) error {
	if min > r.Cap() {
		return ErrOvercap
	}
	if min < 1 {
		min = 1
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
		}
	}
}

func (r *Reader) ReadByte() (byte, error) {
	var b [1]byte
	_, err := r.Read(b[:])
	return b[0], err
}

func (r *Reader) WriteTo(w io.Writer) (int64, error) {
	var chunkArray [8192]byte
	chunk := chunkArray[:]
	written := int64(0)
	for {
		n, err := r.Read(chunk)
		if n > 0 {
			nw, werr := w.Write(chunkArray[:n])
			written += int64(nw)
			if werr != nil {
				return written, werr
			}
		}
		if err != nil {
			if err == io.EOF {
				return written, nil
			} else {
				return written, err
			}
		}
	}
}
