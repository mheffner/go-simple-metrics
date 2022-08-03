//go:build go1.9
// +build go1.9

package prometheus

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	metrics "github.com/mheffner/go-simple-metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

var (
	// DefaultPrometheusOpts is the default set of options used when creating a
	// PrometheusSink.
	DefaultPrometheusOpts = PrometheusOpts{
		Expiration: 60 * time.Second,
		Name:       "default_prometheus_sink",
	}
)

// PrometheusOpts is used to configure the Prometheus Sink
type PrometheusOpts struct {
	// Expiration is the duration a metric is valid for, after which it will be
	// untracked. If the value is zero, a metric is never expired.
	Expiration time.Duration
	Registerer prometheus.Registerer

	// Gauges, Summaries, and Counters allow us to pre-declare metrics by giving
	// their Name, Help, and ConstLabels to the PrometheusSink when it is created.
	// Metrics declared in this way will be initialized at zero and will not be
	// deleted or altered when their expiry is reached.
	//
	// Ex: PrometheusOpts{
	//     Expiration: 10 * time.Second,
	//     Gauges: []GaugeDefinition{
	//         {
	//           Name: []string{ "application", "component", "measurement"},
	//           Help: "application_component_measurement provides an example of how to declare static metrics",
	//           ConstLabels: []metrics.Label{ { Name: "my_label", Value: "does_not_change" }, },
	//         },
	//     },
	// }
	GaugeDefinitions   []GaugeDefinition
	SummaryDefinitions []SummaryDefinition
	CounterDefinitions []CounterDefinition
	Name               string
}

type PrometheusSink struct {
	// If these will ever be copied, they should be converted to *sync.Map values and initialized appropriately
	gauges     sync.Map
	summaries  sync.Map
	counters   sync.Map
	expiration time.Duration
	help       map[string]string
	name       string
}

// expirableMetric is a metric that may be expired at any point in time if it is not updated regularly.
// The updatedAt time tracks the last time it was updated and deleted is marked when it is removed, so
// that callers updating the metric know if they must create a new entry.
//
// When updating a sample value for the metric callers read-lock in-order to read the `deleted` status. The
// collection sweep will write-lock to read updatedAt and, if it is collected, mark deleted=true.
type expirableMetric struct {
	// update with atomic
	updatedAtNano int64
	// canDelete is set if the metric is created during runtime so we know it's ephemeral and can delete it on expiry.
	canDelete bool
	mut       sync.RWMutex
	deleted   bool
}

func (c *expirableMetric) markUpdated() {
	atomic.SwapInt64(&c.updatedAtNano, time.Now().UnixNano())
}

// GaugeDefinition can be provided to PrometheusOpts to declare a constant gauge that is not deleted on expiry.
type GaugeDefinition struct {
	Name        string
	ConstLabels []metrics.Label
	Help        string
}

type gauge struct {
	prometheus.Gauge
	expirableMetric
}

// SummaryDefinition can be provided to PrometheusOpts to declare a constant summary that is not deleted on expiry.
type SummaryDefinition struct {
	Name        string
	ConstLabels []metrics.Label
	Help        string
}

type summary struct {
	prometheus.Summary
	expirableMetric
}

// CounterDefinition can be provided to PrometheusOpts to declare a constant counter that is not deleted on expiry.
type CounterDefinition struct {
	Name        string
	ConstLabels []metrics.Label
	Help        string
}

type counter struct {
	prometheus.Counter
	expirableMetric
}

// NewPrometheusSink creates a new PrometheusSink using the default options.
func NewPrometheusSink() (*PrometheusSink, error) {
	return NewPrometheusSinkFrom(DefaultPrometheusOpts)
}

// NewPrometheusSinkFrom creates a new PrometheusSink using the passed options.
func NewPrometheusSinkFrom(opts PrometheusOpts) (*PrometheusSink, error) {
	name := opts.Name
	if name == "" {
		name = "default_prometheus_sink"
	}
	sink := &PrometheusSink{
		gauges:     sync.Map{},
		summaries:  sync.Map{},
		counters:   sync.Map{},
		expiration: opts.Expiration,
		help:       make(map[string]string),
		name:       name,
	}

	initGauges(&sink.gauges, opts.GaugeDefinitions, sink.help)
	initSummaries(&sink.summaries, opts.SummaryDefinitions, sink.help)
	initCounters(&sink.counters, opts.CounterDefinitions, sink.help)

	reg := opts.Registerer
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	return sink, reg.Register(sink)
}

func (p *PrometheusSink) BuildMetricEmitter(mType metrics.MetricType, keys []string, labels []metrics.Label) metrics.MetricEmitter {
	key, hash := flattenKey(keys, labels)

	if mType == metrics.MetricTypeCounter {
		c := p.loadCounter(key, hash, labels)

		return func(val float64) {
			c.mut.RLock()
			if c.deleted {
				c.mut.RUnlock()
				c = p.newCounter(key, hash, labels)
				c.mut.RLock()
			}
			c.markUpdated()

			c.Add(val)
			c.mut.RUnlock()
		}
	}

	if mType == metrics.MetricTypeGauge {
		g := p.loadGauge(key, hash, labels)

		return func(val float64) {
			g.mut.RLock()
			if g.deleted {
				g.mut.RUnlock()
				g = p.newGauge(key, hash, labels)
				g.mut.RLock()
			}
			g.markUpdated()

			g.Set(val)
			g.mut.RUnlock()
		}
	}

	// these are all handled by the summary type
	if mType == metrics.MetricTypeHistogram ||
		mType == metrics.MetricTypeTimer ||
		mType == metrics.MetricTypeDistribution {
		s := p.loadSummary(key, hash, labels)

		return func(val float64) {
			s.mut.RLock()
			if s.deleted {
				s.mut.RUnlock()
				s = p.newSummary(key, hash, labels)
				s.mut.RLock()
			}
			s.markUpdated()

			s.Observe(val)
			s.mut.RUnlock()
		}
	}

	// should never fall through here
	return func(val float64) {}
}

func (p *PrometheusSink) loadCounter(key string, hash string, labels []metrics.Label) *counter {
	pc, ok := p.counters.Load(hash)
	if ok {
		return pc.(*counter)
	}

	return p.newCounter(key, hash, labels)
}

func (p *PrometheusSink) newCounter(key string, hash string, labels []metrics.Label) *counter {
	help := key
	existingHelp, ok := p.help[fmt.Sprintf("counter.%s", key)]
	if ok {
		help = existingHelp
	}
	c := prometheus.NewCounter(prometheus.CounterOpts{
		Name:        key,
		Help:        help,
		ConstLabels: prometheusLabels(labels),
	})
	pc := &counter{
		Counter: c,
		expirableMetric: expirableMetric{
			updatedAtNano: time.Now().UnixNano(),
			canDelete:     true,
		},
	}
	ret, _ := p.counters.LoadOrStore(hash, pc)

	return ret.(*counter)
}

func (p *PrometheusSink) loadGauge(key string, hash string, labels []metrics.Label) *gauge {
	pg, ok := p.gauges.Load(hash)
	if ok {
		return pg.(*gauge)
	}

	return p.newGauge(key, hash, labels)
}

func (p *PrometheusSink) newGauge(key string, hash string, labels []metrics.Label) *gauge {
	help := key
	existingHelp, ok := p.help[fmt.Sprintf("gauge.%s", key)]
	if ok {
		help = existingHelp
	}
	g := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        key,
		Help:        help,
		ConstLabels: prometheusLabels(labels),
	})

	pg := &gauge{
		Gauge: g,
		expirableMetric: expirableMetric{
			updatedAtNano: time.Now().UnixNano(),
			canDelete:     true,
		},
	}
	ret, _ := p.gauges.LoadOrStore(hash, pg)

	return ret.(*gauge)
}

func (p *PrometheusSink) loadSummary(key string, hash string, labels []metrics.Label) *summary {
	pg, ok := p.summaries.Load(hash)
	if ok {
		return pg.(*summary)
	}

	return p.newSummary(key, hash, labels)
}

func (p *PrometheusSink) newSummary(key string, hash string, labels []metrics.Label) *summary {
	help := key
	existingHelp, ok := p.help[fmt.Sprintf("summary.%s", key)]
	if ok {
		help = existingHelp
	}
	s := prometheus.NewSummary(prometheus.SummaryOpts{
		Name:        key,
		Help:        help,
		MaxAge:      10 * time.Second,
		ConstLabels: prometheusLabels(labels),
		Objectives:  map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})
	ps := &summary{
		Summary: s,
		expirableMetric: expirableMetric{
			updatedAtNano: time.Now().UnixNano(),
			canDelete:     true,
		},
	}
	ret, _ := p.summaries.LoadOrStore(hash, ps)

	return ret.(*summary)
}

// Describe sends a Collector.Describe value from the descriptor created around PrometheusSink.Name
// Note that we cannot describe all the metrics (gauges, counters, summaries) in the sink as
// metrics can be added at any point during the lifecycle of the sink, which does not respect
// the idempotency aspect of the Collector.Describe() interface
func (p *PrometheusSink) Describe(c chan<- *prometheus.Desc) {
	// dummy value to be able to register and unregister "empty" sinks
	// Note this is not actually retained in the PrometheusSink so this has no side effects
	// on the caller's sink. So it shouldn't show up to any of its consumers.
	prometheus.NewGauge(prometheus.GaugeOpts{Name: p.name, Help: p.name}).Describe(c)
}

// Collect meets the collection interface and allows us to enforce our expiration
// logic to clean up ephemeral metrics if their value haven't been set for a
// duration exceeding our allowed expiration time.
func (p *PrometheusSink) Collect(c chan<- prometheus.Metric) {
	p.collectAtTime(c, time.Now())
}

// collectAtTime allows internal testing of the expiry based logic here without
// mocking clocks or making tests timing sensitive.
func (p *PrometheusSink) collectAtTime(c chan<- prometheus.Metric, t time.Time) {
	expire := p.expiration != 0
	p.gauges.Range(func(k, v interface{}) bool {
		if v == nil {
			return true
		}
		g := v.(*gauge)
		g.mut.Lock()

		lastUpdate := time.Unix(0, g.updatedAtNano)
		if expire && lastUpdate.Add(p.expiration).Before(t) {
			if g.canDelete {
				g.deleted = true
				p.gauges.Delete(k)
				g.mut.Unlock()
				return true
			}
		}
		g.mut.Unlock()
		g.Collect(c)
		return true
	})
	p.summaries.Range(func(k, v interface{}) bool {
		if v == nil {
			return true
		}
		s := v.(*summary)
		s.mut.Lock()

		lastUpdate := time.Unix(0, s.updatedAtNano)
		if expire && lastUpdate.Add(p.expiration).Before(t) {
			if s.canDelete {
				s.deleted = true
				p.summaries.Delete(k)
				s.mut.Unlock()
				return true
			}
		}
		s.mut.Unlock()
		s.Collect(c)
		return true
	})
	p.counters.Range(func(k, v interface{}) bool {
		if v == nil {
			return true
		}
		count := v.(*counter)
		count.mut.Lock()

		lastUpdate := time.Unix(0, count.updatedAtNano)
		if expire && lastUpdate.Add(p.expiration).Before(t) {
			if count.canDelete {
				count.deleted = true
				p.counters.Delete(k)
				count.mut.Unlock()
				return true
			}
		}
		count.mut.Unlock()
		count.Collect(c)
		return true
	})
}

func initGauges(m *sync.Map, gauges []GaugeDefinition, help map[string]string) {
	for _, g := range gauges {
		key, hash := flattenKey([]string{g.Name}, g.ConstLabels)
		help[fmt.Sprintf("gauge.%s", key)] = g.Help
		pG := prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        key,
			Help:        g.Help,
			ConstLabels: prometheusLabels(g.ConstLabels),
		})
		m.Store(hash, &gauge{Gauge: pG})
	}
	return
}

func initSummaries(m *sync.Map, summaries []SummaryDefinition, help map[string]string) {
	for _, s := range summaries {
		key, hash := flattenKey([]string{s.Name}, s.ConstLabels)
		help[fmt.Sprintf("summary.%s", key)] = s.Help
		pS := prometheus.NewSummary(prometheus.SummaryOpts{
			Name:        key,
			Help:        s.Help,
			MaxAge:      10 * time.Second,
			ConstLabels: prometheusLabels(s.ConstLabels),
			Objectives:  map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		})
		m.Store(hash, &summary{Summary: pS})
	}
	return
}

func initCounters(m *sync.Map, counters []CounterDefinition, help map[string]string) {
	for _, c := range counters {
		key, hash := flattenKey([]string{c.Name}, c.ConstLabels)
		help[fmt.Sprintf("counter.%s", key)] = c.Help
		pC := prometheus.NewCounter(prometheus.CounterOpts{
			Name:        key,
			Help:        c.Help,
			ConstLabels: prometheusLabels(c.ConstLabels),
		})
		m.Store(hash, &counter{Counter: pC})
	}
	return
}

var forbiddenChars = regexp.MustCompile("[ .=\\-/]")

func flattenKey(parts []string, labels []metrics.Label) (string, string) {
	key := strings.Join(parts, "_")
	key = forbiddenChars.ReplaceAllString(key, "_")

	hash := key
	for _, label := range labels {
		hash += fmt.Sprintf(";%s=%s", label.Name, label.Value)
	}

	return key, hash
}

func prometheusLabels(labels []metrics.Label) prometheus.Labels {
	l := make(prometheus.Labels)
	for _, label := range labels {
		l[label.Name] = label.Value
	}
	return l
}

// PrometheusPushSink wraps a normal prometheus sink and provides an address and facilities to export it to an address
// on an interval.
type PrometheusPushSink struct {
	*PrometheusSink
	pusher       *push.Pusher
	address      string
	pushInterval time.Duration
	stopChan     chan struct{}
}

// NewPrometheusPushSink creates a PrometheusPushSink by taking an address, interval, and destination name.
func NewPrometheusPushSink(address string, pushInterval time.Duration, name string) (*PrometheusPushSink, error) {
	promSink := &PrometheusSink{
		gauges:     sync.Map{},
		summaries:  sync.Map{},
		counters:   sync.Map{},
		expiration: 60 * time.Second,
		name:       "default_prometheus_sink",
	}

	pusher := push.New(address, name).Collector(promSink)

	sink := &PrometheusPushSink{
		promSink,
		pusher,
		address,
		pushInterval,
		make(chan struct{}),
	}

	sink.flushMetrics()
	return sink, nil
}

func (s *PrometheusPushSink) flushMetrics() {
	ticker := time.NewTicker(s.pushInterval)

	go func() {
		for {
			select {
			case <-ticker.C:
				err := s.pusher.Push()
				if err != nil {
					log.Printf("[ERR] Error pushing to Prometheus! Err: %s", err)
				}
			case <-s.stopChan:
				ticker.Stop()
				return
			}
		}
	}()
}

// Shutdown tears down the PrometheusPushSink, and blocks while flushing metrics to the backend.
func (s *PrometheusPushSink) Shutdown() {
	close(s.stopChan)
	// Closing the channel only stops the running goroutine that pushes metrics.
	// To minimize the chance of data loss pusher.Push is called one last time.
	_ = s.pusher.Push()
}
