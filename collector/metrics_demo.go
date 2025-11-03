package collector

import (
	"context"
	"log/slog"
	"math/rand"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/client"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("metrics_demo", true, NewMetricsDemoCollector)
}

type MetricsDemoCollector struct {
	logger *slog.Logger

	counterDesc   *prometheus.Desc
	gaugeDesc     *prometheus.Desc
	histogramDesc *prometheus.Desc
	summaryDesc   *prometheus.Desc

	counter   float64
	histogram prometheus.Histogram
	summary   prometheus.Summary
}

func NewMetricsDemoCollector(logger *slog.Logger, cfg *config.Config) (Collector, error) {
	histogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "demo",
		Name:      "request_duration_seconds",
		Help:      "Request duration distribution",
		Buckets:   prometheus.DefBuckets,
	})

	summary := prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace:  namespace,
		Subsystem:  "demo",
		Name:       "response_size_bytes",
		Help:       "Response size distribution",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})

	return &MetricsDemoCollector{
		logger: logger,
		counterDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "demo", "requests_total"),
			"Total number of requests",
			[]string{"method", "status"}, nil,
		),
		gaugeDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "demo", "active_connections"),
			"Current number of active connections",
			nil, nil,
		),
		histogramDesc: histogram.Desc(),
		summaryDesc:   summary.Desc(),
		counter:       0,
		histogram:     histogram,
		summary:       summary,
	}, nil
}

func (c *MetricsDemoCollector) Update(ctx context.Context, client client.Client, ch chan<- prometheus.Metric) error {
	c.counter += float64(rand.Intn(10) + 1)

	ch <- prometheus.MustNewConstMetric(
		c.counterDesc,
		prometheus.CounterValue,
		c.counter,
		"GET", "200",
	)

	ch <- prometheus.MustNewConstMetric(
		c.counterDesc,
		prometheus.CounterValue,
		c.counter*0.1,
		"POST", "201",
	)

	ch <- prometheus.MustNewConstMetric(
		c.counterDesc,
		prometheus.CounterValue,
		c.counter*0.05,
		"GET", "404",
	)

	activeConnections := float64(rand.Intn(100) + 50)
	ch <- prometheus.MustNewConstMetric(
		c.gaugeDesc,
		prometheus.GaugeValue,
		activeConnections,
	)

	for i := 0; i < 10; i++ {
		c.histogram.Observe(rand.Float64() * 2)
	}

	for i := 0; i < 10; i++ {
		c.summary.Observe(float64(rand.Intn(10000)))
	}

	ch <- c.histogram
	ch <- c.summary

	return nil
}
