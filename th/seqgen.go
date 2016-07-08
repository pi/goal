package th

import "math/rand"

type SeqGen interface {
	Seed(value uint)
	Next() uint
	Reset()
	SetPeriod(period uint)
	Period() uint
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
	r         *rand.Rand
	period    uint
	generated uint
}

func (g *randSG) Next() uint {
	if g.period != 0 && g.period == g.generated {
		g.Reset()
	}
	if g.r == nil {
		g.r = rand.New(rand.NewSource(1))
	}
	g.generated++
	return uint(g.r.Int63())
}
func (g *randSG) Reset() {
	g.r = rand.New(rand.NewSource(1))
	g.generated = 0
}
func (g *randSG) Seed(value uint) {
	g.r = rand.New(rand.NewSource(int64(value)))
}
func (g *randSG) SetPeriod(period uint) {
	g.period = period
}
func (g *randSG) Period() uint {
	return g.period
}

type seqSG struct {
	cur    uint
	period uint
}

func (g *seqSG) Next() uint {
	g.cur++
	if g.period != 0 {
		return g.cur % g.period
	} else {
		return g.cur
	}
}
func (g *seqSG) Reset() {
	g.cur = 0
}
func (g *seqSG) Seed(value uint) {
	g.cur = value
}
func (g *seqSG) SetPeriod(period uint) {
	g.period = period
}
func (g *seqSG) Period() uint {
	return g.period
}

type twistSG struct {
	cur               uint
	period, generated uint
}

func (g *twistSG) Next() uint {
	if g.period != 0 && g.generated == g.period {
		g.Reset()
	}
	if (g.cur & 0x8000000000000000) == 0 {
		g.cur = ^g.cur - 1
	} else {
		g.cur = ^g.cur + 1
	}
	g.generated++
	return g.cur
}

func (g *twistSG) Reset() {
	g.cur = 0
	g.generated = 0
}
func (g *twistSG) Seed(value uint) {
	g.cur = value
}
func (g *twistSG) SetPeriod(period uint) {
	g.period = period
	g.generated = 0
}
func (g *twistSG) Period() uint {
	return g.period
}
