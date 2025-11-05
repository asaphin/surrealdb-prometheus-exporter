package surrealcollectors

import (
	"context"
	"log/slog"
	"time"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/client"
	"github.com/prometheus/client_golang/prometheus"
)

type ServerInfoCollector struct {
	logger *slog.Logger
	client client.Client

	infoDesc   *prometheus.Desc
	uptimeDesc *prometheus.Desc
}

func NewServerInfoCollector(logger *slog.Logger, cl client.Client) *ServerInfoCollector {
	return &ServerInfoCollector{
		logger: logger,
		client: cl,
		infoDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "server", "info"),
			"Server information.",
			[]string{"version", "namespace", "database"}, nil,
		),
		uptimeDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "server", "uptime_seconds"),
			"Server uptime in seconds.",
			nil, nil,
		),
	}
}

func (c *ServerInfoCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.infoDesc
	ch <- c.uptimeDesc
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

func (c *ServerInfoCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	begin := time.Now()
	success := 1.0

	info, err := c.client.Info(ctx)
	if err != nil {
		c.logger.Error("ServerInfoCollector: failed to fetch server info", "error", err)
		success = 0
	} else {
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
	}

	duration := time.Since(begin)

	ch <- prometheus.MustNewConstMetric(
		scrapeDurationDesc,
		prometheus.GaugeValue,
		duration.Seconds(),
		"server_info",
	)

	ch <- prometheus.MustNewConstMetric(
		scrapeSuccessDesc,
		prometheus.GaugeValue,
		success,
		"server_info",
	)
}
