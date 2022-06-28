package datadog

import (
	"fmt"
	"strings"

	"github.com/DataDog/datadog-go/v5/statsd"
	metrics "github.com/mheffner/go-simple-metrics"
)

const defaultRate = 1.0

// DogStatsdSink provides a MetricSink that can be used
// with a dogstatsd server. It utilizes the Dogstatsd client at github.com/DataDog/datadog-go/statsd
type DogStatsdSink struct {
	client            *statsd.Client
	hostName          string
	propagateHostname bool
}

// NewDogStatsdSink is used to create a new DogStatsdSink with sane defaults
func NewDogStatsdSink(addr string, hostName string) (*DogStatsdSink, error) {
	client, err := statsd.New(addr)
	if err != nil {
		return nil, err
	}
	sink := &DogStatsdSink{
		client:            client,
		hostName:          hostName,
		propagateHostname: false,
	}
	return sink, nil
}

func (s *DogStatsdSink) flattenKey(parts []string) string {
	joined := strings.Join(parts, ".")
	return strings.Map(sanitize, joined)
}

func sanitize(r rune) rune {
	switch r {
	case ':':
		fallthrough
	case ' ':
		return '_'
	default:
		return r
	}
}

func (s *DogStatsdSink) parseKey(key []string) ([]string, []metrics.Label) {
	// Since DogStatsd supports dimensionality via tags on metric keys, this sink's approach is to splice the hostname out of the key in favor of a `host` tag
	// The `host` tag is either forced here, or set downstream by the DogStatsd server

	var labels []metrics.Label
	hostName := s.hostName

	// Splice the hostname out of the key
	for i, el := range key {
		if el == hostName {
			key = append(key[:i], key[i+1:]...)
			break
		}
	}

	if s.propagateHostname {
		labels = append(labels, metrics.Label{"host", hostName})
	}
	return key, labels
}

// Implementation of methods in the MetricSink interface

func (s *DogStatsdSink) BuildMetricEmitter(mType metrics.MetricType, keys []string, labels []metrics.Label) metrics.MetricEmitter {
	flatKey, tags := s.getFlatkeyAndCombinedLabels(keys, labels)

	return func(val float64) {
		switch mType {
		case metrics.MetricTypeCounter:
			_ = s.client.Count(flatKey, int64(val), tags, defaultRate)
		case metrics.MetricTypeGauge:
			_ = s.client.Gauge(flatKey, val, tags, defaultRate)
		case metrics.MetricTypeTimer:
			_ = s.client.TimeInMilliseconds(flatKey, val, tags, defaultRate)
		case metrics.MetricTypeDistribution:
			_ = s.client.Distribution(flatKey, val, tags, defaultRate)
		case metrics.MetricTypeHistogram:
			_ = s.client.Histogram(flatKey, val, tags, defaultRate)
		}
	}
}

// Shutdown disables further metric collection, blocks to flush data, and tears down the sink.
func (s *DogStatsdSink) Shutdown() {
	_ = s.client.Close()
}

func (s *DogStatsdSink) getFlatkeyAndCombinedLabels(key []string, labels []metrics.Label) (string, []string) {
	key, parsedLabels := s.parseKey(key)
	flatKey := s.flattenKey(key)
	labels = append(labels, parsedLabels...)

	var tags []string
	for _, label := range labels {
		label.Name = strings.Map(sanitize, label.Name)
		label.Value = strings.Map(sanitize, label.Value)
		if label.Value != "" {
			tags = append(tags, fmt.Sprintf("%s:%s", label.Name, label.Value))
		} else {
			tags = append(tags, label.Name)
		}
	}

	return flatKey, tags
}
