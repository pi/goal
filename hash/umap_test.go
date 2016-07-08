package hash

import (
	"testing"

	th "github.com/ardente/goal/internal/testhelpers"

	"github.com/stretchr/testify/assert"
)

func newKeygen() th.SeqGen {
	return th.NewSeqGen(th.SgTwist)
}

func Test_UintMapGetPut(t *testing.T) {
	sm := th.TotalAlloc()
	m := NewUintMap()
	kg := newKeygen()
	for i := uint(0); i < th.N; i++ {
		m.Put(kg.Next(), i*2)
	}
	kg.Reset()
	for i := 0; i < th.N; i++ {
		k := kg.Next()
		v := m.Get(k)
		assert.Equal(t, v, uint(i*2))
		m.Put(k, v+1)
		assert.Equal(t, m.Get(k), uint(i*2)+1)
	}
	assert.Equal(t, m.Len(), uint(th.N))
	th.ReportMemdelta(sm)
}

func Test_UintMapIter(t *testing.T) {
	sm := th.TotalAlloc()
	var cksum, cvsum uint
	m := NewUintMap()
	kg := newKeygen()
	for i := 0; i < th.N; i++ {
		k := kg.Next()
		v := (k >> 32) | (k << 32)
		m.Put(k, v)
		cksum += k
		cvsum += v
	}

	var nit, ksum, vsum uint
	for it := m.Iterator(); it.Next(); {
		nit++
		ksum += it.CurKey()
		vsum += it.Cur()
	}
	assert.Equal(t, nit, m.Len())
	assert.Equal(t, cksum, ksum)
	assert.Equal(t, cvsum, vsum)
	th.ReportMemdelta(sm)
}

func Test_UintMapDelete(t *testing.T) {
	//return //TODO
	sm := th.TotalAlloc()
	const N = th.N
	//m := NewUintMap(15)
	//assert.Equal(t, uint(15), uint(m.dirBits))
	m := NewUintMap()
	kg := newKeygen()
	for i := 0; i < N; i++ {
		k := kg.Next()
		m.Put(k, ^k)
	}
	kg.Reset()
	for i := 0; i < N; i++ {
		k := kg.Next()
		if (i & 1) == 1 {
			m.Delete(k)
		}
	}
	kg.Reset()
	for i := 0; i < N; i++ {
		k := kg.Next()
		if (i & 1) == 1 {
			assert.False(t, m.IncludesKey(k))
		} else {
			assert.True(t, m.IncludesKey(k))
		}
	}

	m.Put(0, 33)
	assert.Equal(t, m.Get(0), uint(33))
	m.Delete(0)
	assert.False(t, m.IncludesKey(0))
	th.ReportMemdelta(sm)
}

func Test_UintMapDo(t *testing.T) {
	const N = 1000
	var n int
	kg := newKeygen()
	m := NewUintMap()
	keys := make(map[uint]bool)
	for i := 0; i < N; i++ {
		k := kg.Next()
		keys[k] = true
		m.Put(k, ^k)
	}

	m.Do(func(k, v uint) {
		n++
		if k != ^v {
			t.Fail()
		}
		delete(keys, k)
	})
	assert.Equal(t, len(keys), 0)
	assert.Equal(t, n, N)
}
