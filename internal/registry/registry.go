package registry

import (
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/surrealcollectors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type Config interface {
	InfoCollectorEnabled() bool
	RecordCountCollectorEnabled() bool
	LiveQueryEnabled() bool
	StatsTableEnabled() bool
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
	statsTableProvider surrealcollectors.StatsTableInfoProvider,
	liveQueryFilter surrealcollectors.TableFilter,
	statsTableFilter surrealcollectors.TableFilter,
) (prometheus.Gatherer, error) {
	registry := prometheus.NewRegistry()

	constantLabels := prometheus.Labels{
		"cluster":         cfg.ClusterName(),
		"storage_engine":  cfg.StorageEngine(),
		"deployment_mode": cfg.DeploymentMode(),
	}

	prometheus.WrapCollectorWith(constantLabels, registry)

	// Info collector is always active
	if cfg.InfoCollectorEnabled() {
		registry.MustRegister(
			prometheus.WrapCollectorWith(constantLabels, surrealcollectors.NewInfoCollector(versionReader, infoMetricsReader)),
		)
	}

	// Record count collector is now separately configurable
	if cfg.RecordCountCollectorEnabled() {
		registry.MustRegister(
			prometheus.WrapCollectorWith(constantLabels, surrealcollectors.NewRecordCountCollector(recordCountReader)),
		)
	}

	if cfg.LiveQueryEnabled() {
		registry.MustRegister(prometheus.WrapCollectorWith(constantLabels, surrealcollectors.NewLiveQueryCollector(liveQueryProvider, liveQueryFilter)))
	}

	if cfg.StatsTableEnabled() {
		registry.MustRegister(prometheus.WrapCollectorWith(constantLabels, surrealcollectors.NewStatsTableCollector(statsTableProvider, statsTableFilter)))
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
