package surrealcollectors

import (
	"context"
	"log/slog"
	"time"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/client"
	"github.com/prometheus/client_golang/prometheus"
)

type UpCollector struct {
	logger *slog.Logger
	client client.Client
}

func NewUpCollector(logger *slog.Logger, cl client.Client) *UpCollector {
	return &UpCollector{
		logger: logger,
		client: cl,
	}
}

func (c *UpCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- upDesc
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

func (c *UpCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()
	begin := time.Now()
	success := 1.0
	up := 1.0

	if _, err := c.client.Info(ctx); err != nil {
		c.logger.Error("UpCollector: SurrealDB health check failed", "error", err)
		up = 0
		success = 0
	}

	ch <- prometheus.MustNewConstMetric(
		upDesc,
		prometheus.GaugeValue,
		up,
	)

	duration := time.Since(begin)

	ch <- prometheus.MustNewConstMetric(
		scrapeDurationDesc,
		prometheus.GaugeValue,
		duration.Seconds(),
		"up",
	)

	ch <- prometheus.MustNewConstMetric(
		scrapeSuccessDesc,
		prometheus.GaugeValue,
		success,
		"up",
	)
}
