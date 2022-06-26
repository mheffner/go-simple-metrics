package metrics

import (
	"io/ioutil"
	"log"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	conf := defaultConfig()
	if conf.ServiceName != "" {
		t.Fatalf("Bad name")
	}
	if conf.HostName == "" {
		t.Fatalf("missing hostname")
	}
	if !conf.EnableHostnameLabel || !conf.EnableRuntimeMetrics {
		t.Fatalf("expect true")
	}
	if conf.EnableTypePrefix {
		t.Fatalf("expect false")
	}
	if conf.TimerGranularity != time.Millisecond {
		t.Fatalf("bad granularity")
	}
	if conf.ProfileInterval != time.Second {
		t.Fatalf("bad interval")
	}
}

func Test_GlobalMetrics_Labels(t *testing.T) {
	labels := []Label{{"a", "b"}}
	var tests = []struct {
		desc   string
		key    string
		val    float64
		fn     func(string, float64, ...Label)
		labels []Label
	}{
		{"SetGauge", "test", 42, SetGauge[float64], labels},
		{"Incr", "test", 42, Incr[float64], labels},
		{"Sample", "test", 42, Sample[float64], labels},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			s := &MockSink{}
			globalMetrics.Store(&Metrics{cfg: Config{FilterDefault: true}, sink: s})
			tt.fn(tt.key, tt.val, tt.labels...)
			if got, want := s.keys[0], tt.key; !reflect.DeepEqual(got, []string{want}) {
				t.Fatalf("got key %s want %s", got, want)
			}
			if got, want := s.vals[0], tt.val; !reflect.DeepEqual(got, want) {
				t.Fatalf("got val %v want %v", got, want)
			}
			if got, want := s.labels[0], tt.labels; !reflect.DeepEqual(got, want) {
				t.Fatalf("got val %s want %s", got, want)
			}
		})
	}
}

func Test_GlobalMetrics_Memomized(t *testing.T) {
	labels := []Label{{"a", "b"}}

	s := &MockSink{}
	globalMetrics.Store(&Metrics{cfg: Config{FilterDefault: true, TimerGranularity: time.Millisecond}, sink: s})

	g := NewGauge("gkey", labels...)
	g.Set(4)
	g.Set(5)

	c := NewCounter("ckey", labels...)
	c.Incr(10)
	c.Incr(20)

	tm := NewTimer("tkey", labels...)
	tm.MeasureSince(time.Now().Add(time.Millisecond * -2))

	h := NewHistogram("hkey", labels...)
	h.Sample(40)
	h.Sample(50)

	d := NewDistribution("dkey", labels...)
	d.Observe(100)
	d.Observe(110)

	require.Equal(t, "gkey", s.keys[0][0])
	require.Equal(t, "gkey", s.keys[1][0])

	require.Equal(t, float64(4), s.vals[0])
	require.Equal(t, float64(5), s.vals[1])

	require.Equal(t, labels, s.labels[0])
	require.Equal(t, labels, s.labels[1])

	require.Equal(t, "ckey", s.keys[2][0])
	require.Equal(t, "ckey", s.keys[3][0])

	require.Equal(t, float64(10), s.vals[2])
	require.Equal(t, float64(20), s.vals[3])

	require.Equal(t, labels, s.labels[2])
	require.Equal(t, labels, s.labels[3])

	require.Equal(t, "tkey", s.keys[4][0])

	require.Greater(t, s.vals[4], 1.0)
	require.Less(t, s.vals[4], 4.0)

	require.Equal(t, labels, s.labels[4])

	require.Equal(t, "hkey", s.keys[5][0])
	require.Equal(t, "hkey", s.keys[6][0])

	require.Equal(t, float64(40), s.vals[5])
	require.Equal(t, float64(50), s.vals[6])

	require.Equal(t, labels, s.labels[5])
	require.Equal(t, labels, s.labels[6])

	require.Equal(t, "dkey", s.keys[7][0])
	require.Equal(t, "dkey", s.keys[8][0])

	require.Equal(t, float64(100), s.vals[7])
	require.Equal(t, float64(110), s.vals[8])

	require.Equal(t, labels, s.labels[7])
	require.Equal(t, labels, s.labels[8])
}

func Test_GlobalMetrics_DefaultLabels(t *testing.T) {
	config := Config{
		HostName:            "host1",
		ServiceName:         "redis",
		EnableHostnameLabel: true,
		EnableServiceLabel:  true,
		FilterDefault:       true,
	}
	labels := []Label{
		{"host", config.HostName},
		{"service", config.ServiceName},
	}
	var tests = []struct {
		desc   string
		key    string
		val    float64
		fn     func(string, float64, ...Label)
		labels []Label
	}{
		{"SetGauge", "test", 42, SetGauge[float64], labels},
		{"Incr", "test", 42, Incr[float64], labels},
		{"Sample", "test", 42, Sample[float64], labels},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			s := &MockSink{}
			globalMetrics.Store(&Metrics{cfg: config, sink: s})
			tt.fn(tt.key, tt.val)
			if got, want := s.keys[0], tt.key; !reflect.DeepEqual(got, []string{want}) {
				t.Fatalf("got key %s want %s", got, want)
			}
			if got, want := s.vals[0], tt.val; !reflect.DeepEqual(got, want) {
				t.Fatalf("got val %v want %v", got, want)
			}
			if got, want := s.labels[0], tt.labels; !reflect.DeepEqual(got, want) {
				t.Fatalf("got val %s want %s", got, want)
			}
		})
	}
}

func Test_GlobalMetrics_MeasureSince(t *testing.T) {
	s := &MockSink{}
	m := &Metrics{sink: s, cfg: Config{TimerGranularity: time.Millisecond, FilterDefault: true}}
	globalMetrics.Store(m)

	k := "test"
	now := time.Now()
	MeasureSince(k, now)

	if !reflect.DeepEqual(s.keys[0], []string{k}) {
		t.Fatalf("key not equal")
	}
	if s.vals[0] > 0.1 {
		t.Fatalf("val too large %v", s.vals[0])
	}

	labels := []Label{{"a", "b"}}
	MeasureSince(k, now, labels...)
	if got, want := s.keys[1], k; !reflect.DeepEqual(got, []string{want}) {
		t.Fatalf("got key %s want %s", got, want)
	}
	if s.vals[1] > 0.1 {
		t.Fatalf("val too large %v", s.vals[0])
	}
	if got, want := s.labels[1], labels; !reflect.DeepEqual(got, want) {
		t.Fatalf("got val %s want %s", got, want)
	}
}

func Test_GlobalMetrics_Shutdown(t *testing.T) {
	s := &MockSink{}
	m := &Metrics{sink: s}
	globalMetrics.Store(m)

	Shutdown()

	loaded := globalMetrics.Load()
	metrics, ok := loaded.(*Metrics)
	if !ok {
		t.Fatalf("Expected globalMetrics to contain a Metrics pointer, but found: %v", loaded)
	}
	if metrics == m {
		t.Errorf("Calling shutdown should have replaced the Metrics struct stored in globalMetrics")
	}
	if !s.shutdown {
		t.Errorf("Expected Shutdown to have been called on MockSink")
	}
}

// Benchmark_GlobalMetrics_Direct/direct-8         	 5000000	       278 ns/op
// Benchmark_GlobalMetrics_Direct/atomic.Value-8   	 5000000	       235 ns/op
func Benchmark_GlobalMetrics_Direct(b *testing.B) {
	log.SetOutput(ioutil.Discard)
	s := &MockSink{}
	m := &Metrics{sink: s}
	var v atomic.Value
	v.Store(m)
	k := "test"
	b.Run("direct", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			m.Incr(k, 1)
		}
	})
	b.Run("atomic.Value", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			v.Load().(*Metrics).Incr(k, 1)
		}
	})
	// do something with m so that the compiler does not optimize this away
	b.Logf("%v", m.cfg.TimerGranularity)
}
