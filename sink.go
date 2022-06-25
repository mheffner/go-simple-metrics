package metrics

import (
	"fmt"
	"net/url"
)

type MetricType int

const (
	MetricTypeCounter MetricType = iota
	MetricTypeGauge
	MetricTypeTimer
	MetricTypeHistogram
	MetricTypeDistribution
)

// The MetricSink interface is used to transmit metrics information
// to an external system
type MetricSink interface {
	BuildMetricEmitter(mType MetricType, keys []string, labels []Label) MetricEmitter
}

type ShutdownSink interface {
	MetricSink

	// Shutdown the metric sink, flush metrics to storage, and cleanup resources.
	// Called immediately prior to application exit. Implementations must block
	// until metrics are flushed to storage.
	Shutdown()
}

// BlackholeSink is used to just blackhole messages
type BlackholeSink struct{}

func (s *BlackholeSink) BuildMetricEmitter(_ MetricType, _ []string, _ []Label) MetricEmitter {
	return func(val float64) {
	}
}

// FanoutSink is used to sink to fanout values to multiple sinks
type FanoutSink struct {
	Sinks []MetricSink
}

func (fh FanoutSink) BuildMetricEmitter(mType MetricType, keys []string, labels []Label) MetricEmitter {
	emitters := make([]MetricEmitter, len(fh.Sinks))
	for i := 0; i < len(fh.Sinks); i++ {
		emitters[i] = fh.Sinks[i].BuildMetricEmitter(mType, keys, labels)
	}

	return func(val float64) {
		for i := 0; i < len(fh.Sinks); i++ {
			emitters[i](val)
		}
	}
}

func (fh FanoutSink) Shutdown() {
	for _, s := range fh.Sinks {
		if ss, ok := s.(ShutdownSink); ok {
			ss.Shutdown()
		}
	}
}

// sinkURLFactoryFunc is an generic interface around the *SinkFromURL() function provided
// by each sink type
type sinkURLFactoryFunc func(*url.URL) (MetricSink, error)

// sinkRegistry supports the generic NewMetricSink function by mapping URL
// schemes to metric sink factory functions
var sinkRegistry = map[string]sinkURLFactoryFunc{
	"statsite": NewStatsiteSinkFromURL,
	"inmem":    NewInmemSinkFromURL,
}

// NewMetricSinkFromURL allows a generic URL input to configure any of the
// supported sinks. The scheme of the URL identifies the type of the sink, the
// and query parameters are used to set options.
//
// "statsd://" - Initializes a StatsdSink. The host and port are passed through
// as the "addr" of the sink
//
// "statsite://" - Initializes a StatsiteSink. The host and port become the
// "addr" of the sink
//
// "inmem://" - Initializes an InmemSink. The host and port are ignored. The
// "interval" and "duration" query parameters must be specified with valid
// durations, see NewInmemSink for details.
func NewMetricSinkFromURL(urlStr string) (MetricSink, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	sinkURLFactoryFunc := sinkRegistry[u.Scheme]
	if sinkURLFactoryFunc == nil {
		return nil, fmt.Errorf(
			"cannot create metric sink, unrecognized sink name: %q", u.Scheme)
	}

	return sinkURLFactoryFunc(u)
}
