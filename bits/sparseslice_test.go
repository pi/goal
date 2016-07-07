package bits

import (
	"testing"

	th "github.com/ardente/goal/internal/testhelpers"
	"github.com/stretchr/testify/assert"
)

func Test_SparseBitSlice(t *testing.T) {
	s := NewSparseBitSlice()
	assert.EqualValues(t, 0, s.Len())
	s.AppendBits(1, 1)
	assert.EqualValues(t, 1, s.Len())
	s.AppendBits(1, 1)
	assert.EqualValues(t, 2, s.Len())
	v := s.GetBitRange(0, 1)
	assert.EqualValues(t, 3, v)
}

/*
func Test_GiganticSparseSlice(t *testing.T) {
	g := th.NewSeqGen(th.SgRand)
	s := NewSparseBitSlice()
	s.SetLen(^uint(0))
	var i uint
	for {
		if (i%1000000 == 0) && (i != 0) {
			println(i / 1000000)
		}
		v := g.Next()
		if s.GetBit(v) {
			println("unique randoms:", i)
			return
		}
		s.PutBit(v, true)
		i++
	}
}
*/
