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
func TotalAllocs() uint64 {
	return CurMemStats().Mallocs
}

func CurAlloc() uint64 {
	return CurMemStats().Alloc
}

func MemString(v uint64) string {
	if v < 1024 {
		return fmt.Sprintf("%d", v)
	} else if v < 1024*1024 {
		return fmt.Sprintf("%d (%.2f KiB)", v, float64(v)/float64(1024))
	} else if v < 1024*1024*1024 {
		return fmt.Sprintf("%d (%.2f MiB)", v, float64(v)/float64(1024*1024))
	} else if v < 1024*1024*1024*1024 {
		return fmt.Sprintf("%d (%.2f GiB)", v, float64(v)/float64(1024*1024*1024))
	}
	return fmt.Sprintf("%d (%.2f TiB)", v, float64(v)/float64(1024*1024*1024*1024))
}

func MemSince(prev uint64) string {
	return MemString(TotalAlloc() - prev)
}

func ReportMemDelta(prev uint64) {
	fmt.Printf("[Mem:%s]\n", MemSince(prev))
}
