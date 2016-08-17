package atomic

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInt(t *testing.T) {
	var a Int

	require.Equal(t, 0, a.Get())
	a.Inc(1)
	require.Equal(t, 1, a.Get())
	a.Dec(1)
	require.Equal(t, 0, a.Get())
	a.Set(3)
	require.Equal(t, 3, a.Get())
	require.Equal(t, 3, a.Swap(2))
	require.Equal(t, 2, a.Get())
	require.True(t, a.CompareAndSwap(2, 5))
	require.Equal(t, 5, a.Get())
}

func TestUint(t *testing.T) {
	var a Uint

	require.EqualValues(t, 0, a.Get())
	a.Inc(1)
	require.EqualValues(t, 1, a.Get())
	a.Dec(1)
	require.EqualValues(t, 0, a.Get())
	a.Set(3)
	require.EqualValues(t, 3, a.Get())
	require.EqualValues(t, 3, a.Swap(2))
	require.EqualValues(t, 2, a.Get())
	require.True(t, a.CompareAndSwap(2, 5))
	require.EqualValues(t, 5, a.Get())
}
