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
			return 0, nil
		}
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
				return written, io.EOF
			}
			<-w.rsig
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
				return written, io.EOF
			}
			select {
			case <-w.rsig:
			case <-ctx.Done():
				return written, ctx.Err()
			}
		}
	}
	return toWrite, nil
}

func (w *Writer) Write(data []byte) (int, error) {
	toWrite := len(data)
	if toWrite == 0 {
		if w.IsClosed() {
			return 0, io.EOF
		} else {
			return 0, nil
		}
	}

	if w.synchronized != 0 {
		err := w.lock()
		if err != nil {
			return 0, err
		}
	}

	written := 0
	for written < toWrite {
		_, closed, head, sz := w.loadHeader()
		if closed {
			if w.synchronized != 0 {
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
				if w.synchronized != 0 {
					w.unlock()
				}
				return written, io.EOF
			}
			<-w.rsig
		}
	}
	if w.synchronized != 0 {
		w.unlock()
	}
	return toWrite, nil
}

func (w *Writer) WriteWithContext(ctx context.Context, data []byte) (int, error) {
	toWrite := len(data)
	if toWrite == 0 {
		if w.IsClosed() {
			return 0, io.EOF
		} else {
			return 0, nil
		}
	}

	if w.synchronized != 0 {
		err := w.lockWithContext(ctx)
		if err != nil {
			return 0, err
		}
	}

	written := 0
	for written < toWrite {
		_, closed, head, sz := w.loadHeader()
		if closed {
			if w.synchronized != 0 {
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
				if w.synchronized != 0 {
					w.unlock()
				}
				return written, io.EOF
			}
			select {
			case <-w.rsig:
				/*case <-ctx.Done():
				if w.synchronized != 0 {
					w.unlock()
				}
				return written, ctx.Err()*/
			}
		}
	}
	if w.synchronized != 0 {
		w.unlock()
	}
	return toWrite, nil
}

func (w *Writer) WriteAll(chunks ...[]byte) (int64, error) {
	if w.synchronized != 0 {
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
			if w.synchronized != 0 {
				w.unlock()
			}
			return written, err
		}
	}
	if w.synchronized != 0 {
		w.unlock()
	}
	return written, nil
}

func (w *Writer) WriteAllWithContext(ctx context.Context, chunks ...[]byte) (int64, error) {
	if w.synchronized != 0 {
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
			if w.synchronized != 0 {
				w.unlock()
			}
			return written, err
		}
	}
	if w.synchronized != 0 {
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
	for {
		_, closed, _, sz := w.loadHeader()
		if closed {
			return io.EOF
		}
		if w.Cap()-sz >= min {
			return nil
		}
		<-w.rsig
	}
}

func (w *Writer) WriteWaitWithContext(ctx context.Context, min int) error {
	if min > w.Cap() {
		return ErrOvercap
	}
	if min < 1 {
		min = 1
	}
	for {
		_, closed, _, sz := w.loadHeader()
		if closed {
			return io.EOF
		}
		if w.Cap()-sz >= min {
			return nil
		}
		select {
		case <-w.rsig:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (w *Writer) ReadFrom(r io.Reader) (int64, error) {
	if w.synchronized != 0 {
		err := w.lock()
		if err != nil {
			return 0, err
		}
	}
	var chunkArray [8192]byte
	chunk := chunkArray[:]
	readed := int64(0)
	for {
		n, err := r.Read(chunk)
		if n > 0 {
			n, werr := w.writeUnlocked(chunkArray[:n])
			readed += int64(n)
			if werr != nil {
				if w.synchronized != 0 {
					w.unlock()
				}
				return readed, werr
			}
		}
		if err != nil {
			if w.synchronized != 0 {
				w.unlock()
			}
			if err == io.EOF {
				return readed, nil
			} else {
				return readed, err
			}
		}
	}
}
