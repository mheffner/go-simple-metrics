package metrics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnrich(t *testing.T) {
	m := Metrics{cfg: Config{FilterDefault: true, ServiceName: "svcfoo", EnableServicePrefix: false}}
	ok, key, _ := m.enrich("gauge", "metricname", []Label{})
	require.True(t, ok)
	require.Equal(t, []string{"metricname"}, key)

	m = Metrics{cfg: Config{FilterDefault: true, ServiceName: "svcfoo", EnableServicePrefix: true}}
	ok, key, _ = m.enrich("gauge", "metricname", []Label{})
	require.True(t, ok)
	require.Equal(t, []string{"svcfoo", "metricname"}, key)

}
