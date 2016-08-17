// Inter-goroutine connection. Similar to net.Pipe
package pipe

import (
	"net"
	"sync"
	"time"

	"github.com/pi/goal/md"
)

type pipeAddr int

func (a pipeAddr) Network() string { return "pipe" }
func (a pipeAddr) String() string  { return "pipe" }

type pipeConn struct {
	rp, wp        *Pipe
	rmux, wmux    sync.Mutex
	readDeadline  time.Duration // md.Monotime
	writeDeadline time.Duration // md.Monotime
}

func (c *pipeConn) LocalAddr() net.Addr {
	return pipeAddr(0)
}

func (c *pipeConn) RemoteAddr() net.Addr {
	return pipeAddr(0)
}

func (c *pipeConn) SetReadDeadline(deadline time.Time) error {
	if deadline.IsZero() {
		c.readDeadline = 0
	} else {
		c.readDeadline = md.Monotime() + deadline.Sub(time.Now())
	}
	return nil
}

func (c *pipeConn) SetWriteDeadline(deadline time.Time) error {
	if deadline.IsZero() {
		c.writeDeadline = 0
	} else {
		c.writeDeadline = md.Monotime() + deadline.Sub(time.Now())
	}
	return nil
}

func (c *pipeConn) SetDeadline(deadline time.Time) error {
	c.SetReadDeadline(deadline)
	c.writeDeadline = c.readDeadline
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
	var timeout time.Duration
	if c.readDeadline == 0 {
		timeout = 0
	} else {
		timeout = md.Monotime() - c.readDeadline
	}
	c.rmux.Lock()
	defer c.rmux.Unlock()
	return c.rp.ReadWithTimeout(buf, timeout)
}

func (c *pipeConn) Write(buf []byte) (int, error) {
	var timeout time.Duration
	if c.writeDeadline == 0 {
		timeout = 0
	} else {
		timeout = md.Monotime() - c.writeDeadline
	}
	c.wmux.Lock()
	defer c.wmux.Unlock()
	return c.wp.WriteWithTimeout(buf, timeout)
}

func Conn(bufSize int) (net.Conn, net.Conn) {
	p1 := New(bufSize)
	p2 := New(bufSize)
	return &pipeConn{rp: p1, wp: p2}, &pipeConn{rp: p2, wp: p1}
}
