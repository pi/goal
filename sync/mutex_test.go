package sync

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMutex(t *testing.T) {
	m := Mutex()
	require.NoError(t, m.Lock(nil))
	m.Unlock()

	ctx, cf := context.WithTimeout(context.Background(), time.Millisecond)
	require.NoError(t, m.Lock(nil))
	err := m.Lock(ctx)
	require.Error(t, err)
	require.Equal(t, context.DeadlineExceeded, err)
	m.Unlock()
	require.NoError(t, m.Lock(nil))

	cf()
}

func BenchmarkMutexSpeed(b *testing.B) {
	m := sync.Mutex{}
	wg := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			for i := 0; i < 40000; i++ {
				m.Lock()
				m.Unlock()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkGoalMutex(b *testing.B) {
	ctx, _ := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	m := Mutex()
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			for i := 0; i < 40000; i++ {
				m.Lock(ctx)
				m.Unlock()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}
