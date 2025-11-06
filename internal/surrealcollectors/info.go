package surrealcollectors

import (
	"context"
	"log/slog"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "surrealcollectors" // TODO move to config

type MetricsReader interface {
	Info(ctx context.Context) (*domain.SurrealDBInfo, error)
}

type InfoCollector struct {
	metricsReader MetricsReader

	availableParallelismDesc *prometheus.Desc
}

func NewServerInfoCollector(metricsReader MetricsReader) *InfoCollector {
	return &InfoCollector{
		metricsReader: metricsReader,
		availableParallelismDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "server", "available_parallelism"),
			"SurrealDB node available parallelism",
			/*[]string{"version", "namespace", "database"}*/ nil, nil,
		),
	}
}

func (c *InfoCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.availableParallelismDesc
}

func (c *InfoCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	info, err := c.metricsReader.Info(ctx)
	if err != nil {
		slog.Error("InfoCollector: failed to fetch server info", "error", err)
	} else {
		ch <- prometheus.MustNewConstMetric(
			c.availableParallelismDesc,
			prometheus.GaugeValue,
			info.AvailableParallelism(),
			//info.Version,
			//info.Namespace,
			//info.Database,
		)
	}
}
