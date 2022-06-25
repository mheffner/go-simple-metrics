package metrics

import (
	"strings"

	iradix "github.com/hashicorp/go-immutable-radix"
)

// setFilterAndLabels overwrites the existing filter with the given rules.
func (m *Metrics) setFilterAndLabels(allow, block, allowedLabels, blockedLabels []string) {
	m.cfg.AllowedPrefixes = allow
	m.cfg.BlockedPrefixes = block

	if allowedLabels == nil {
		// Having a white list means we take only elements from it
		m.allowedLabels = nil
	} else {
		m.allowedLabels = make(map[string]bool)
		for _, v := range allowedLabels {
			m.allowedLabels[v] = true
		}
	}
	m.blockedLabels = make(map[string]bool)
	for _, v := range blockedLabels {
		m.blockedLabels[v] = true
	}
	m.cfg.AllowedLabels = allowedLabels
	m.cfg.BlockedLabels = blockedLabels

	m.filter = iradix.New()
	for _, prefix := range m.cfg.AllowedPrefixes {
		m.filter, _, _ = m.filter.Insert([]byte(prefix), true)
	}
	for _, prefix := range m.cfg.BlockedPrefixes {
		m.filter, _, _ = m.filter.Insert([]byte(prefix), false)
	}
}

// labelIsAllowed return true if a should be included in metric
// the caller should lock m.filterLock while calling this method
func (m *Metrics) labelIsAllowed(label *Label) bool {
	labelName := (*label).Name
	if m.blockedLabels != nil {
		_, ok := m.blockedLabels[labelName]
		if ok {
			// If present, let's remove this label
			return false
		}
	}
	if m.allowedLabels != nil {
		_, ok := m.allowedLabels[labelName]
		return ok
	}
	// Allow by default
	return true
}

// filterLabels return only allowed labels
// the caller should lock m.filterLock while calling this method
func (m *Metrics) filterLabels(labels []Label) []Label {
	if labels == nil {
		return nil
	}
	toReturn := []Label{}
	for _, label := range labels {
		if m.labelIsAllowed(&label) {
			toReturn = append(toReturn, label)
		}
	}
	return toReturn
}

// Returns whether the metric should be allowed based on configured prefix filters
// Also return the applicable labels
func (m *Metrics) allowMetric(key []string, labels []Label) (bool, []Label) {
	if m.filter == nil || m.filter.Len() == 0 {
		return m.cfg.FilterDefault, m.filterLabels(labels)
	}

	_, allowed, ok := m.filter.Root().LongestPrefix([]byte(strings.Join(key, ".")))
	if !ok {
		return m.cfg.FilterDefault, m.filterLabels(labels)
	}

	return allowed.(bool), m.filterLabels(labels)
}
