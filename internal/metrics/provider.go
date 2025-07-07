package metrics

import (
	gxo "github.com/gxo-labs/gxo/pkg/gxo/v1/metrics" // Use pkg interface
	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusRegistryProvider implements the RegistryProvider interface
// using a standard Prometheus registry.
type PrometheusRegistryProvider struct {
	registry *prometheus.Registry
}

// NewPrometheusRegistryProvider creates a new metrics provider backed by Prometheus.
func NewPrometheusRegistryProvider() *PrometheusRegistryProvider {
	return &PrometheusRegistryProvider{
		registry: prometheus.NewRegistry(),
	}
}

// Registry returns the underlying Prometheus registry.
func (p *PrometheusRegistryProvider) Registry() *prometheus.Registry {
	return p.registry
}

// Ensure implementation satisfies the interface.
var _ gxo.RegistryProvider = (*PrometheusRegistryProvider)(nil)