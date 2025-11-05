package surrealcollectors

import (
	"context"
	"log/slog"
	"time"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/client"
	"github.com/prometheus/client_golang/prometheus"
)

type InfoCollector struct {
	client client.Client

	infoDesc   *prometheus.Desc
	uptimeDesc *prometheus.Desc
}

func NewServerInfoCollector(cl client.Client) *InfoCollector {
	return &InfoCollector{
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

func (c *InfoCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.infoDesc
	ch <- c.uptimeDesc
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

func (c *InfoCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	begin := time.Now()
	success := 1.0

	info, err := c.client.Info(ctx)
	if err != nil {
		slog.Error("InfoCollector: failed to fetch server info", "error", err)
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
