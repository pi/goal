package gut

import (
	"sync"
	"testing"
)

func TestZeroLocker(t *testing.T) {
	cv := sync.NewCond(ZeroLocker)
	go func() { cv.Signal() }()

	cv.Wait()
}
