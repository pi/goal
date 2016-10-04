package gut

import (
	"runtime"
	"sync/atomic"
)

type Spinlock int32

func (sl *Spinlock) Lock() {
	for {
		if atomic.CompareAndSwapInt32((*int32)(sl), 0, 1) {
			return
		}
		runtime.Gosched()
	}
}

func (sl *Spinlock) Unlock() {
	atomic.StoreInt32((*int32)(sl), 0)
}
