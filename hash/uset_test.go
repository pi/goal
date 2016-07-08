package hash

// todo

import (
	"testing"

	"github.com/ardente/goal/th"

	. "github.com/ardente/goal/internal/testhelpers"

	"github.com/stretchr/testify/assert"
)

func Test_UintSet(t *testing.T) {
	sm := th.TotalAlloc()
	defer th.ReportMemDelta(sm)

	s := NewUintSet()
	kg := th.NewSeqGen(th.SgRand)

	for i := uint(0); i < N; i++ {
		s.Add(i)
	}

	for i := uint(0); i < N; i += 2 {
		s.Delete(i)
	}
	for i := uint(1); i < N; i += 2 {
		s.Delete(i)
	}
	assert.EqualValues(t, 0, s.Len())

	for i := 0; i < N; i++ {
		s.Add(uint(kg.Next()))
	}
	kg.Reset()
	for i := uint64(0); i < N; i++ {
		assert.True(t, s.Includes(uint(kg.Next())))
	}
	println("memuse", s.memuse())
}

func Test_UintSetLarge(t *testing.T) {
	sm := th.TotalAlloc()
	defer th.ReportMemDelta(sm)

	g := th.NewSeqGen(th.SgRand)
	s := NewUintSet()
	limit := uint(1 * 1000 * 1000)
	for {
		l := s.Len()
		if (l%1000000 == 0) && (l != 0) {
			print(".")
		}
		v := g.Next()
		s.Add(v)
		if l == s.Len() || l == limit {
			println("\nunique randoms:", l, "memuse:", s.memuse(), "dir:", len(s.dir))
			return
		}
	}
}

func Test_MapLarge(t *testing.T) {
	sm := th.TotalAlloc()
	defer th.ReportMemDelta(sm)

	g := th.NewSeqGen(th.SgRand)
	s := make(map[uint]bool)
	limit := 1 * 1000 * 1000
	for {
		l := len(s)
		if (l%1000000 == 0) && (l != 0) {
			print(".")
		}
		v := g.Next()
		s[v] = true
		if l == len(s) || l == limit {
			println("\nunique randoms:", l)
			return
		}
	}
}

func TestUsetIntersection(t *testing.T) {
	t.Fail()
}

func TestUsetCopy(t *testing.T) {
	t.Fail()
}
