package collector

import (
	"context"
	"log/slog"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/client"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("server_info", true, NewServerInfoCollector)
}

type ServerInfoCollector struct {
	logger *slog.Logger

	infoDesc   *prometheus.Desc
	uptimeDesc *prometheus.Desc
}

func NewServerInfoCollector(logger *slog.Logger, cfg *config.Config) (Collector, error) {
	return &ServerInfoCollector{
		logger: logger,
		infoDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "server", "info"),
			"Server information",
			[]string{"version", "namespace", "database"}, nil,
		),
		uptimeDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "server", "uptime_seconds"),
			"Server uptime in seconds",
			nil, nil,
		),
	}, nil
}

func (c *ServerInfoCollector) Update(ctx context.Context, client client.Client, ch chan<- prometheus.Metric) error {
	info, err := client.Info(ctx)
	if err != nil {
		return err
	}

	ch <- prometheus.MustNewConstMetric(
		c.infoDesc,
		prometheus.GaugeValue,
		1,
		info.Version,
		info.Namespace,
		info.Database,
	)

	ch <- prometheus.MustNewConstMetric(
		c.uptimeDesc,
		prometheus.GaugeValue,
		info.Uptime.Seconds(),
	)

	return nil
}
