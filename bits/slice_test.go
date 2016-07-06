package bits

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_SparseBitSlice(t *testing.T) {
	s := NewSparseBitSlice()
	assert.EqualValues(t, 0, s.Len())
	s.AppendBits(1, 1)
	assert.EqualValues(t, 1, s.Len())
	s.AppendBits(1, 1)
	assert.EqualValues(t, 2, s.Len())
	v := s.GetBits(0, 1)
	assert.EqualValues(t, 3, v)
}
