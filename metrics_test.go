package metrics

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testOptConf func(c *Config)

func mockMetric(t *testing.T, opts ...testOptConf) (*MockSink, *Metrics) {
	m := &MockSink{}
	c := Config{FilterDefault: true}
	for _, opt := range opts {
		opt(&c)
	}
	met := &Metrics{cfg: c, sink: m}
	t.Cleanup(func() {
		met.Shutdown()
	})
	return m, met
}

func TestMetrics_BaseLabels(t *testing.T) {
	labels := []Label{
		L("dog", "cat"),
		L("coffee", "tea"),
	}

	m, met := mockMetric(t, func(c *Config) {
		c.BaseLabels = labels
	})

	met.SetGauge("gkey", 1, L("gauge", "label"))

	all := append([]Label{L("gauge", "label")}, labels...)

	require.Len(t, m.labels[0], 3)
	require.Equal(t, all, m.labels[0])
}

func TestMetrics_SetGauge(t *testing.T) {
	m, met := mockMetric(t)
	met.SetGauge("key", float64(1))
	if m.getKeys()[0][0] != "key" {
		t.Fatalf("")
	}
	if m.vals[0] != 1 {
		t.Fatalf("")
	}

	m, met = mockMetric(t)
	labels := []Label{{"a", "b"}}
	met.SetGauge("key", float64(1), labels...)
	if m.getKeys()[0][0] != "key" {
		t.Fatalf("")
	}
	if m.vals[0] != 1 {
		t.Fatalf("")
	}
	if !reflect.DeepEqual(m.labels[0], labels) {
		t.Fatalf("")
	}

	m, met = mockMetric(t, func(c *Config) {
		c.EnableTypePrefix = true
	})
	met.SetGauge("key", float64(1))
	if m.getKeys()[0][0] != "gauge" || m.getKeys()[0][1] != "key" {
		t.Fatalf("")
	}
	if m.vals[0] != 1 {
		t.Fatalf("")
	}

	m, met = mockMetric(t, func(c *Config) {
		c.ServiceName = "service"
	})
	met.SetGauge("key", float64(1))
	require.Equal(t, "key", m.getKeys()[0][0])
	if m.vals[0] != 1 {
		t.Fatalf("")
	}
}

func TestMetrics_Incr(t *testing.T) {
	m, met := mockMetric(t)
	met.Incr("key", float64(1))
	if m.getKeys()[0][0] != "key" {
		t.Fatalf("")
	}
	if m.vals[0] != 1 {
		t.Fatalf("")
	}

	m, met = mockMetric(t)
	labels := []Label{{"a", "b"}}
	met.Incr("key", float64(1), labels...)
	if m.getKeys()[0][0] != "key" {
		t.Fatalf("")
	}
	if m.vals[0] != 1 {
		t.Fatalf("")
	}
	if !reflect.DeepEqual(m.labels[0], labels) {
		t.Fatalf("")
	}

	m, met = mockMetric(t, func(c *Config) {
		c.EnableTypePrefix = true
	})
	met.Incr("key", float64(1))
	if m.getKeys()[0][0] != "counter" || m.getKeys()[0][1] != "key" {
		t.Fatalf("")
	}
	if m.vals[0] != 1 {
		t.Fatalf("")
	}

	m, met = mockMetric(t, func(c *Config) {
		c.ServiceName = "service"
	})
	met.Incr("key", float64(1))
	require.Equal(t, "key", m.getKeys()[0][0])
	if m.vals[0] != 1 {
		t.Fatalf("")
	}
}

func TestMetrics_Sample(t *testing.T) {
	m, met := mockMetric(t)
	met.Sample("key", float64(1))
	if m.getKeys()[0][0] != "key" {
		t.Fatalf("")
	}
	if m.vals[0] != 1 {
		t.Fatalf("")
	}

	m, met = mockMetric(t)
	labels := []Label{{"a", "b"}}
	met.Sample("key", float64(1), labels...)
	if m.getKeys()[0][0] != "key" {
		t.Fatalf("")
	}
	if m.vals[0] != 1 {
		t.Fatalf("")
	}
	if !reflect.DeepEqual(m.labels[0], labels) {
		t.Fatalf("")
	}

	m, met = mockMetric(t, func(c *Config) {
		c.EnableTypePrefix = true
	})
	met.Sample("key", float64(1))
	if m.getKeys()[0][0] != "histogram" || m.getKeys()[0][1] != "key" {
		t.Fatalf("%+v", m.getKeys())
	}
	if m.vals[0] != 1 {
		t.Fatalf("")
	}

	m, met = mockMetric(t, func(c *Config) {
		c.ServiceName = "service"
	})
	met.Sample("key", float64(1))
	require.Equal(t, "key", m.getKeys()[0][0])
	if m.vals[0] != 1 {
		t.Fatalf("")
	}
}

func TestMetrics_MeasureSince(t *testing.T) {
	m, met := mockMetric(t, func(c *Config) {
		c.TimerGranularity = time.Millisecond
	})
	met.MeasureSince("key", time.Now())
	if m.getKeys()[0][0] != "key" {
		t.Fatalf("")
	}
	if m.vals[0] > 0.5 {
		t.Fatalf("")
	}

	m, met = mockMetric(t, func(c *Config) {
		c.TimerGranularity = time.Millisecond
	})
	labels := []Label{{"a", "b"}}
	met.MeasureSince("key", time.Now(), labels...)
	if m.getKeys()[0][0] != "key" {
		t.Fatalf("")
	}
	if m.vals[0] > 0.5 {
		t.Fatalf("")
	}
	if !reflect.DeepEqual(m.labels[0], labels) {
		t.Fatalf("")
	}

	m, met = mockMetric(t, func(c *Config) {
		c.TimerGranularity = time.Millisecond
		c.EnableTypePrefix = true
	})
	met.MeasureSince("key", time.Now())
	if m.getKeys()[0][0] != "timer" || m.getKeys()[0][1] != "key" {
		t.Fatalf("")
	}
	if m.vals[0] > 0.5 {
		t.Fatalf("")
	}

	m, met = mockMetric(t, func(c *Config) {
		c.TimerGranularity = time.Millisecond
		c.ServiceName = "service"
	})
	met.MeasureSince("key", time.Now())
	require.Equal(t, "key", m.getKeys()[0][0])
	if m.vals[0] > 0.5 {
		t.Fatalf("value is greater than 0.1: %f", m.vals[0])
	}
}

func TestInsert(t *testing.T) {
	k := []string{"hi", "bob"}
	exp := []string{"hi", "there", "bob"}
	out := insert(1, "there", k)
	if !reflect.DeepEqual(exp, out) {
		t.Fatalf("bad insert %v %v", exp, out)
	}
}

func HasElem(s interface{}, elem interface{}) bool {
	arrV := reflect.ValueOf(s)

	if arrV.Kind() == reflect.Slice {
		for i := 0; i < arrV.Len(); i++ {
			if arrV.Index(i).Interface() == elem {
				return true
			}
		}
	}

	return false
}
