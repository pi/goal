package sync

import (
	"context"
	"sync/atomic"

	"github.com/pi/goal/debug"
)

type Mux struct {
	plck *int32
	plq  *int32
	lsig chan struct{}
}

func Mutex() *Mux {
	return (&Mux{}).Init()
}

func (m *Mux) Init() *Mux {
	m.plck = new(int32)
	m.plq = new(int32)
	m.lsig = make(chan struct{})
	return m
}

func (m *Mux) Unlock() {
	if debug.Enabled {
		if atomic.LoadInt32(m.plck) == 0 {
			panic("unlocking not locked mux")
		}
	}
	atomic.StoreInt32(m.plck, 0)
	if atomic.LoadInt32(m.plq) > 0 {
		select {
		case m.lsig <- struct{}{}:
		default:
		}
	}
}

func (m *Mux) TryLock() bool {
	lck := atomic.LoadInt32(m.plck)
	if (lck == 0) && atomic.CompareAndSwapInt32(m.plck, 0, 1) {
		return true
	} else {
		return false
	}
}

func (m *Mux) Lock(ctx context.Context) error {
	// fast path
	lck := atomic.LoadInt32(m.plck)
	if (lck == 0) && atomic.CompareAndSwapInt32(m.plck, 0, 1) {
		return nil
	}
	// slow path
	atomic.AddInt32(m.plq, 1)
	for {
		// first spin some
		for i := 0; i < 100; i++ {
			lck = atomic.LoadInt32(m.plck)
			if (lck == 0) && atomic.CompareAndSwapInt32(m.plck, 0, 1) {
				atomic.AddInt32(m.plq, -1)
				return nil
			}
		}
		// then wait notification
		select {
		case <-m.lsig:
		case <-ctx.Done():
			atomic.AddInt32(m.plq, -1)
			return ctx.Err()
		}
	}
}
