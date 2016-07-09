package th

import (
	"fmt"
	"runtime"
)

func CurMemStats() *runtime.MemStats {
	ms := &runtime.MemStats{}
	runtime.ReadMemStats(ms)
	return ms
}
func TotalAlloc() uint64 {
	return CurMemStats().TotalAlloc
}

func CurAlloc() uint64 {
	return CurMemStats().Alloc
}

func MemSince(prev uint64) string {
	v := TotalAlloc() - prev
	if v < 1024 {
		return fmt.Sprintf("%d", v)
	} else if v < 1024*1024 {
		return fmt.Sprintf("%d (%.2f KiB)", v, float64(v)/float64(1024))
	} else if v < 1024*1024*1024 {
		return fmt.Sprintf("%d (%.2f MiB)", v, float64(v)/float64(1024*1024))
	}
	return fmt.Sprintf("%d (%.2f GiB)", v, float64(v)/float64(1024*1024*1024))
}

func ReportMemDelta(prev uint64) {
	fmt.Printf("[Mem:%s]\n", MemSince(prev))
}
