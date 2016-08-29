package pipe

func pipe(max int, rsync, wsync bool) (*Reader, *Writer) {
	r := &Reader{}
	w := &Writer{}
	r.init(max, rsync)
	w.initFrom(&r.ringbuf, wsync)
	return r, w
}

func Pipe(max int) (*Reader, *Writer) {
	return pipe(max, false, false)
}

func SyncWritePipe(max int) (*Reader, *Writer) {
	return pipe(max, false, true)
}

func SyncPipe(max int) (*Reader, *Writer) {
	return pipe(max, true, true)
}
