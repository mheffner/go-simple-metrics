package metrics

import (
	"context"
	"sync/atomic"
	"time"
)

type PersistentGauge interface {
	Stop()
	Set(val int64) int64
	Incr(val int64) int64
	Decr(val int64) int64
}

type persistentGauge struct {
	m     *Metrics
	gauge Gauge
	val   int64
}

func (m *Metrics) NewPersistentGauge(key string, labels ...Label) PersistentGauge {
	g := &persistentGauge{
		m:     m,
		gauge: m.NewGauge(key, labels...),
	}

	m.persistedGauges.Store(g, struct{}{})

	return g
}

func (p *persistentGauge) Stop() {
	p.m.persistedGauges.Delete(p)
}

func (p *persistentGauge) Set(newval int64) int64 {
	return atomic.SwapInt64(&p.val, newval)
}

func (p *persistentGauge) Incr(delta int64) int64 {
	return atomic.AddInt64(&p.val, delta)
}

func (p *persistentGauge) Decr(delta int64) int64 {
	return atomic.AddInt64(&p.val, -delta)
}

func (p *persistentGauge) report() {
	curr := atomic.LoadInt64(&p.val)
	p.gauge.Set(float64(curr))
}

type AggregatedCounter interface {
	Stop()
	Incr(delta int64)
}

type aggregatedCounter struct {
	m       *Metrics
	counter Counter
	val     int64
}

func (m *Metrics) NewAggregatedCounter(key string, labels ...Label) AggregatedCounter {
	c := &aggregatedCounter{
		m:       m,
		counter: m.NewCounter(key, labels...),
	}

	m.aggregatedCounters.Store(c, struct{}{})
	return c
}

func (a *aggregatedCounter) Stop() {
	a.m.aggregatedCounters.Delete(a)
}

func (a *aggregatedCounter) Incr(delta int64) {
	atomic.AddInt64(&a.val, delta)
}

func (a *aggregatedCounter) report() {
	curr := atomic.SwapInt64(&a.val, 0)
	// We could elide this if curr == 0?
	a.counter.Incr(float64(curr))
}

//
// Reporting
//

func (m *Metrics) pollPersistedMetrics(ctx context.Context) {
	t := time.NewTicker(m.cfg.PersistentInterval)

	for {
		select {
		case <-t.C:
			m.publishPersistedMetrics()
		case <-ctx.Done():
			// publish one last time
			m.publishPersistedMetrics()
			return
		}
	}
}

func (m *Metrics) publishPersistedMetrics() {
	m.persistedGauges.Range(func(key, value any) bool {
		g, ok := key.(*persistentGauge)
		if !ok {
			// invariant
			return true
		}

		g.report()
		return true
	})

	m.aggregatedCounters.Range(func(key, value any) bool {
		c, ok := key.(*aggregatedCounter)
		if !ok {
			// invariant
			return true
		}

		c.report()
		return true
	})
}
