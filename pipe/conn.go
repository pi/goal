// Inter-goroutine connection. Similar to net.Pipe
package pipe

import (
	"net"
	"time"
)

type pipeAddr int

func (a pipeAddr) Network() string { return "pipe" }
func (a pipeAddr) String() string  { return "pipe" }

type pipeConn struct {
	r Reader
	w Writer
}

func newConn(r1 Reader, w1 Writer, r2 Reader, w2 Writer) (net.Conn, net.Conn) {
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
	r1, w1 := Pipe(bufSize)
	r2, w2 := Pipe(bufSize)
	return newConn(r1, w1, r2, w2)
}

func SyncConn(bufSize int) (net.Conn, net.Conn) {
	r1, w1 := SyncPipe(bufSize)
	r2, w2 := SyncPipe(bufSize)
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

func (c *pipeConn) SetReadDeadline(deadline time.Time) error {
	//TODO c.rp.SetReadDeadline(deadline)
	return nil
}

func (c *pipeConn) SetWriteDeadline(deadline time.Time) error {
	//TODO c.wp.SetWriteDeadline(deadline)
	return nil
}

func (c *pipeConn) SetDeadline(deadline time.Time) error {
	//TODO c.rp.SetReadDeadline(deadline)
	//TODO c.wp.SetWriteDeadline(deadline)
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
	return c.r.Read(buf)
}

func (c *pipeConn) Write(buf []byte) (int, error) {
	return c.w.Write(buf)
}
