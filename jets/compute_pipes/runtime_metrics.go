package compute_pipes

import (
	"log"
	"runtime"
)

func ReportMetrics(config []Metric) {
	if len(config) > 0 {
		var m runtime.MemStats
		f := uint64(1024 * 1024)
		runtime.ReadMemStats(&m)
		log.Printf("ReportMetrics :: Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v",
			m.Alloc/f, m.TotalAlloc/f, m.Sys/f, m.NumGC)
	}
}
