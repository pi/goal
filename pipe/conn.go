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
	rp, wp *Pipe
	rmux   sync.Mutex
	rsig   chan struct{}
	wsig   chan struct{}
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
	close(c.rsig)
	err1 := c.rp.Close()
	err := c.wp.Close()
	if err == nil {
		err = err1
	}
	return err
}

func (c *pipeConn) Read(buf []byte) (int, error) {
	if c.rp.readDeadline == 0 {
		<-c.rsig
	} else {
		dl := c.rp.readDeadline - md.Monotime()
		select {
		case <-c.rsig:
		case <-time.After(dl):
			return 0, ErrTimeout
		}
	}
	n, err := c.rp.Read(buf)
	if c.rp.ReadAvail() > 0 {
		select {
		case c.rsig <- struct{}{}:
		default:
		}
	}
	return n, err
}

func (c *pipeConn) Write(buf []byte) (int, error) {
	n, err := c.wp.Write(buf)
	select {
	case c.wsig <- struct{}{}:
	default:
	}
	return n, err
}

func Conn(bufSize int) (net.Conn, net.Conn) {
	p1 := New(bufSize)
	p2 := New(bufSize)
	s1 := make(chan struct{}, 1)
	s2 := make(chan struct{}, 1)
	return &pipeConn{
			rp:   p1,
			wp:   p2,
			rsig: s1,
			wsig: s2,
		},
		&pipeConn{
			rp:   p2,
			wp:   p1,
			rsig: s2,
			wsig: s1,
		}
}
