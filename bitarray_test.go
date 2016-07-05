package goal

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func Test_BitArray(t *testing.T) {
	const N = 10000000
	a := NewBitArray(N)

	for i := 0; i < N; i++ {
		if a.Get(i) {
			assert.FailNow(t, "1")
		}
	}

	for i := 0; i < N; i++ {
		a.Put(i, true)
		if !a.Get(i) {
			assert.FailNow(t, "2.0")
		}
	}

	for i := 0; i < N; i++ {
		if !a.Get(i) {
			assert.FailNow(t, "2")
		}
	}

	a.Clear()

	for i := 0; i < N; i += 2 {
		a.Put(i, false)
		a.Put(i+1, true)
	}

	for i := 0; i < N; i += 2 {
		if a.Get(i) {
			assert.Fail(t, "3")
		}
		if !a.Get(i + 1) {
			assert.Fail(t, "4")
		}
	}
}
