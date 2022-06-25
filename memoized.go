package metrics

import "time"

type MetricEmitter func(val float64)

type baseMetric struct {
	drop    bool
	emitter MetricEmitter
}

type Gauge interface {
	Set(val float64)
}

type gauge struct {
	baseMetric
}

func (m *Metrics) NewGauge(key string, labels ...Label) Gauge {
	g := &gauge{}
	allowed, keys, labels := m.enrich("gauge", key, labels)
	if !allowed {
		g.drop = true
		return g
	}

	g.emitter = m.sink.BuildMetricEmitter(MetricTypeGauge, keys, labels)
	return g
}

func (g *gauge) Set(val float64) {
	if g.drop {
		return
	}

	g.emitter(val)
}

type Counter interface {
	Incr(val float64)
}

type counter struct {
	baseMetric
}

func (m *Metrics) NewCounter(key string, labels ...Label) Counter {
	c := &counter{}
	allowed, keys, labels := m.enrich("counter", key, labels)
	if !allowed {
		c.drop = true
		return c
	}

	c.emitter = m.sink.BuildMetricEmitter(MetricTypeCounter, keys, labels)
	return c
}

func (c *counter) Incr(val float64) {
	if c.drop {
		return
	}

	c.emitter(val)
}

type Timer interface {
	MeasureSince(start time.Time)
}

type timer struct {
	baseMetric
	granularity time.Duration
}

func (m *Metrics) NewTimer(key string, labels ...Label) Timer {
	t := &timer{granularity: m.cfg.TimerGranularity}
	allowed, keys, labels := m.enrich("timer", key, labels)
	if !allowed {
		t.drop = true
		return t
	}

	t.emitter = m.sink.BuildMetricEmitter(MetricTypeTimer, keys, labels)
	return t

}

func (t *timer) MeasureSince(start time.Time) {
	if t.drop {
		return
	}

	now := time.Now()
	elapsed := now.Sub(start)
	msec := float64(elapsed.Nanoseconds()) / float64(t.granularity)

	t.emitter(msec)
}

type Histogram interface {
	Sample(val float64)
}

type histogram struct {
	baseMetric
}

func (m *Metrics) NewHistogram(key string, labels ...Label) Histogram {
	h := &histogram{}
	allowed, keys, labels := m.enrich("histogram", key, labels)
	if !allowed {
		h.drop = true
		return h
	}

	h.emitter = m.sink.BuildMetricEmitter(MetricTypeHistogram, keys, labels)
	return h
}

func (h *histogram) Sample(val float64) {
	if h.drop {
		return
	}

	h.emitter(val)
}
