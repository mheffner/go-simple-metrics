package metrics

import (
	"context"
	"runtime"
	"time"
)

type runtimeMetrics struct {
	numGoroutines  Gauge
	allocBytes     Gauge
	sysBytes       Gauge
	mallocCount    Gauge
	freeCount      Gauge
	heapObjects    Gauge
	totalGCPauseNS Gauge
	totalGCRuns    Gauge
	gcPauseNS      Histogram
}

// Periodically collects runtime stats to publish
func (m *Metrics) collectStats(ctx context.Context) {
	rm := &runtimeMetrics{
		numGoroutines:  m.NewGauge("runtime.num_goroutines"),
		allocBytes:     m.NewGauge("runtime.alloc_bytes"),
		sysBytes:       m.NewGauge("runtime.sys_bytes"),
		mallocCount:    m.NewGauge("runtime.malloc_count"),
		freeCount:      m.NewGauge("runtime.free_count"),
		heapObjects:    m.NewGauge("runtime.heap_objects"),
		totalGCPauseNS: m.NewGauge("runtime.total_gc_pause_ns"),
		totalGCRuns:    m.NewGauge("runtime.total_gc_runs"),
		gcPauseNS:      m.NewHistogram("runtime.gc_pause_ns"),
	}

	lastNumGC := uint32(0)

	t := time.NewTicker(m.cfg.ProfileInterval)

	for {
		select {
		case <-t.C:
			m.emitRuntimeStats(rm, &lastNumGC)
		case <-ctx.Done():
			return
		}
	}
}

// Emits various runtime statsitics
func (m *Metrics) emitRuntimeStats(rm *runtimeMetrics, lastNumGC *uint32) {
	// Export number of Goroutines
	numRoutines := runtime.NumGoroutine()
	rm.numGoroutines.Set(float64(numRoutines))

	// Export memory stats
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	rm.allocBytes.Set(float64(stats.Alloc))
	rm.sysBytes.Set(float64(stats.Sys))
	rm.mallocCount.Set(float64(stats.Mallocs))
	rm.freeCount.Set(float64(stats.Frees))
	rm.heapObjects.Set(float64(stats.HeapObjects))
	rm.totalGCPauseNS.Set(float64(stats.PauseTotalNs))
	rm.totalGCRuns.Set(float64(stats.NumGC))

	// Export info about the last few GC runs
	num := stats.NumGC

	// Handle wrap around
	if num < *lastNumGC {
		*lastNumGC = 0
	}

	// Ensure we don't scan more than 256
	if num-*lastNumGC >= 256 {
		*lastNumGC = num - 255
	}

	for i := *lastNumGC; i < num; i++ {
		pause := stats.PauseNs[i%256]
		rm.gcPauseNS.Sample(float64(pause))
	}
	*lastNumGC = num
}
