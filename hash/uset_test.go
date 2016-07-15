package hash

// todo

import (
	"testing"

	"github.com/pi/goal/th"

	. "github.com/pi/goal/internal/testhelpers"

	"github.com/stretchr/testify/assert"
)

func Test_UintSet(t *testing.T) {
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
}

func reportUniqueKeyShortage(actual, needed uint) {
	println("\n!!! SHORT OF UNIQUE KEYS:", actual, "OF", needed)
}

func fillLargeUintSet() {
	g := th.NewSeqGen(th.SgRand)
	s := NewUintSet()
	limit := uint(10 * 1000 * 1000)
	for {
		l := s.Len()
		if (l%1000000 == 0) && (l != 0) {
			//print(".")
		}
		v := g.Next()
		s.Add(v)
		if l == s.Len() || l == limit {
			if l == s.Len() {
				reportUniqueKeyShortage(l, limit)
			}
			return
		}
	}
}

func fillLargeMap() {
	g := th.NewSeqGen(th.SgRand)
	s := make(map[uint]bool)
	limit := 10 * 1000 * 1000
	for {
		l := len(s)
		if (l%1000000 == 0) && (l != 0) {
			//print(".")
		}
		v := g.Next()
		s[v] = true
		if l == len(s) || l == limit {
			if l == len(s) {
				reportUniqueKeyShortage(uint(l), uint(limit))
			}
			return
		}
	}
}

func Test_UintSetLarge(t *testing.T) {
	fillLargeUintSet()
}
func Benchmark_UintSetLarge(b *testing.B) {
	b.ReportAllocs()
	fillLargeUintSet()
}

func Test_MapLarge(t *testing.T) {
	fillLargeMap()
}

func Benchmark_MapLarge(b *testing.B) {
	b.ReportAllocs()
	fillLargeMap()
}

func TestUsetIntersection(t *testing.T) {
	//	t.Fail()
}

func TestUsetCopy(t *testing.T) {
	//	t.Fail()
}
