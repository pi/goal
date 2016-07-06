package goal

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

const N = 100 //10000000

type keygen interface {
	Next() uint
	Reset() uint
}

type randKeygen struct {
	r *rand.Rand
}

func (kg *randKeygen) Next() uint {
	return uint(kg.r.Int63())
}
func newKeygen() *randKeygen {
	return &randKeygen{r: rand.New(rand.NewSource(1))}
}
func (kg *randKeygen) Reset() {
	kg.r = rand.New(rand.NewSource(1))
}

type seqKeygen struct {
	cur uint
}

func (kg *seqKeygen) Next() uint {
	kg.cur++
	return kg.cur
}
func (kg *seqKeygen) Reset() {
	kg.cur = 0
}
func newSeqKeygen() *seqKeygen {
	return &seqKeygen{cur: 0}
}

type twistKeygen struct {
	cur uint
}

func (kg *twistKeygen) Next() uint {
	if (kg.cur & 0x8000000000000000) == 0 {
		kg.cur = ^kg.cur - 1
	} else {
		kg.cur = ^kg.cur + 1
	}
	return kg.cur
}

func (kg *twistKeygen) Reset() {
	kg.cur = 0
}

func newTwistKeygen() *twistKeygen {
	return &twistKeygen{cur: 0}
}

func Test_UintHashMapGetPut(t *testing.T) {
	m := NewUintHashMap()
	kg := newTwistKeygen()
	for i := uint(0); i < N; i++ {
		m.Put(kg.Next(), i*2)
	}
	kg.Reset()
	for i := 0; i < N; i++ {
		k := kg.Next()
		v := m.Get(k)
		assert.Equal(t, v, uint(i*2))
		m.Put(k, v+1)
		assert.Equal(t, m.Get(k), uint(i*2)+1)
	}
	assert.Equal(t, m.Len(), uint(N))
}

func Test_Iter(t *testing.T) {
	var cksum, cvsum uint
	m := NewUintHashMap()
	kg := newTwistKeygen()
	for i := 0; i < N; i++ {
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
}

func Test_UintHashMapDelete(t *testing.T) {
	const N = 5
	m := NewUintHashMap(1 << 15)
	assert.Equal(t, uint(15), uint(m.dirBits))
	kg := newTwistKeygen()
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
}
