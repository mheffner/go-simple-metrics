package metrics

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"time"

	iradix "github.com/hashicorp/go-immutable-radix"
)

// Config is used to configure metrics settings
type Config struct {
	ServiceName          string        // Name of service, added to labels if EnableServiceLabel is set
	HostName             string        // Hostname to use. If not provided and EnableHostname, it will be os.Hostname
	EnableHostnameLabel  bool          // Enable adding hostname to labels
	EnableServiceLabel   bool          // Enable adding service to labels
	EnableServicePrefix  bool          // Enable adding service to the metrics key
	EnableRuntimeMetrics bool          // Enables profiling of runtime metrics (GC, Goroutines, Memory)
	EnableTypePrefix     bool          // Prefixes key with a type ("counter", "gauge", "timer")
	TimerGranularity     time.Duration // Granularity of timers.
	ProfileInterval      time.Duration // Interval to profile runtime metrics
	PersistentInterval   time.Duration // Interval to publish persisted metrics

	BaseLabels []Label // Default labels applied to all measurements

	AllowedPrefixes []string // A list of metric prefixes to allow, with '.' as the separator
	BlockedPrefixes []string // A list of metric prefixes to block, with '.' as the separator
	AllowedLabels   []string // A list of metric labels to allow, with '.' as the separator
	BlockedLabels   []string // A list of metric labels to block, with '.' as the separator
	FilterDefault   bool     // Whether to allow metrics by default
}

type Label struct {
	Name  string
	Value string
}

// Metrics represents an instance of a metrics sink that can
// be used to emit
type Metrics struct {
	cfg           Config
	sink          MetricSink
	filter        *iradix.Tree
	allowedLabels map[string]bool
	blockedLabels map[string]bool

	runtimeMetricsCancel context.CancelFunc
	runtimeWaitG         sync.WaitGroup

	persistedGauges        sync.Map
	aggregatedCounters     sync.Map
	persistedPublishCancel context.CancelFunc
	persistedPublishWaitG  sync.WaitGroup
}

// Shared global metrics instance
var globalMetrics atomic.Value // *Metrics

func init() {
	// Initialize to a blackhole sink to avoid errors
	globalMetrics.Store(&Metrics{sink: &BlackholeSink{}})
}

// Default returns the shared global metrics instance.
func Default() *Metrics {
	return currMetrics()
}

// default configuration
func defaultConfig() *Config {
	c := &Config{
		ServiceName:          "",
		HostName:             "",
		EnableHostnameLabel:  true,             // Enable hostname label
		EnableRuntimeMetrics: true,             // Enable runtime profiling
		EnableTypePrefix:     false,            // Disable type prefix
		TimerGranularity:     time.Millisecond, // Timers are in milliseconds
		ProfileInterval:      time.Second,      // Poll runtime every second
		FilterDefault:        true,             // Don't filter metrics by default
		PersistentInterval:   time.Second,      // Publish persisted metrics every 1sec
	}

	// Try to get the hostname
	name, _ := os.Hostname()
	c.HostName = name
	return c
}

type ConfigOption func(cfg *Config)

// New is used to create a new instance of Metrics
func New(sink MetricSink, opts ...ConfigOption) (*Metrics, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	met := &Metrics{}
	met.cfg = *cfg
	met.sink = sink
	met.persistedGauges = sync.Map{}
	met.setFilterAndLabels(met.cfg.AllowedPrefixes, met.cfg.BlockedPrefixes, met.cfg.AllowedLabels, met.cfg.BlockedLabels)

	// Start the runtime collector
	if met.cfg.EnableRuntimeMetrics {
		ctx, cancel := context.WithCancel(context.Background())
		met.runtimeMetricsCancel = cancel
		met.runtimeWaitG = sync.WaitGroup{}
		met.runtimeWaitG.Add(1)

		go func() {
			// prevent any impact to app
			defer panicRecover()

			defer met.runtimeWaitG.Done()
			met.collectStats(ctx)
		}()
	}

	// Start the publishing of persisted metrics
	if met.cfg.PersistentInterval > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		met.persistedPublishCancel = cancel
		met.persistedPublishWaitG = sync.WaitGroup{}
		met.persistedPublishWaitG.Add(1)

		go func() {
			// prevent any impact to app
			defer panicRecover()

			defer met.persistedPublishWaitG.Done()
			met.pollPersistedMetrics(ctx)
		}()
	}

	return met, nil
}

// NewGlobal is the same as New, but it assigns the metrics object to be
// used globally as well as returning it.
func NewGlobal(sink MetricSink, opts ...ConfigOption) (*Metrics, error) {
	metrics, err := New(sink, opts...)
	if err == nil {
		globalMetrics.Store(metrics)
	}
	return metrics, err
}

// L is a shorthand for creating a label
func L(name string, value string) Label {
	return Label{Name: name, Value: value}
}

// StatValue defines the allowed values for the simple interface
type StatValue interface {
	int | int8 | int32 | int64 | float32 | float64
}

//
// Proxy all the methods to the globalMetrics instance
//

// SetGauge records the current observed value
func SetGauge[V StatValue](key string, val V, labels ...Label) {
	currMetrics().SetGauge(key, float64(val), labels...)
}

// Incr increments a counter
func Incr[V StatValue](key string, val V, labels ...Label) {
	currMetrics().Incr(key, float64(val), labels...)
}

// Sample records an observation in a histogram
func Sample[V StatValue](key string, val V, labels ...Label) {
	currMetrics().Sample(key, float64(val), labels...)
}

// MeasureSince records the time elapsed since an event, often as a histogram
func MeasureSince(key string, start time.Time, labels ...Label) {
	currMetrics().MeasureSince(key, start, labels...)
}

// Observe records an observation as part of a distribution
func Observe[V StatValue](key string, val V, labels ...Label) {
	currMetrics().Observe(key, float64(val), labels...)
}

//
// Memoized versions
//

// NewGauge creates a memoized gauge
func NewGauge(key string, labels ...Label) Gauge {
	return currMetrics().NewGauge(key, labels...)
}

// NewCounter creates a memoized counter
func NewCounter(key string, labels ...Label) Counter {
	return currMetrics().NewCounter(key, labels...)
}

// NewTimer creates a memoized timer
func NewTimer(key string, labels ...Label) Timer {
	return currMetrics().NewTimer(key, labels...)
}

// NewHistogram creates a memoized histogram
func NewHistogram(key string, labels ...Label) Histogram {
	return currMetrics().NewHistogram(key, labels...)
}

// NewDistribution creates a memoized histogram
func NewDistribution(key string, labels ...Label) Distribution {
	return currMetrics().NewDistribution(key, labels...)
}

//
// persistent versions
//

func NewPersistentGauge(key string, labels ...Label) PersistentGauge {
	return currMetrics().NewPersistentGauge(key, labels...)
}

func NewAggregatedCounter(key string, labels ...Label) AggregatedCounter {
	return currMetrics().NewAggregatedCounter(key, labels...)
}

// Shutdown disables metric collection, then blocks while attempting to flush metrics to storage.
// WARNING: Not all MetricSink backends support this functionality, and calling this will cause them to leak resources.
// This is intended for use immediately prior to application exit.
func Shutdown() {
	m := globalMetrics.Load().(*Metrics)
	// Swap whatever MetricSink is currently active with a BlackholeSink. Callers must not have a
	// reason to expect that calls to the library will successfully collect metrics after Shutdown
	// has been called.
	globalMetrics.Store(&Metrics{sink: &BlackholeSink{}})
	m.Shutdown()
}

func currMetrics() *Metrics {
	return globalMetrics.Load().(*Metrics)
}

func panicRecover() {
	if r := recover(); r != nil {
		// log?
	}
}
