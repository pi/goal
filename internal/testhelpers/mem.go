package testhelpers

import (
	"fmt"
	"runtime"
)

func TotalAlloc() uint64 {
	ms := &runtime.MemStats{}
	runtime.ReadMemStats(ms)
	return ms.TotalAlloc
}

func KMG(v uint64) string {
	if v < 1024 {
		return fmt.Sprintf("%d", v)
	} else if v < 1024*1024 {
		return fmt.Sprintf("%d (%.2f KiB)", v, float64(v)/float64(1024))
	} else if v < 1024*1024*1024 {
		return fmt.Sprintf("%d (%.2f MiB)", v, float64(v)/float64(1024*1024))
	} else {
		return fmt.Sprintf("%d (%.2f GiB)", v, float64(v)/float64(1024*1024*1024))
	}
}

func ReportMemdelta(startMem uint64) {
	fmt.Printf("[Mem:%s]\n", KMG(TotalAlloc()-startMem))
}
