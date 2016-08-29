// Inter-goroutine connection. Similar to net.Pipe
package pipe

import (
	"context"
	"net"
	"time"

	"github.com/pi/goal/md"
)

type pipeAddr int

func (a pipeAddr) Network() string { return "pipe" }
func (a pipeAddr) String() string  { return "pipe" }

type pipeConn struct {
	r             *Reader
	w             *Writer
	readDeadline  time.Duration
	writeDeadline time.Duration
}

func newConn(r1 *Reader, w1 *Writer, r2 *Reader, w2 *Writer) (net.Conn, net.Conn) {
	return &pipeConn{
			r: r1,
			w: w2,
		},
		&pipeConn{
			r: r2,
			w: w1,
		}
}

func Conn(bufSize int) (net.Conn, net.Conn) {
	r1, w1 := SyncPipe(bufSize)
	r2, w2 := SyncPipe(bufSize)
	return newConn(r1, w1, r2, w2)
}

func UnsyncConn(bufSize int) (net.Conn, net.Conn) {
	r1, w1 := Pipe(bufSize)
	r2, w2 := Pipe(bufSize)
	return newConn(r1, w1, r2, w2)
}

func SyncWriteConn(bufSize int) (net.Conn, net.Conn) {
	r1, w1 := SyncWritePipe(bufSize)
	r2, w2 := SyncWritePipe(bufSize)
	return newConn(r1, w1, r2, w2)
}

func (c *pipeConn) LocalAddr() net.Addr {
	return pipeAddr(0)
}

func (c *pipeConn) RemoteAddr() net.Addr {
	return pipeAddr(0)
}

func calcDeadline(deadline time.Time) time.Duration {
	if deadline.IsZero() {
		return 0
	}
	return md.Monotime() + deadline.Sub(time.Now())
}

func (c *pipeConn) SetReadDeadline(deadline time.Time) error {
	c.readDeadline = calcDeadline(deadline)
	return nil
}

func (c *pipeConn) SetWriteDeadline(deadline time.Time) error {
	c.writeDeadline = calcDeadline(deadline)
	return nil
}

func (c *pipeConn) SetDeadline(deadline time.Time) error {
	c.SetReadDeadline(deadline)
	c.writeDeadline = c.readDeadline
	return nil
}

func (c *pipeConn) Close() error {
	err1 := c.r.Close()
	err := c.w.Close()
	if err == nil {
		err = err1
	}
	return err
}

func (c *pipeConn) Read(buf []byte) (int, error) {
	if c.readDeadline == 0 {
		return c.r.Read(buf)
	}
	ctx, cf := context.WithTimeout(context.Background(), c.readDeadline-md.Monotime())
	n, err := c.r.ReadWithContext(ctx, buf)
	cf()
	return n, err
}

func (c *pipeConn) Write(buf []byte) (int, error) {
	if c.writeDeadline == 0 {
		return c.w.Write(buf)
	}
	ctx, cf := context.WithTimeout(context.Background(), c.writeDeadline-md.Monotime())
	n, err := c.w.WriteWithContext(ctx, buf)
	cf()
	return n, err
}
