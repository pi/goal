package gut

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func poolTestHelper(t *testing.T, p *UnsafeMemoryPool) {
	assert.EqualValues(t, len(p.mem), 8192)
	assert.EqualValues(t, cap(p.mem), 8192)
	//assert.Fail(t, "test AllocBytes etc")
}
func TestUnsafeMemPool(t *testing.T) {
	p, err := NewUnsafeMemoryPool(8192)
	assert.NoError(t, err)
	poolTestHelper(t, p)
	p.Done()
}

func TestSharedUnsafeMemPool(t *testing.T) {
	p, err := NewSharedUnsafeMemoryPool(8192)
	assert.NoError(t, err)
	poolTestHelper(t, p)
	p.Done()
}
