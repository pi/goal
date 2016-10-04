package pipe

import (
	"context"
	"io"
	"sync/atomic"
)

type Writer struct {
	ringbuf
}

func (w *Writer) writeUnlocked(data []byte) (int, error) {
	toWrite := len(data)
	if toWrite == 0 {
		if w.IsClosed() {
			return 0, io.EOF
		} else {
			return 0, w.checkDeadline()
		}
	}
	timeoutChan, exceed := w.timeoutChan()
	if exceed {
		return 0, timeoutError
	}
	written := 0
	for written < toWrite {
		_, closed, head, sz := w.loadHeader()
		if closed {
			notify(w.rsig) // resume other writers (if any)
			return written, io.EOF
		}
		nw := minInt(w.Cap()-sz, toWrite-written)
		if nw > 0 {
			writePos := (head + sz) & w.mask
			if writePos > w.Cap()-nw {
				// wrapped
				ll := w.Cap() - writePos
				copy(w.mem[writePos:], data[written:written+ll])
				copy(w.mem[:nw-ll], data[written+ll:written+nw])
			} else {
				copy(w.mem[writePos:writePos+nw], data[written:written+nw])
			}
			atomic.AddUint64(w.pbits, uint64(nw))
			written += nw
			notify(w.wsig)
		} else {
			if closed {
				notify(w.rsig) // resume other writers (if any)
				return written, io.EOF
			}
			select {
			case <-w.rsig:
			case <-timeoutChan:
				return written, timeoutError
			}
		}
	}
	return toWrite, nil
}

func (w *Writer) writeUnlockedWithContext(ctx context.Context, data []byte) (int, error) {
	toWrite := len(data)
	if toWrite == 0 {
		if w.IsClosed() {
			return 0, io.EOF
		} else {
			return 0, nil
		}
	}
	timeoutChan, exceed := w.timeoutChan()
	if exceed {
		return 0, timeoutError
	}
	written := 0
	for written < toWrite {
		_, closed, head, sz := w.loadHeader()
		if closed {
			return written, io.EOF
		}
		nw := minInt(w.Cap()-sz, toWrite-written)
		if nw > 0 {
			writePos := (head + sz) & w.mask
			if writePos > w.Cap()-nw {
				// wrapped
				ll := w.Cap() - writePos
				copy(w.mem[writePos:], data[written:written+ll])
				copy(w.mem[:nw-ll], data[written+ll:written+nw])
			} else {
				copy(w.mem[writePos:writePos+nw], data[written:written+nw])
			}
			atomic.AddUint64(w.pbits, uint64(nw))
			written += nw
			notify(w.wsig)
		} else {
			if closed {
				notify(w.rsig) // resume other writers (if any)
				return written, io.EOF
			}
			select {
			case <-w.rsig:
			case <-timeoutChan:
				return written, timeoutError
			case <-ctx.Done():
				return written, ctx.Err()
			}
		}
	}
	return toWrite, nil
}

func (w *Writer) Write(data []byte) (int, error) {
	if w.IsClosed() {
		return 0, io.EOF
	}

	toWrite := len(data)
	if toWrite == 0 {
		return 0, nil
	}

	if w.synchronized {
		err := w.lock()
		if err != nil {
			return 0, err
		}
	}

	timeoutChan, exceed := w.timeoutChan()
	if exceed {
		if w.synchronized {
			w.unlock()
		}
		return 0, timeoutError
	}

	written := 0
	for written < toWrite {
		_, closed, head, sz := w.loadHeader()
		if closed {
			if w.synchronized {
				w.unlock()
			}
			return written, io.EOF
		}
		nw := minInt(w.Cap()-sz, toWrite-written)
		if nw > 0 {
			writePos := (head + sz) & w.mask
			if writePos > w.Cap()-nw {
				// wrapped
				ll := w.Cap() - writePos
				copy(w.mem[writePos:], data[written:written+ll])
				copy(w.mem[:nw-ll], data[written+ll:written+nw])
			} else {
				copy(w.mem[writePos:writePos+nw], data[written:written+nw])
			}
			atomic.AddUint64(w.pbits, uint64(nw))
			written += nw
			notify(w.wsig)
		} else {
			if closed {
				if w.synchronized {
					w.unlock()
				}
				notify(w.rsig) // resume other writers (if any)
				return written, io.EOF
			}
			select {
			case <-w.rsig:
			case <-timeoutChan:
				if w.synchronized {
					w.unlock()
				}
				return written, timeoutError
			}
		}
	}
	if w.synchronized {
		w.unlock()
	}
	return toWrite, nil
}

func (w *Writer) WriteWithContext(ctx context.Context, data []byte) (int, error) {
	if w.IsClosed() {
		return 0, io.EOF
	}
	toWrite := len(data)
	if toWrite == 0 {
		return 0, nil
	}

	if w.synchronized {
		err := w.lockWithContext(ctx)
		if err != nil {
			return 0, err
		}
	}

	timeoutChan, exceed := w.timeoutChan()
	if exceed {
		if w.synchronized {
			w.unlock()
		}
		return 0, timeoutError
	}

	written := 0
	for written < toWrite {
		_, closed, head, sz := w.loadHeader()
		if closed {
			if w.synchronized {
				w.unlock()
			}
			return written, io.EOF
		}
		nw := minInt(w.Cap()-sz, toWrite-written)
		if nw > 0 {
			writePos := (head + sz) & w.mask
			if writePos > w.Cap()-nw {
				// wrapped
				ll := w.Cap() - writePos
				copy(w.mem[writePos:], data[written:written+ll])
				copy(w.mem[:nw-ll], data[written+ll:written+nw])
			} else {
				copy(w.mem[writePos:writePos+nw], data[written:written+nw])
			}
			atomic.AddUint64(w.pbits, uint64(nw))
			written += nw
			notify(w.wsig)
		} else {
			if closed {
				if w.synchronized {
					w.unlock()
				}
				notify(w.rsig) // resume other writers (if any)
				return written, io.EOF
			}
			select {
			case <-w.rsig:
			case <-ctx.Done():
				if w.synchronized {
					w.unlock()
				}
				return written, ctx.Err()
			case <-timeoutChan:
				if w.synchronized {
					w.unlock()
				}
				return written, timeoutError
			}
		}
	}
	if w.synchronized {
		w.unlock()
	}
	return toWrite, nil
}

func (w *Writer) WriteAll(chunks ...[]byte) (int64, error) {
	//TODO optimize
	if w.IsClosed() {
		return 0, io.EOF
	}
	if w.synchronized {
		err := w.lock()
		if err != nil {
			return 0, err
		}
	}
	var written int64
	for _, data := range chunks {
		n, err := w.writeUnlocked(data)
		written += int64(n)
		if err != nil {
			if w.synchronized {
				w.unlock()
			}
			return written, err
		}
	}
	if w.synchronized {
		w.unlock()
	}
	return written, nil
}

func (w *Writer) WriteAllWithContext(ctx context.Context, chunks ...[]byte) (int64, error) {
	if w.IsClosed() {
		return 0, io.EOF
	}
	if w.synchronized {
		err := w.lockWithContext(ctx)
		if err != nil {
			return 0, err
		}
	}
	var written int64
	for _, data := range chunks {
		n, err := w.writeUnlockedWithContext(ctx, data)
		written += int64(n)
		if err != nil {
			if w.synchronized {
				w.unlock()
			}
			return written, err
		}
	}
	if w.synchronized {
		w.unlock()
	}
	return written, nil
}

func (w *Writer) WriteByte(b byte) error {
	var data [1]byte
	data[0] = b
	_, err := w.Write(data[:])
	return err
}

func (w *Writer) WriteWait(min int) error {
	if min > w.Cap() {
		return ErrOvercap
	}
	if min < 1 {
		min = 1
	}
	timeoutChan, exceed := w.timeoutChan()
	if exceed {
		return timeoutError
	}
	for {
		_, closed, _, sz := w.loadHeader()
		if closed {
			notify(w.rsig) // resume other writers (if any)
			return io.EOF
		}
		if w.Cap()-sz >= min {
			return nil
		}
		select {
		case <-w.rsig:
		case <-timeoutChan:
			return timeoutError
		}
	}
}

func (w *Writer) WriteWaitWithContext(ctx context.Context, min int) error {
	if min > w.Cap() {
		return ErrOvercap
	}
	if min < 1 {
		min = 1
	}
	timeoutChan, exceed := w.timeoutChan()
	if exceed {
		return timeoutError
	}
	for {
		_, closed, _, sz := w.loadHeader()
		if closed {
			notify(w.rsig) // resume other writers (if any)
			return io.EOF
		}
		if w.Cap()-sz >= min {
			return nil
		}
		select {
		case <-w.rsig:
		case <-ctx.Done():
			return ctx.Err()
		case <-timeoutChan:
			return timeoutError
		}
	}
}

func (w *Writer) ReadFrom(r io.Reader) (written int64, err error) {
	if w.synchronized {
		err = w.lock()
		if err != nil {
			return 0, err
		}
	}
	timeoutChan, exceed := w.timeoutChan()
	if exceed {
		if w.synchronized {
			w.unlock()
		}
		return 0, timeoutError
	}
	for {
		_, closed, head, sz := w.loadHeader()
		if closed {
			if w.synchronized {
				w.unlock()
			}
			return written, io.EOF
		}
		if (w.Cap() - sz) > 0 {
			writePos := (head + sz) & w.mask
			var nw int
			if writePos < head {
				// wrapped
				nw, err = r.Read(w.mem[writePos:head])
			} else {
				var n1, n2 int
				n1, err = r.Read(w.mem[writePos:])
				if err == nil {
					n2, err = r.Read(w.mem[:head])
				}
				nw = n1 + n2
			}
			if nw > 0 {
				atomic.AddUint64(w.pbits, uint64(nw))
				written += int64(nw)
				notify(w.wsig)
			}
			if err != nil {
				if err == io.EOF {
					err = nil
				}
				if w.synchronized {
					w.unlock()
				}
				return written, err
			}
		} else {
			if closed {
				if w.synchronized {
					w.unlock()
				}
				notify(w.rsig) // resume other writers (if any)
				return written, io.EOF
			}
			select {
			case <-w.rsig:
			case <-timeoutChan:
				if w.synchronized {
					w.unlock()
				}
				return written, timeoutError
			}
		}
	}
}
