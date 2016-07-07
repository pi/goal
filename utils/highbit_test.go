package utils

import (
	"testing"

	th "github.com/ardente/goal/internal/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestBitLen(t *testing.T) {
	bl := func(v uint) (n int) {
		for v > 0 {
			n++
			v >>= 1
		}
		return
	}

	g := th.NewSeqGen(th.SgRand)
	for i := 0; i < 10000; i++ {
		v := g.Next()
		assert.Equal(t, bl(v), BitLen(v), "%x", v)
	}
}
