package set

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringSet(t *testing.T) {
	assert.Fail(t, "TODO")
}

func TestIntSet(t *testing.T) {
	s := make(IntSet)
	for i := 0; i < 10; i++ {
		s.Add(i)
	}
	for i := 0; i < 10; i++ {
		assert.True(t, s.Includes(i))
	}
	for i := 10; i < 20; i++ {
		assert.False(t, s.Includes(i))
	}
	assert.Fail(t, "TODO")
}
