package registry

import (
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/surrealcollectors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type Config interface {
	GoCollectorEnabled() bool
	ProcessCollectorEnabled() bool
}

func New(cfg Config, metricsReader surrealcollectors.MetricsReader) (*prometheus.Registry, error) {
	registry := prometheus.NewRegistry()

	registry.MustRegister(
		surrealcollectors.NewServerInfoCollector(metricsReader),
		//surrealcollectors.NewMetricsDemoCollector(,
		//surrealcollectors.NewUpCollector(cl),
	)

	if cfg.GoCollectorEnabled() {
		registry.MustRegister(collectors.NewBuildInfoCollector())
		registry.MustRegister(collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll)))
	}

	if cfg.GoCollectorEnabled() {
		registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}

	return registry, nil
}
