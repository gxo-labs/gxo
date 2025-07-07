package metrics

import "github.com/prometheus/client_golang/prometheus"

// RegistryProvider defines the interface for accessing the engine's metrics registry.
// This allows consumers of the GXO library to expose metrics via their chosen method
// (e.g., Prometheus HTTP endpoint).
type RegistryProvider interface {
	// Registry returns the Prometheus registry containing GXO engine metrics.
	Registry() *prometheus.Registry
}