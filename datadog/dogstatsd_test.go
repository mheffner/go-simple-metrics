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
