// Inter-goroutine connection. Similar to net.Pipe
package pipe

import (
	"net"
	"sync"
	"time"
)

type pipeAddr int

func (a pipeAddr) Network() string { return "pipe" }
func (a pipeAddr) String() string  { return "pipe" }

type pipeConn struct {
	rp, wp *Pipe
	rlock  sync.Mutex
}

func (c *pipeConn) LocalAddr() net.Addr {
	return pipeAddr(0)
}

func (c *pipeConn) RemoteAddr() net.Addr {
	return pipeAddr(0)
}

func (c *pipeConn) SetReadDeadline(deadline time.Time) error {
	c.rp.SetReadDeadline(deadline)
	return nil
}

func (c *pipeConn) SetWriteDeadline(deadline time.Time) error {
	c.wp.SetWriteDeadline(deadline)
	return nil
}

func (c *pipeConn) SetDeadline(deadline time.Time) error {
	c.rp.SetReadDeadline(deadline)
	c.wp.SetWriteDeadline(deadline)
	return nil
}

func (c *pipeConn) Close() error {
	err1 := c.rp.Close()
	err := c.wp.Close()
	if err == nil {
		err = err1
	}
	return err
}

func (c *pipeConn) Read(buf []byte) (int, error) {
	c.rlock.Lock()
	defer c.rlock.Unlock()
	return c.rp.Read(buf)
}

func (c *pipeConn) Write(buf []byte) (int, error) {
	return c.wp.Write(buf)
}

func Conn(bufSize int) (net.Conn, net.Conn) {
	p1 := New(bufSize)
	p2 := New(bufSize)
	return &pipeConn{
			rp: p1,
			wp: p2,
		},
		&pipeConn{
			rp: p2,
			wp: p1,
		}
}
