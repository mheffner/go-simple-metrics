go-simple-metrics
==========

This library provides an opinionated `metrics` package which can be used to instrument code,
expose application metrics, and profile runtime performance in a flexible manner. It is derived
from the fantastic and widely used version of [@armon](https://github.com/armon)'s
[go-metrics](https://github.com/armon/go-metrics).

See the section on [Design Philosophies](#design-philosophy) below to understand the guiding design
principles for this library.

**NOTE: This is an early version of this library so caution should be used before using it in
production. There are no guarantees that breaking changes may not occur in early updates.**

## Examples

Here is an example of using the package:

```go
func SlowMethod() {
    // Profiling the runtime of a method
    defer metrics.MeasureSince("SlowMethod", time.Now(), metrics.L("method_name", "slow"))

    // do something slow
}

// Configure a statsite sink as the global metrics sink
sink, _ := metrics.NewStatsiteSink("statsite:8125")
metrics.NewGlobal(metrics.DefaultConfig("service-name"), sink)

// count an occurrence
metrics.Incr("method-counter", 1)

// sample an observed value, add some labels
metrics.Sample("queue-latency", 43.5, metrics.L("name", "msgs"), metrics.L("type", "sqs"))
```

## Sinks

The `metrics` package makes use of a `MetricSink` interface to support delivery
to any type of backend. Currently, the following sinks are provided:

* StatsiteSink : Sinks to a [statsite](https://github.com/armon/statsite/) instance (TCP)
* Datadog: Sinks to a DataDog dogstatsd instance.
* InmemSink : Provides in-memory aggregation, can be used to export stats
* FanoutSink : Sinks to multiple sinks. Enables writing to multiple statsite instances for example.
* BlackholeSink : Sinks to nowhere

In addition to the sinks, the `InmemSignal` can be used to catch a signal,
and dump a formatted output of recent metrics. For example, when a process gets
a SIGUSR1, it can dump to stderr recent performance metrics for debugging.

## Instrumentation Methods

### Gauge: `SetGauge()`

A gauge represents the current value of some observed property, for example the current number of items
pending in a queue. While multiple observations may be recorded over an interval, typically a sink will
keep only the last recorded value. 

### Counter: `Incr()`

Counters represent occurrences of an event over time and are typically graphed as a rate. For example, the total number of
messages read from a queue.

### Histogram: `Sample()`

Histograms track aggregations and quantile values over multiple observed values. For example, the
number of bytes in each message received from a queue, recorded per-message, may be tracked as a
histogram. Aggregation sinks may typically calculate quantile metrics like 95th percentile.

### Timer: `MeasureSince()`

Timers are a special purpose histogram for tracking time metrics, like latency. The `MeasureSince` 
method makes it easy to record the time spent since some the start of an event. When invoked with a `defer`
as the example above shows, it makes it easy to record the time of a code block.

## Memoized Metrics

In most scenarios the one-liner methods above should be enough to instrument any block of code quickly
and easily. However, in tight-loop hot paths there can be some overhead to construct the metric names and
labels and sanitize the names for the resulting sink. In these cases where multiple observations from
the same metric name and labels are expected, you can use a memoized version of the metric and emit
new observations as needed with lower overhead.

For example, using the memoized counter in a message processing loop:
```go

func pollMessages(messages chan<- string) {
	c := metrics.NewCounter("msg-count", metrics.L("queue", "msgs"))
	for msg := range messages {
		c.Incr(1)
		fmt.Println("received ", msg)
	}
}
```

There are similar methods for all metric types: `NewGauge`, `NewHistogram`, `NewTimer`.

## Persisted and Aggregated Metrics

Finally, there are two special metric types, `PersistedGauge` and `AggregatedCounter`, that can be
used for even further performance improvements. Both of these elide updates to the sink across
individual metric observations, instead publishing aggregated updates to the sink once per publishing
interval. This can help when you want to further reduce the overhead of a `1:1` ratio of observation
and sink update.

*CAUTION: Because these metrics batch updates, there is a chance for some data loss
in the case of an unhandled crash of an application*.

### Persisted Gauges

Persisted gauges maintain an observed value internally and publish the last seen value on the
publishing interval. The behavior is the same as using the `SetGauge` method, however instead of
each call posting a value to the sink and the sink (e.g. agent) keeping the last value, this happens
before the sink.

For example, if you wanted to track the active number of open connections on a busy server:
```go
type watcher struct {
	gauge metrics.PersistentGauge
}

func (w *watcher) OnStateChange(conn net.Conn, state http.ConnState) {
	switch state {
	case http.StateNew:
		w.gauge.Incr(1)
	case http.StateHijacked, http.StateClosed:
		w.gauge.Decr(1)
	}
}

w := &watcher{
	gauge: metrics.NewPersistentGauge("server.active-conns", metrics.L("port", "443")),
}

s := &http.Server{
	ConnState: w.OnStateChange
}

// run server

// when finished, you must stop the gauge
w.gauge.Stop()
```

### Aggregated Counters

An aggregated counter can be useful for extremely hot-path metric instrumentation. It aggregates the
total increment delta internally and publishes the current delta on each report interval. Unlike the
Persisted Gauge, an Aggregated Counter will reset its value to zero on each reporting interval.

Here's the example from earlier using an aggregated counter instead:
```go

func pollMessages(messages chan<- string) {
	c := metrics.NewAggregatedCounter("msg-count", metrics.L("queue", "msgs"))
	for msg := range messages {
		c.Incr(1)
		fmt.Println("received ", msg)
	}
	// must stop the counter when finished to stop reporting
	c.Stop()
}
```

## Design Philosophy

This library draws on a few philosophies which we believe are required for modern metrics
instrumentation and observability.

### Metrics are multidimensional

Call them tags or labels, but multidimensional metrics are the standard and observability systems
must treat them as first-class citizens. Similarly, they should be the default for instrumentation
libraries, allowing the ability to configure them globally (infra dimensions) as well as per-metric
based on application context.

### Instrumentation should be succinct

Instrumenting code should not distract from the content of the code itself. It should be clear
to a reviewer where and how the code has been instrumented, but readability of the application
must be maintained.

### Quick to instrument

Instrumenting foreign code blocks is particularly useful when dropping into a codebase for the first time,
possibly under pressure of an ongoing incident. You must be able to instrument
quickly without having to understand larger structures of the code or refactor major portions to
inject the right instrumentation. Instrumentation should only require a single line of code in most
circumstances.

### Low impact

Generally developers should not have to worry whether instrumenting a block of code may negatively
impact the performance of their application in a severe way. Instrumentation must be lightweight enough
that in most scenarios there is little impact to introducing it to the code. In extreme cases where
that may not be possible there should be graduated levels of instrumentation methods available to
use in those hot paths, even if they may conflict partially with the previous two design points. 
