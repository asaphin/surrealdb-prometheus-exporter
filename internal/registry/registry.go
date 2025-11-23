package registry

import (
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/surrealcollectors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type Config interface {
	InfoCollectorEnabled() bool
	LiveQueryEnabled() bool
	GoCollectorEnabled() bool
	ProcessCollectorEnabled() bool
	ClusterName() string
	StorageEngine() string
	DeploymentMode() string
}

func New(
	cfg Config,
	versionReader surrealcollectors.VersionReader,
	infoMetricsReader surrealcollectors.InfoMetricsReader,
	recordCountReader surrealcollectors.RecordCountReader,
	liveQueryProvider surrealcollectors.LiveQueryInfoProvider,
	filter surrealcollectors.TableFilter,
) (prometheus.Gatherer, error) {
	registry := prometheus.NewRegistry()

	constantLabels := prometheus.Labels{
		"cluster":         cfg.ClusterName(),
		"storage_engine":  cfg.StorageEngine(),
		"deployment_mode": cfg.DeploymentMode(),
	}

	prometheus.WrapCollectorWith(constantLabels, registry)

	if cfg.InfoCollectorEnabled() {
		registry.MustRegister(
			prometheus.WrapCollectorWith(constantLabels, surrealcollectors.NewInfoCollector(versionReader, infoMetricsReader)),
			prometheus.WrapCollectorWith(constantLabels, surrealcollectors.NewRecordCountCollector(recordCountReader)),
		)
	}

	if cfg.LiveQueryEnabled() {
		registry.MustRegister(prometheus.WrapCollectorWith(constantLabels, surrealcollectors.NewLiveQueryCollector(liveQueryProvider, filter)))
	}

	if cfg.GoCollectorEnabled() {
		registry.MustRegister(prometheus.WrapCollectorWith(constantLabels, collectors.NewBuildInfoCollector()))
		registry.MustRegister(prometheus.WrapCollectorWith(constantLabels, collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll))))
	}

	if cfg.ProcessCollectorEnabled() {
		registry.MustRegister(prometheus.WrapCollectorWith(constantLabels, collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})))
	}

	return registry, nil
}
