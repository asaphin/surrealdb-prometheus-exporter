package registry

import (
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/surrealcollectors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type Config interface {
	surrealcollectors.Config
	InfoCollectorEnabled() bool
	GoCollectorEnabled() bool
	ProcessCollectorEnabled() bool
}

func New(cfg Config, versionReader surrealcollectors.VersionReader, infoMetricsReader surrealcollectors.InfoMetricsReader, recordCountReader surrealcollectors.RecordCountReader) (*prometheus.Registry, error) {
	registry := prometheus.NewRegistry()

	if cfg.InfoCollectorEnabled() {
		registry.MustRegister(
			surrealcollectors.NewInfoCollector(cfg, versionReader, infoMetricsReader),
			surrealcollectors.NewRecordCountCollector(recordCountReader),
		)
	}

	if cfg.GoCollectorEnabled() {
		registry.MustRegister(collectors.NewBuildInfoCollector())
		registry.MustRegister(collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll)))
	}

	if cfg.GoCollectorEnabled() {
		registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}

	return registry, nil
}
