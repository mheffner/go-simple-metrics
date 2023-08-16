package metrics

func (m *Metrics) enrich(typeName string, key string, labels []Label) (bool, []string, []Label) {
	keys := []string{key}
	if m.cfg.HostName != "" && m.cfg.EnableHostnameLabel {
		labels = append(labels, Label{"host", m.cfg.HostName})
	}
	if m.cfg.ServiceName != "" && m.cfg.EnableServicePrefix {
		keys = insert(0, m.cfg.ServiceName, keys)
	}
	if m.cfg.EnableTypePrefix {
		keys = insert(0, typeName, keys)
	}
	if m.cfg.ServiceName != "" && m.cfg.EnableServiceLabel {
		labels = append(labels, Label{"service", m.cfg.ServiceName})
	}
	labels = append(labels, m.cfg.BaseLabels...)

	allowed, labelsFiltered := m.allowMetric(keys, labels)

	return allowed, keys, labelsFiltered
}
