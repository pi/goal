package future

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFuture(t *testing.T) {
	var results [3]int

	f := New(func(*Future) error { results[0] = 1; return nil }).
		Then(func(*Future) error { results[1] = 2; return nil }).
		Then(func(*Future) error { results[2] = 3; return nil }).
		Then(func(ft *Future) error { ft.SetResult(33); return nil }).
		Go()

	r, err := f.WaitResult()
	require.Equal(t, results[0], 1)
	require.Equal(t, results[1], 2)
	require.Equal(t, results[2], 3)
	require.NoError(t, err)
	require.Equal(t, r.(int), 33)
}

func TestFutureReady(t *testing.T) {
	f := New(func(*Future) error { time.Sleep(time.Second); return nil }).Go()
	require.False(t, f.IsReady())
	err := f.Wait()
	require.NoError(t, err)
	require.True(t, f.IsReady())
}

func TestFutureTimedWait(t *testing.T) {
	f := New(func(*Future) error { time.Sleep(time.Second); return nil }).Go()
	r, e := f.TimedWait(100 * time.Millisecond)
	require.NoError(t, e)
	require.False(t, r)
	r, e = f.TimedWait(time.Second * 2)
	require.NoError(t, e)
	require.True(t, r)
}

func TestFutureOnComplete(t *testing.T) {
	var results [3]int

	f := New(func(ft *Future) error { results[0] = 1; ft.SetResult(1); return nil }).
		OnComplete(func(ft *Future) { require.Equal(t, ft.Result().(int), 1) }).
		Then(func(ft *Future) error { results[1] = 2; ft.SetResult(2); return nil }).
		OnComplete(func(ft *Future) { require.Equal(t, ft.Result().(int), 2) }).
		Then(func(ft *Future) error { results[2] = 3; ft.SetResult(3); return nil }).
		OnComplete(func(ft *Future) { require.Equal(t, ft.Result().(int), 3) }).
		Then(func(ft *Future) error { ft.SetResult(33); return nil }).
		OnComplete(func(ft *Future) { require.Equal(t, ft.Result().(int), 33) }).
		Go()

	r, err := f.WaitResult()
	require.Equal(t, results[0], 1)
	require.Equal(t, results[1], 2)
	require.Equal(t, results[2], 3)
	require.NoError(t, err)
	require.Equal(t, r.(int), 33)
}

func TestFutureOnError(t *testing.T) {
	var results [3]int

	f := New(func(ft *Future) error { results[0] = 1; ft.SetResult(1); return nil }).
		Then(func(ft *Future) error { results[1] = 2; ft.SetResult(2); return errors.New("err") }).
		OnError(func(ft *Future) { require.Error(t, ft.Err()) }).
		OnComplete(func(ft *Future) { require.Equal(t, ft.Result().(int), 2) }).
		Then(func(ft *Future) error { results[2] = 3; ft.SetResult(3); return nil }).
		OnComplete(func(ft *Future) { require.Equal(t, ft.Result().(int), 3) }).
		Then(func(ft *Future) error { ft.SetResult(33); return nil }).
		OnComplete(func(ft *Future) { require.Equal(t, ft.Result().(int), 33) }).
		Go()

	r, err := f.WaitResult()
	require.Error(t, err)
	require.Equal(t, r.(int), 2)
	require.Equal(t, results[0], 1)
	require.Equal(t, results[1], 2)
	require.Equal(t, results[2], 0)
}
