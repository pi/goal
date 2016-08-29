package _testing

import (
	"fmt"
	"runtime"
	"time"

	"github.com/pi/goal/th"
)

const N = 100000
const S = 32
const BS = S * 1000

const NPIPES = 1000

var _ = runtime.NumCPU

/*
func init() {
	NPIPES = runtime.NumCPU() * 10 //= 1000
}*/

func XferSpeed(nmsg uint64, elapsed time.Duration) string {
	return fmt.Sprintf("mps:%.2fM, %s/s", float64(nmsg)*1000.0/float64(elapsed.Nanoseconds()), th.MemString(uint64(float64(nmsg)*S/elapsed.Seconds())))
}
