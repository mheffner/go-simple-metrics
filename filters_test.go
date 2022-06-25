package metrics

import (
	"reflect"
	"testing"
)

func TestMetrics_Filter_Denylist(t *testing.T) {
	m := &MockSink{}
	conf := DefaultConfig("")
	conf.AllowedPrefixes = []string{"service", "debug.thing"}
	conf.BlockedPrefixes = []string{"debug"}
	met, err := New(conf, m)
	if err != nil {
		t.Fatal(err)
	}

	// Allowed by default
	key := "thing"
	met.SetGauge(key, 1)
	if !reflect.DeepEqual(m.getKeys()[0], []string{key}) {
		t.Fatalf("key doesn't exist %v, %v", m.getKeys()[0], key)
	}
	if m.vals[0] != 1 {
		t.Fatalf("bad val: %v", m.vals[0])
	}

	// Allowed by filter
	key = "service.thing"
	met.SetGauge(key, 2)
	if !reflect.DeepEqual(m.getKeys()[1], []string{key}) {
		t.Fatalf("key doesn't exist")
	}
	if m.vals[1] != 2 {
		t.Fatalf("bad val: %v", m.vals[1])
	}

	// Allowed by filter, subtree of a blocked entry
	key = "debug.thing"
	met.SetGauge(key, 3)
	if !reflect.DeepEqual(m.getKeys()[2], []string{key}) {
		t.Fatalf("key doesn't exist")
	}
	if m.vals[2] != 3 {
		t.Fatalf("bad val: %v", m.vals[2])
	}

	// Blocked by filter
	key = "debug.other-thing"
	met.SetGauge(key, 4)
	if len(m.getKeys()) != 3 {
		t.Fatalf("key shouldn't exist")
	}
}

func TestMetrics_Filter_Allowlist(t *testing.T) {
	m := &MockSink{}
	conf := DefaultConfig("")
	conf.AllowedPrefixes = []string{"service", "debug.thing"}
	conf.BlockedPrefixes = []string{"debug"}
	conf.FilterDefault = false
	conf.BlockedLabels = []string{"bad_label"}
	met, err := New(conf, m)
	if err != nil {
		t.Fatal(err)
	}

	// Blocked by default
	key := "thing"
	met.SetGauge(key, 1)
	if len(m.getKeys()) != 0 {
		t.Fatalf("key should not exist")
	}

	// Allowed by filter
	key = "service.thing"
	met.SetGauge(key, 2)
	if !reflect.DeepEqual(m.getKeys()[0], []string{key}) {
		t.Fatalf("key doesn't exist")
	}
	if m.vals[0] != 2 {
		t.Fatalf("bad val: %v", m.vals[0])
	}

	// Allowed by filter, subtree of a blocked entry
	key = "debug.thing"
	met.SetGauge(key, 3)
	if !reflect.DeepEqual(m.getKeys()[1], []string{key}) {
		t.Fatalf("key doesn't exist")
	}
	if m.vals[1] != 3 {
		t.Fatalf("bad val: %v", m.vals[1])
	}

	// Blocked by filter
	key = "debug.other-thing"
	met.SetGauge(key, 4)
	if len(m.getKeys()) != 2 {
		t.Fatalf("key shouldn't exist")
	}
	// Test blacklisting of labels
	key = "debug.thing"
	goodLabel := Label{Name: "good", Value: "should be present"}
	badLabel := Label{Name: "bad_label", Value: "should not be there"}
	labels := []Label{badLabel, goodLabel}
	met.SetGauge(key, 3, labels...)
	if !reflect.DeepEqual(m.getKeys()[1], []string{key}) {
		t.Fatalf("key doesn't exist")
	}
	if m.vals[2] != 3 {
		t.Fatalf("bad val: %v", m.vals[1])
	}
	if HasElem(m.labels[2], badLabel) {
		t.Fatalf("bad_label should not be present in %v", m.labels[2])
	}
	if !HasElem(m.labels[2], goodLabel) {
		t.Fatalf("good label is not present in %v", m.labels[2])
	}
}

func TestMetrics_Filter_Labels_Allowlist(t *testing.T) {
	m := &MockSink{}
	conf := DefaultConfig("")
	conf.AllowedPrefixes = []string{"service", "debug.thing"}
	conf.BlockedPrefixes = []string{"debug"}
	conf.FilterDefault = false
	conf.AllowedLabels = []string{"good_label"}
	conf.BlockedLabels = []string{"bad_label"}
	conf.EnableRuntimeMetrics = false
	met, err := New(conf, m)
	if err != nil {
		t.Fatal(err)
	}

	// Blocked by default
	key := "thing"
	key = "debug.thing"
	goodLabel := Label{Name: "good_label", Value: "should be present"}
	notReallyGoodLabel := Label{Name: "not_really_good_label", Value: "not whitelisted, but not blacklisted"}
	badLabel := Label{Name: "bad_label", Value: "should not be there"}
	labels := []Label{badLabel, notReallyGoodLabel, goodLabel}
	met.SetGauge(key, 1, labels...)

	if HasElem(m.labels[0], badLabel) {
		t.Fatalf("bad_label should not be present in %v", m.labels[0])
	}
	if HasElem(m.labels[0], notReallyGoodLabel) {
		t.Fatalf("not_really_good_label should not be present in %v", m.labels[0])
	}
	if !HasElem(m.labels[0], goodLabel) {
		t.Fatalf("good label is not present in %v", m.labels[0])
	}

	conf.AllowedLabels = nil
	met.setFilterAndLabels(conf.AllowedPrefixes, conf.BlockedLabels, conf.AllowedLabels, conf.BlockedLabels)
	met.SetGauge(key, 1, labels...)

	if HasElem(m.labels[1], badLabel) {
		t.Fatalf("bad_label should not be present in %v", m.labels[1])
	}
	// Since no whitelist, not_really_good_label should be there
	if !HasElem(m.labels[1], notReallyGoodLabel) {
		t.Fatalf("not_really_good_label is not present in %v", m.labels[1])
	}
	if !HasElem(m.labels[1], goodLabel) {
		t.Fatalf("good label is not present in %v", m.labels[1])
	}
}

func TestMetrics_Filter_Labels_ModifyArgs(t *testing.T) {
	m := &MockSink{}
	conf := DefaultConfig("")
	conf.FilterDefault = false
	conf.AllowedLabels = []string{"keep"}
	conf.BlockedLabels = []string{"delete"}
	met, err := New(conf, m)
	if err != nil {
		t.Fatal(err)
	}

	// Blocked by default
	key := "thing"
	key = "debug.thing"
	goodLabel := Label{Name: "keep", Value: "should be kept"}
	badLabel := Label{Name: "delete", Value: "should be deleted"}
	argLabels := []Label{badLabel, goodLabel, badLabel, goodLabel, badLabel, goodLabel, badLabel}
	origLabels := append([]Label{}, argLabels...)
	met.SetGauge(key, 1, argLabels...)

	if !reflect.DeepEqual(argLabels, origLabels) {
		t.Fatalf("SetGauge modified the input argument")
	}
}
