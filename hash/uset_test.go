package hash

// todo

import (
	"testing"

	th "github.com/ardente/goal/internal/testhelpers"

	"github.com/stretchr/testify/assert"
)

func newKeygen() th.SeqGen {
	return th.NewSeqGen(th.SgRand)
}

func Test_UintSet(t *testing.T) {
	sm := th.TotalAlloc()
	s := NewUintSet()
	kg := newKeygen()

	for i := uint(0); i < th.N; i++ {
		s.Add(i)
	}

	for i := uint(0); i < th.N; i += 2 {
		s.Delete(i)
	}
	for i := uint(1); i < th.N; i += 2 {
		s.Delete(i)
	}
	assert.EqualValues(t, 0, s.Len())

	for i := 0; i < th.N; i++ {
		s.Add(uint(kg.Next()))
	}
	kg.Reset()
	for i := uint64(0); i < th.N; i++ {
		assert.True(t, s.Includes(uint(kg.Next())))
	}
	th.ReportMemdelta(sm)
}

func Test_UintSetLarge(t *testing.T) {
	g := th.NewSeqGen(th.SgRand)
	s := NewUintSet()
	for {
		c := s.Len()
		if (c%1000000 == 0) && (c != 0) {
			println(c / 1000000)
		}
		v := g.Next()
		s.Add(v)
		if c == s.Len() {
			println("unique randoms:", c)
			return
		}
	}
}

func TestUsetIntersection(t *testing.T) {

}
