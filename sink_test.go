package metrics

import (
	"reflect"
	"strings"
	"sync"
	"testing"
)

type MockSink struct {
	lock sync.Mutex

	shutdown bool
	keys     [][]string
	vals     []float64
	labels   [][]Label
}

var _ MetricSink = &MockSink{}

func (m *MockSink) getKeys() [][]string {
	m.lock.Lock()
	defer m.lock.Unlock()

	return m.keys
}

func (m *MockSink) BuildMetricEmitter(mType MetricType, keys []string, labels []Label) MetricEmitter {
	return func(val float64) {
		m.lock.Lock()
		defer m.lock.Unlock()

		m.keys = append(m.keys, keys)
		m.vals = append(m.vals, val)
		m.labels = append(m.labels, labels)
	}
}

func (m *MockSink) Shutdown() {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.shutdown = true
}

func TestFanoutSink_Gauge_Labels(t *testing.T) {
	m1 := &MockSink{}
	m2 := &MockSink{}
	fh := &FanoutSink{Sinks: []MetricSink{m1, m2}}

	k := []string{"test"}
	v := float64(42.0)
	l := []Label{{"a", "b"}}
	fh.BuildMetricEmitter(MetricTypeGauge, k, l)(v)

	if !reflect.DeepEqual(m1.keys[0], k) {
		t.Fatalf("key not equal")
	}
	if !reflect.DeepEqual(m2.keys[0], k) {
		t.Fatalf("key not equal")
	}
	if !reflect.DeepEqual(m1.vals[0], v) {
		t.Fatalf("val not equal")
	}
	if !reflect.DeepEqual(m2.vals[0], v) {
		t.Fatalf("val not equal")
	}
	if !reflect.DeepEqual(m1.labels[0], l) {
		t.Fatalf("labels not equal")
	}
	if !reflect.DeepEqual(m2.labels[0], l) {
		t.Fatalf("labels not equal")
	}
}

func TestFanoutSink_Counter_Labels(t *testing.T) {
	m1 := &MockSink{}
	m2 := &MockSink{}
	fh := &FanoutSink{Sinks: []MetricSink{m1, m2}}

	k := []string{"test"}
	v := float64(42.0)
	l := []Label{{"a", "b"}}
	fh.BuildMetricEmitter(MetricTypeCounter, k, l)(v)

	if !reflect.DeepEqual(m1.keys[0], k) {
		t.Fatalf("key not equal")
	}
	if !reflect.DeepEqual(m2.keys[0], k) {
		t.Fatalf("key not equal")
	}
	if !reflect.DeepEqual(m1.vals[0], v) {
		t.Fatalf("val not equal")
	}
	if !reflect.DeepEqual(m2.vals[0], v) {
		t.Fatalf("val not equal")
	}
	if !reflect.DeepEqual(m1.labels[0], l) {
		t.Fatalf("labels not equal")
	}
	if !reflect.DeepEqual(m2.labels[0], l) {
		t.Fatalf("labels not equal")
	}
}

func TestFanoutSink_Sample_Labels(t *testing.T) {
	m1 := &MockSink{}
	m2 := &MockSink{}
	fh := &FanoutSink{Sinks: []MetricSink{m1, m2}}

	k := []string{"test"}
	v := float64(42.0)
	l := []Label{{"a", "b"}}
	fh.BuildMetricEmitter(MetricTypeHistogram, k, l)(v)

	if !reflect.DeepEqual(m1.keys[0], k) {
		t.Fatalf("key not equal")
	}
	if !reflect.DeepEqual(m2.keys[0], k) {
		t.Fatalf("key not equal")
	}
	if !reflect.DeepEqual(m1.vals[0], v) {
		t.Fatalf("val not equal")
	}
	if !reflect.DeepEqual(m2.vals[0], v) {
		t.Fatalf("val not equal")
	}
	if !reflect.DeepEqual(m1.labels[0], l) {
		t.Fatalf("labels not equal")
	}
	if !reflect.DeepEqual(m2.labels[0], l) {
		t.Fatalf("labels not equal")
	}
}

func TestNewMetricSinkFromURL(t *testing.T) {
	for _, tc := range []struct {
		desc      string
		input     string
		expect    reflect.Type
		expectErr string
	}{
		{
			desc:   "statsite scheme yields a StatsiteSink",
			input:  "statsite://someserver:123",
			expect: reflect.TypeOf(&StatsiteSink{}),
		},
		{
			desc:   "inmem scheme yields an InmemSink",
			input:  "inmem://?interval=30s&retain=30s",
			expect: reflect.TypeOf(&InmemSink{}),
		},
		{
			desc:      "unknown scheme yields an error",
			input:     "notasink://whatever",
			expectErr: "unrecognized sink name: \"notasink\"",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			ms, err := NewMetricSinkFromURL(tc.input)
			if tc.expectErr != "" {
				if !strings.Contains(err.Error(), tc.expectErr) {
					t.Fatalf("expected err: %q to contain: %q", err, tc.expectErr)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected err: %s", err)
				}
				got := reflect.TypeOf(ms)
				if got != tc.expect {
					t.Fatalf("expected return type to be %v, got: %v", tc.expect, got)
				}
			}
		})
	}
}
