package metrics

import "time"

// Proxy all the methods to the globalMetrics instance
func (m *Metrics) SetGauge(key string, val float64, labels ...Label) {
	m.NewGauge(key, labels...).Set(val)
}

func (m *Metrics) Incr(key string, val float64, labels ...Label) {
	m.NewCounter(key, labels...).Incr(val)
}

func (m *Metrics) Sample(key string, val float64, labels ...Label) {
	m.NewHistogram(key, labels...).Sample(val)
}

func (m *Metrics) MeasureSince(key string, start time.Time, labels ...Label) {
	m.NewTimer(key, labels...).MeasureSince(start)
}

func (m *Metrics) Observe(key string, val float64, labels ...Label) {
	m.NewDistribution(key, labels...).Observe(val)
}

func (m *Metrics) Shutdown() {
	if m.runtimeMetricsCancel != nil {
		m.runtimeMetricsCancel()
		m.runtimeWaitG.Wait()
	}
	if m.persistedPublishCancel != nil {
		m.persistedPublishCancel()
		m.persistedPublishWaitG.Wait()
	}

	if ss, ok := m.sink.(ShutdownSink); ok {
		ss.Shutdown()
	}
}

// Creates a new slice with the provided string value as the first element
// and the provided slice values as the remaining values.
// Ordering of the values in the provided input slice is kept in tact in the output slice.
func insert(i int, v string, s []string) []string {
	// Allocate new slice to avoid modifying the input slice
	newS := make([]string, len(s)+1)

	// Copy s[0, i-1] into newS
	for j := 0; j < i; j++ {
		newS[j] = s[j]
	}

	// Insert provided element at index i
	newS[i] = v

	// Copy s[i, len(s)-1] into newS starting at newS[i+1]
	for j := i; j < len(s); j++ {
		newS[j+1] = s[j]
	}

	return newS
}
