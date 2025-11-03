package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/client"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_duration_seconds"),
		"Duration of a collector scrape",
		[]string{"collector"}, nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_success"),
		"Whether a collector succeeded",
		[]string{"collector"}, nil,
	)
	upDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Whether the SurrealDB server is up",
		nil, nil,
	)
)

type Exporter struct {
	client     client.Client
	collectors map[string]Collector
	logger     *slog.Logger
}

func NewExporter(logger *slog.Logger, cfg *config.Config) (*Exporter, error) {
	cl, err := client.New(
		cfg.SurrealDB.URI,
		cfg.SurrealDB.Username,
		cfg.SurrealDB.Password,
		cfg.SurrealDB.Namespace,
		cfg.SurrealDB.Database,
		cfg.SurrealDB.Timeout,
	)
	if err != nil {
		return nil, err
	}

	collectors := make(map[string]Collector)
	for name, factory := range factories {
		if !isCollectorEnabled(name, cfg) {
			logger.Debug("Collector disabled", "name", name)
			continue
		}

		collector, err := factory(logger, cfg)
		if err != nil {
			logger.Error("Failed to create collector", "name", name, "error", err)
			continue
		}

		collectors[name] = collector
		logger.Info("Collector enabled", "name", name)
	}

	return &Exporter{
		client:     cl,
		collectors: collectors,
		logger:     logger,
	}, nil
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
	ch <- upDesc
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	up := e.scrape(ctx, ch)

	ch <- prometheus.MustNewConstMetric(upDesc, prometheus.GaugeValue, up)
}

func (e *Exporter) scrape(ctx context.Context, ch chan<- prometheus.Metric) float64 {
	var wg sync.WaitGroup
	wg.Add(len(e.collectors))

	for name, collector := range e.collectors {
		go func(name string, c Collector) {
			defer wg.Done()
			e.executeCollector(ctx, name, c, ch)
		}(name, collector)
	}

	wg.Wait()

	return 1
}

func (e *Exporter) executeCollector(ctx context.Context, name string, c Collector, ch chan<- prometheus.Metric) {
	begin := time.Now()
	err := c.Update(ctx, e.client, ch)
	duration := time.Since(begin)

	var success float64 = 1
	if err != nil {
		e.logger.Error("Collector failed", "name", name, "error", err, "duration", duration)
		success = 0
	} else {
		e.logger.Debug("Collector succeeded", "name", name, "duration", duration)
	}

	ch <- prometheus.MustNewConstMetric(
		scrapeDurationDesc,
		prometheus.GaugeValue,
		duration.Seconds(),
		name,
	)

	ch <- prometheus.MustNewConstMetric(
		scrapeSuccessDesc,
		prometheus.GaugeValue,
		success,
		name,
	)
}
