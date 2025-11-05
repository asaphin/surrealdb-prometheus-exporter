package registry

import (
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/client"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/config"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/surrealcollectors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

func New(cfg *config.Config, cl client.Client) (*prometheus.Registry, error) {
	registry := prometheus.NewRegistry()

	registry.MustRegister(
		surrealcollectors.NewServerInfoCollector(cl),
		//surrealcollectors.NewMetricsDemoCollector(,
		//surrealcollectors.NewUpCollector(cl),
	)

	if cfg.Collectors.Go.Enabled {
		registry.MustRegister(collectors.NewBuildInfoCollector())
		registry.MustRegister(collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll)))
	}

	if cfg.Collectors.Process.Enabled {
		registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}

	return registry, nil
}
