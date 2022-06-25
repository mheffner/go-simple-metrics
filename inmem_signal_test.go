package metrics

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestInmemSignal(t *testing.T) {
	buf := newBuffer()
	inm := NewInmemSink(10*time.Millisecond, 50*time.Millisecond)
	sig := NewInmemSignal(inm, syscall.SIGUSR1, buf)
	defer sig.Stop()

	gEmitter := inm.BuildMetricEmitter(MetricTypeGauge, []string{"foo"}, []Label{})
	gEmitterLabels := inm.BuildMetricEmitter(MetricTypeGauge, []string{"asdf"}, []Label{{"a", "b"}})
	cEmitter := inm.BuildMetricEmitter(MetricTypeCounter, []string{"baz"}, []Label{})
	cEmitterLabels := inm.BuildMetricEmitter(MetricTypeCounter, []string{"qwer"}, []Label{{"a", "b"}})
	sEmitter := inm.BuildMetricEmitter(MetricTypeHistogram, []string{"wow"}, []Label{})
	sEmitterLabels := inm.BuildMetricEmitter(MetricTypeHistogram, []string{"zxcv"}, []Label{{"a", "b"}})

	gEmitter(42)
	cEmitter(42)
	sEmitter(42)
	gEmitterLabels(42)
	cEmitterLabels(42)
	sEmitterLabels(42)

	// Wait for period to end
	time.Sleep(15 * time.Millisecond)

	// Send signal!
	syscall.Kill(os.Getpid(), syscall.SIGUSR1)

	// Wait for flush
	time.Sleep(10 * time.Millisecond)

	// Check the output
	out := buf.String()
	if !strings.Contains(out, "[G] 'foo': 42") {
		t.Fatalf("bad: %v", out)
	}
	if !strings.Contains(out, "[C] 'baz': Count: 1 Sum: 42") {
		t.Fatalf("bad: %v", out)
	}
	if !strings.Contains(out, "[S] 'wow': Count: 1 Sum: 42") {
		t.Fatalf("bad: %v", out)
	}
	if !strings.Contains(out, "[G] 'asdf.b': 42") {
		t.Fatalf("bad: %v", out)
	}
	if !strings.Contains(out, "[C] 'qwer.b': Count: 1 Sum: 42") {
		t.Fatalf("bad: %v", out)
	}
	if !strings.Contains(out, "[S] 'zxcv.b': Count: 1 Sum: 42") {
		t.Fatalf("bad: %v", out)
	}
}

func newBuffer() *syncBuffer {
	return &syncBuffer{buf: bytes.NewBuffer(nil)}
}

type syncBuffer struct {
	buf  *bytes.Buffer
	lock sync.Mutex
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.buf.String()
}
