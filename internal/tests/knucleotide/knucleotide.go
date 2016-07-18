/* The Computer Language Benchmarks Game
 * http://benchmarksgame.alioth.debian.org/
 *
 * contributed by Tylor Arndt
 */

package main

import (
	"bufio"
	"bytes"
	//"encoding/binary"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/pi/goal/hash"
	"github.com/pi/goal/th"
)

var useNative = true

const useStdin = false
const parallel = false

var writeSeqs = false

type countMap interface {
	Len() uint
	Get(uint) uint
	Inc(uint, uint)
	Do(func(uint, uint))
}

type nativeCountMap map[uint]uint

func (m nativeCountMap) Inc(key uint, delta uint) {
	m[key]++ // !!!
}
func (m nativeCountMap) Get(key uint) uint {
	return uint(m[key])
}
func (m nativeCountMap) Do(f func(uint, uint)) {
	for k, v := range m {
		f(k, v)
	}
}
func (m nativeCountMap) Len() uint {
	return uint(len(m))
}

func newCountMap() countMap {
	if useNative {
		return make(nativeCountMap)
	} else {
		return hash.NewUintMap()
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	useNative = true
	bench()
	writeSeqs = false

	/*runtime.GC()

	useNative = false
	bench()*/
}

func bench() {
	st := time.Now()
	sm := th.TotalAlloc()
	defer th.ReportMemDelta(sm)

	dna := readEncDNA()
	cl := new([7]chan string)
	for i := 0; i < 7; i++ {
		cl[i] = make(chan string)
	}
	report(cl[0], dna, 1)
	report(cl[1], dna, 2)
	report(cl[2], dna, 3)
	report(cl[3], dna, 4)
	report(cl[4], dna, 6)
	report(cl[5], dna, 12)
	report(cl[6], dna, 18)
	if parallel {
		for i := 0; i < 7; i++ {
			fmt.Print(<-cl[i])
		}
	}
	fmt.Printf("\ntook %v", time.Since(st))
}

func readEncDNA() []byte {
	var f *os.File

	if useStdin {
		f = os.Stdin
	} else {
		const fn = "fasta25000000.txt"
		var err error
		f, err = os.Open(fn)
		if err != nil {
			panic("can't open " + fn)
		}
		defer f.Close()
	}
	in, startTok := bufio.NewReader(f), []byte(">THREE ")
	for line, err := in.ReadSlice('\n'); !bytes.HasPrefix(line, startTok); line, err = in.ReadSlice('\n') {
		if err != nil {
			log.Panicf("Error: Could not read input from stdin; Details: %s", err)
		}
	}
	ascii, err := ioutil.ReadAll(in)
	if err != nil {
		log.Panicf("Error: Could not read input from stdin; Details: %s", err)
	}
	j := 0
	for i, c, asciic := 0, byte(0), len(ascii); i < asciic; i++ {
		c = ascii[i]
		switch c {
		case 'a', 'A':
			c = 0
		case 't', 'T':
			c = 1
		case 'g', 'G':
			c = 2
		case 'c', 'C':
			c = 3
		case '\n':
			continue
		default:
			log.Fatalf("Error: Invalid nucleotide value: '%c'", ascii[i])
		}
		ascii[j] = c
		j++
	}
	return ascii[:j+1]
}

var targSeqs = []string{3: "GGT", 4: "GGTA", 6: "GGTATT", 12: "GGTATTTTAATT", 18: "GGTATTTTAATTTATAGT"}

func report(rc chan string, dna []byte, n int) {
	rfunc := func() {
		sm := th.TotalAlloc()
		st := time.Now()
		tbl, output := count(dna, n), ""
		switch n {
		case 1, 2:
			output = freqReport(tbl, n)
		default:
			targ := targSeqs[n]
			output = fmt.Sprintf("%d\t%s| %d %s %v\n", tbl.Get(uint(compStr(targ))), targ, tbl.Len(), th.MemSince(sm), time.Since(st))
		}
		if parallel {
			rc <- output
		} else {
			print(output)
		}
	}
	if parallel {
		go rfunc()
	} else {
		rfunc()
	}
}

func count(dna []byte, n int) countMap {
	tbl := newCountMap()

	if writeSeqs {
		var st time.Time

		seq := make([]uint64, len(dna))
		ns := 0
		st = time.Now()
		for i, end := 0, len(dna)-n; i < end; i++ {
			seq[ns] = compress(dna[i : i+n])
			ns++
		}
		seq = seq[:ns]
		fmt.Printf("[parse time:%v]", time.Since(st))

		f, err := os.Create(fmt.Sprintf("seqs.%d", n))
		if err != nil {
			panic(err.Error())
		}
		defer f.Close()
		st = time.Now()
		enc := gob.NewEncoder(f)
		enc.Encode(seq)
		fmt.Printf("[write time:%v]", time.Since(st))

		for _, s := range seq {
			tbl.Inc(uint(s), 1)
		}
	} else {
		for i, end := 0, len(dna)-n; i < end; i++ {
			tbl.Inc(uint(compress(dna[i:i+n])), 1)
		}
	}
	return tbl
}

func compress(dna []byte) uint64 {
	var val uint64
	for i, dnac := 0, len(dna); i < dnac; i++ {
		val = (val << 2) | uint64(dna[i])
	}
	return val
}

func compStr(dna string) uint64 {
	raw := []byte(dna)
	for i, rawc, c := 0, len(raw), byte(0); i < rawc; i++ {
		c = raw[i]
		switch c {
		case 'A':
			c = 0
		case 'T':
			c = 1
		case 'G':
			c = 2
		case 'C':
			c = 3
		}
		raw[i] = c
	}
	return compress(raw)
}

func decompToBytes(compDNA uint64, n int) []byte {
	buf := bytes.NewBuffer(make([]byte, 0, n))
	var c byte
	for i := 0; i < n; i++ {
		switch compDNA & 3 {
		case 0:
			c = 'A'
		case 1:
			c = 'T'
		case 2:
			c = 'G'
		case 3:
			c = 'C'
		}
		buf.WriteByte(c)
		compDNA = compDNA >> 2
	}
	if n > 1 {
		return reverse(buf.Bytes())
	}
	return buf.Bytes()
}

func reverse(s []byte) []byte {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

func freqReport(tbl countMap, n int) string {
	seqs := make(seqSlice, 0, tbl.Len())
	var sum uint64
	tbl.Do(func(val, count uint) {
		seqs = append(seqs, seq{nuc: decompToBytes(uint64(val), n), n: uint64(count)})
		sum += uint64(count)
	})
	sort.Sort(seqs)
	var buf bytes.Buffer
	sumFloat, entry := float64(sum), seq{}
	for _, entry = range seqs {
		fmt.Fprintf(&buf, "%s %.3f\n", entry.nuc, (100*float64(entry.n))/sumFloat)
	}
	buf.WriteByte('\n')
	return buf.String()
}

type seq struct {
	nuc []byte
	n   uint64
}

type seqSlice []seq

func (seq seqSlice) Len() int      { return len(seq) }
func (seq seqSlice) Swap(i, j int) { seq[i], seq[j] = seq[j], seq[i] }
func (seq seqSlice) Less(i, j int) bool {
	if seq[i].n == seq[j].n {
		return bytes.Compare(seq[i].nuc, seq[j].nuc) < 0
	}
	return seq[i].n > seq[j].n
}
