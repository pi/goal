package pipe

type Reader interface {
	io.Closer
	io.Reader
	io.WriterTo
	io.ByteReader
	ReadWithContext(ctx context.Context, data []byte) (int, error)
	Skip(toSkip int) (int, error)
	SkipWithContext(ctx context.Context, data []byte) (int, error)
	Peek(buf []byte) (int, error)
	SkipPending() (int, error)
	Pending() int
	ReadWait(n int) (int, error)
	ReadWaitWithContext(ctx context.Context, n int) (int, error)
}

type Writer interface {
	io.Closer
	io.Writer
	io.ByteWriter
	io.ReaderFrom
	WriteAll(data ...[]byte) (int64, error)
	WriteAllWithContext(ctx context.Context, data ...[]byte) (int64, error)
	WriteWait(n int) (int, error)
	WriteWaitWithContext(ctx context.Context, n int) (int, error)
}

type reader struct {
	ringbuf
}

type syncReader struct {
	Reader
}

type writer struct {
	ringbuf
}

type syncWriter struct {
	writer
}

func newPipe(mem []byte) (r Reader, w Writer) {
	max := len(mem)
	var bits uint64
	rsig := make(chan struct{}, 1)
	wsig := make(chan struct{}, 1)
	r = &reader{
		ringbuf{
			pbits: &bits,
			mem:   mem,
			mask:  max - 1,
			wsig:  wsig,
			rsig:  rsig,
			lsig:  make(chan struct{}, 1),
		},
	}
	w = &writer{
		ringbuf{
			pbits: &bits,
			mem:   mem,
			mask:  max - 1,
			wsig:  wsig,
			rsig:  rsig,
			lsig:  make(chan struct{}, 1),
		},
	}
	return
}

func newSyncWritePipe(mem []byte) (r Reader, w Writer) {
	max := len(mem)
	var bits uint64
	rsig := make(chan struct{}, 1)
	wsig := make(chan struct{}, 1)
	r = &reader{
		ringbuf{
			pbits: &bits,
			mem:   mem,
			mask:  max - 1,
			wsig:  wsig,
			rsig:  rsig,
			lsig:  make(chan struct{}, 1),
		},
	}
	w = &synchronizedWriter{
		writer{
			ringbuf{
				pbits: &bits,
				mem:   mem,
				mask:  max - 1,
				wsig:  wsig,
				rsig:  rsig,
				lsig:  make(chan struct{}, 1),
			},
		},
	}
	return
}

func newSyncPipe(mem []byte) (r Reader, w Writer) {
	max := len(mem)
	var bits uint64
	rsig := make(chan struct{}, 1)
	wsig := make(chan struct{}, 1)
	r = &syncReader{
		reader{
			ringbuf{
				pbits: &bits,
				mem:   mem,
				mask:  max - 1,
				wsig:  wsig,
				rsig:  rsig,
				lsig:  make(chan struct{}, 1),
			},
		},
	}
	w = &syncWriter{
		writer{
			ringbuf{
				pbits: &bits,
				mem:   mem,
				mask:  max - 1,
				wsig:  wsig,
				rsig:  rsig,
				lsig:  make(chan struct{}, 1),
			},
		},
	}
	return
}

func allocBuf(max int) (mem []byte) {
	if max == 0 {
		max = defaultBufferSize
	} else if max < minBufferSize {
		max = minBufferSize
	} else if (max & (max - 1)) != 0 {
		// round up to power of two
		max = 1 << gut.BitLen(uint(max))
	}
	if int(low31bits) < max-1 {
		panic("ringbuffer size is too large")
	}
	return make([]byte, max)
}

func Pipe(max int) (r Reader, w Writer) {
	return newPipe(allocBuf(max))
}

func SyncWritePipe(max int) (r Reader, w Writer) {
	return newSyncWritePipe(allocBuf(max))
}

func SyncPipe(max int) (r Reader, w Writer) {
	return newSyncPipe(allocBuf(max))
}

// WriteAvail returns number of bytes that can be written immediately
func (b *writer) WriteAvail() int {
	return b.Cap() - b.readAvail()
}
