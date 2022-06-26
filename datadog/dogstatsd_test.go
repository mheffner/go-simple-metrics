package datadog

import (
	"net"
	"testing"

	metrics "github.com/mheffner/go-simple-metrics"
)

var EmptyTags []metrics.Label

const (
	DogStatsdAddr    = "127.0.0.1:7254"
	HostnameEnabled  = true
	HostnameDisabled = false
	TestHostname     = "test_hostname"
)

func MockGetHostname() string {
	return TestHostname
}

func mockNewDogStatsdSink(addr string) *DogStatsdSink {
	dog, _ := NewDogStatsdSink(addr, MockGetHostname())
	return dog
}

func setupTestServerAndBuffer(t *testing.T) (*net.UDPConn, []byte) {
	udpAddr, err := net.ResolveUDPAddr("udp", DogStatsdAddr)
	if err != nil {
		t.Fatal(err)
	}
	server, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		t.Fatal(err)
	}
	return server, make([]byte, 1024)
}

func TestTaggableMetrics(t *testing.T) {
	server, buf := setupTestServerAndBuffer(t)
	defer server.Close()

	dog := mockNewDogStatsdSink(DogStatsdAddr)

	keys := []string{"sample", "thing"}
	labels := []metrics.Label{{"tagkey", "tagvalue"}}

	dog.BuildMetricEmitter(metrics.MetricTypeTimer, keys, labels)(4)
	assertServerMatchesExpected(t, server, buf, "sample.thing:4.000000|ms|#tagkey:tagvalue")

	dog.BuildMetricEmitter(metrics.MetricTypeGauge, keys, labels)(4)
	assertServerMatchesExpected(t, server, buf, "sample.thing:4|g|#tagkey:tagvalue")

	dog.BuildMetricEmitter(metrics.MetricTypeCounter, keys, labels)(4)
	assertServerMatchesExpected(t, server, buf, "sample.thing:4|c|#tagkey:tagvalue")
}

func assertServerMatchesExpected(t *testing.T, server *net.UDPConn, buf []byte, expected string) {
	t.Helper()
	n, _ := server.Read(buf)
	msg := buf[:n]
	if string(msg) != expected {
		t.Fatalf("Line %s does not match expected: %s", string(msg), expected)
	}
}

func TestMetricSinkInterface(t *testing.T) {
	var dd *DogStatsdSink
	_ = metrics.MetricSink(dd)
}

//
// These benchmarks aren't specific to Datadog, but we use the DD sink to get
// realistic performance comparisons (vs using the Blackhole Sink)
//

func BenchmarkSimpleCounter(b *testing.B) {
	cfg := metrics.DefaultConfig("svcname")
	cfg.EnableServiceLabel = true

	s, err := NewDogStatsdSink("127.0.0.1:2181", "my-host")
	if err != nil {
		panic(err)
	}
	met, err := metrics.New(cfg, s)
	if err != nil {
		panic(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		met.Incr("foo", 5, metrics.L("label1", "value1"),
			metrics.L("label2", "value2"))
	}
	b.StopTimer()
	met.Shutdown()
}

func BenchmarkMemoizedCounter(b *testing.B) {
	cfg := metrics.DefaultConfig("svcname")
	cfg.EnableServiceLabel = true

	s, err := NewDogStatsdSink("127.0.0.1:2181", "my-host")
	if err != nil {
		panic(err)
	}
	met, err := metrics.New(cfg, s)
	if err != nil {
		panic(err)
	}

	c := met.NewCounter("foo", metrics.L("label1", "value1"), metrics.L("label2", "value2"))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Incr(5)
	}
	b.StopTimer()
	met.Shutdown()
}

func BenchmarkAggregatedCounter(b *testing.B) {
	cfg := metrics.DefaultConfig("svcname")
	cfg.EnableServiceLabel = true

	s, err := NewDogStatsdSink("127.0.0.1:2181", "my-host")
	if err != nil {
		panic(err)
	}
	met, err := metrics.New(cfg, s)
	if err != nil {
		panic(err)
	}

	c := met.NewAggregatedCounter("foo", metrics.L("label1", "value1"), metrics.L("label2", "value2"))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Incr(5)
	}
	b.StopTimer()
	met.Shutdown()
}
