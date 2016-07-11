package main

import (
	"encoding/gob"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	. "github.com/ardente/goal/gut"
	"github.com/ardente/goal/hash"
	"github.com/ardente/goal/th"
)

var _ = gob.NewEncoder
var _ = os.Open
var _ = Perr

func t(f func()) {
	st := time.Now()
	f()
	fmt.Printf("elapsed: %v\n", time.Since(st))
}

type benchRec struct {
	period uint64
	time   [2]time.Duration // 0 - native, 1 - my
	mem    [2]uint64        // 0 - native, 1 - my
	allocs [2]uint64
}

func percLess(my, native uint64) float64 {
	return 100.0 - float64(my)*100.0/float64(native)
}
func (s benchRec) String() string {
	return fmt.Sprintf("%dk\t%d\t%d\t%d\t%d\t%.2f%%\t%.2f%%", s.period/1000,
		s.time[0]/time.Millisecond, s.time[1]/time.Millisecond,
		s.mem[0], s.mem[1],
		percLess(uint64(s.time[1]), uint64(s.time[0])), percLess(s.mem[1], s.mem[0]))
}
func (s benchRec) CsvString() string {
	str := fmt.Sprintf("%d\t%.3f\t%.3f\t%.3f\t%.3f\t%.2f\t%.2f", s.period/1000,
		float64(s.time[0])/float64(time.Second), float64(s.time[1])/float64(time.Second),
		float64(s.mem[0])/(10*1024*1024), float64(s.mem[1])/(10*1024*1024),
		float64(s.time[0])/float64(s.time[1]), float64(s.mem[0])/float64(s.mem[1]))

	return strings.Replace(str, ".", ",", -1)
}

func testMaps() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	const N = 10 * 1000 * 1000 //0
	const Step = 1 * 1000      //0
	const Steps = 100

	newSeqGen := func(p uint) th.SeqGen {
		g := th.NewSeqGen(th.SgRand)
		g.SetPeriod(p)
		return g
	}

	funcs := [2]func(th.SeqGen){
		func(g th.SeqGen) {
			m := make(map[uint]uint)
			// fill
			for i := 0; i < N; i++ {
				m[g.Next()]++
			}
			/*
				// iter
				rc := 0
				for _, _ = range m {
					rc++
				}
				// del
				for i := 0; i < N; i++ {
					delete(m, g.Next())
				}
			*/
		},
		func(g th.SeqGen) {
			m := hash.NewUintMap()
			// fill
			for i := 0; i < N; i++ {
				m.Inc(g.Next(), 1)
			}
			/*
				// iter
				rc := 0
				for it := m.Iterator(); it.Next(); rc++ {
				}
				// del
				g.Reset()
				for i := 0; i < N; i++ {
					m.Delete(g.Next())
				}
			*/
		},
	}

	var samples []benchRec

	for p := 0; p < Steps; p++ {
		var curSample benchRec

		curSample.period = uint64((p + 1) * Step)
		for i := 0; i < 2; i++ {
			runtime.GC()
			st := time.Now()
			mem := th.TotalAlloc()
			funcs[i](newSeqGen(uint(curSample.period)))
			curSample.mem[i] = th.TotalAlloc() - mem
			curSample.time[i] = time.Since(st)
		}

		samples = append(samples, curSample)

		fmt.Printf("%s\n", curSample.String())
	}

	fmt.Printf("======= csv =======\n")
	fmt.Printf("# uq.keys, k\tgo.time, s\tmy.time, s\tgo.mem, 10MiBs\tmy.mem,10MiBs\ttimes faster\ttimes smaller\n")

	for _, s := range samples {
		fmt.Printf("%s\n", s.CsvString())
	}
	fmt.Printf("\ndone\n")
}

func main() {
	//testMaps()
	for i := 0; i < 10; i++ {
		_, _ = testSet()
	}
}

func testSet() (mem uint64, took time.Duration) {
	const fn = "results.txt"
	const label = "rk1"
	var f *os.File
	var err error
	if _, e := os.Stat(fn); e == nil {
		f, err = os.OpenFile(fn, os.O_RDWR, 0666)
		f.Seek(0, os.SEEK_END)
	} else {
		f, err = os.OpenFile("results.txt", os.O_RDWR|os.O_CREATE, 0666)
	}
	if err != nil {
		panic(err)
	}
	defer f.Close()

	st := time.Now()
	sm := th.TotalAlloc()
	sa := th.TotalAllocs()
	m := hash.NewUintSet()
	g := th.NewSeqGen(th.SgRand)
	for i := uint(1); i < 30000000; i++ {
		m.Add(g.Next())
	}
	mem = th.TotalAlloc() - sm
	took = time.Since(st)
	s := fmt.Sprintf("%s\t%s\t%d\t%s\n", label, th.MemSince(sm), th.TotalAllocs()-sa, took.String())
	f.WriteString(s)
	print(s)
	return mem, took
}
