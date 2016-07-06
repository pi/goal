package testhelpers

import "math/rand"

type SeqGen interface {
	Seed(value uint)
	Next() uint
	Reset()
}

const (
	SgRand = iota
	SgSeq
	SgTwist
)

func NewSeqGen(sgt int) SeqGen {
	switch sgt {
	case SgRand:
		return &randSG{}
	case SgSeq:
		return &seqSG{}
	case SgTwist:
		return &twistSG{}
	default:
		panic("invalid sequence generator type")
	}
}

type randSG struct {
	r *rand.Rand
}

func (g *randSG) Next() uint {
	if g.r == nil {
		g.r = rand.New(rand.NewSource(1))
	}
	return uint(g.r.Int63())
}
func (g *randSG) Reset() {
	g.r = rand.New(rand.NewSource(1))
}
func (g *randSG) Seed(value uint) {
	g.r = rand.New(rand.NewSource(int64(value)))
}

type seqSG struct {
	cur uint
}

func (g *seqSG) Next() uint {
	g.cur++
	return g.cur
}
func (g *seqSG) Reset() {
	g.cur = 0
}
func (g *seqSG) Seed(value uint) {
	g.cur = value
}

type twistSG struct {
	cur uint
}

func (g *twistSG) Next() uint {
	if (g.cur & 0x8000000000000000) == 0 {
		g.cur = ^g.cur - 1
	} else {
		g.cur = ^g.cur + 1
	}
	return g.cur
}

func (g *twistSG) Reset() {
	g.cur = 0
}
func (g *twistSG) Seed(value uint) {
	g.cur = value
}
