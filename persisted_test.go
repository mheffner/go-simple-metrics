package metrics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPersistentGauge(t *testing.T) {
	m, met := mockMetric(t)

	label := L("label", "value")
	pg := met.NewPersistentGauge("pkey", label)
	pg.Set(1)

	met.publishPersistedMetrics()

	require.Len(t, m.keys, 1)

	require.Equal(t, "pkey", m.keys[0][0])
	require.Equal(t, float64(1), m.vals[0])
	require.Equal(t, []Label{label}, m.labels[0])

	pg.Incr(2)
	met.publishPersistedMetrics()

	require.Len(t, m.keys, 2)
	require.Equal(t, float64(3), m.vals[1])

	pg.Stop()

	met.publishPersistedMetrics()
	require.Len(t, m.keys, 2)
}

func TestAggregatedCounter(t *testing.T) {
	m, met := mockMetric(t)

	label := L("label", "value")
	ag := met.NewAggregatedCounter("ckey", label)
	ag.Incr(3)

	met.publishPersistedMetrics()

	require.Len(t, m.keys, 1)

	require.Equal(t, "ckey", m.keys[0][0])
	require.Equal(t, float64(3), m.vals[0])
	require.Equal(t, []Label{label}, m.labels[0])

	ag.Incr(5)
	met.publishPersistedMetrics()

	require.Len(t, m.keys, 2)
	require.Equal(t, float64(5), m.vals[1])

	ag.Stop()

	met.publishPersistedMetrics()

	require.Len(t, m.keys, 2)
}
