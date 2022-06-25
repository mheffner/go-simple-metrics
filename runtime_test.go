package metrics

import (
	"runtime"
	"testing"
)

func TestMetrics_EmitRuntimeStats(t *testing.T) {
	runtime.GC()
	m, met := mockMetric(t)

	rm := &runtimeMetrics{
		numGoroutines:  met.NewGauge("runtime.num_goroutines"),
		allocBytes:     met.NewGauge("runtime.alloc_bytes"),
		sysBytes:       met.NewGauge("runtime.sys_bytes"),
		mallocCount:    met.NewGauge("runtime.malloc_count"),
		freeCount:      met.NewGauge("runtime.free_count"),
		heapObjects:    met.NewGauge("runtime.heap_objects"),
		totalGCPauseNS: met.NewGauge("runtime.total_gc_pause_ns"),
		totalGCRuns:    met.NewGauge("runtime.total_gc_runs"),
		gcPauseNS:      met.NewHistogram("runtime.gc_pause_ns"),
	}

	lastNumGC := uint32(0)

	met.emitRuntimeStats(rm, &lastNumGC)

	// check the first key, assume others ok
	if m.getKeys()[0][0] != "runtime.num_goroutines" {
		t.Fatalf("bad key %v", m.getKeys())
	}
	if m.vals[0] <= 1 {
		t.Fatalf("bad val: %v", m.vals)
	}

	if m.vals[1] <= 40000 {
		t.Fatalf("bad val: %v", m.vals)
	}

	if m.vals[2] <= 100000 {
		t.Fatalf("bad val: %v", m.vals)
	}

	if m.vals[3] <= 100 {
		t.Fatalf("bad val: %v", m.vals)
	}

	if m.vals[4] <= 100 {
		t.Fatalf("bad val: %v", m.vals)
	}

	if m.vals[5] <= 100 {
		t.Fatalf("bad val: %v", m.vals)
	}

	if m.vals[6] <= 100 {
		t.Fatalf("bad val: %v\nkeys: %v", m.vals, m.getKeys())
	}

	if m.vals[7] < 1 {
		t.Fatalf("bad val: %v", m.vals)
	}

	if m.vals[8] <= 1000 {
		t.Fatalf("bad val: %v", m.vals)
	}
}
