package gut

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemPool(t *testing.T) {
	p := NewUnsafeMemoryPool(8192)
	assert.EqualValues(t, len(p.mem), 8192)
	assert.EqualValues(t, cap(p.mem), 8192)
	assert.Fail(t, "test AllocBytes etc")
}
