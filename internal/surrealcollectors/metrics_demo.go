package surrealcollectors

import (
	"log/slog"
	"math/rand"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type MetricsDemoCollector struct {
	logger *slog.Logger

	counterDesc *prometheus.Desc
	gaugeDesc   *prometheus.Desc

	counter   float64
	histogram prometheus.Histogram
	summary   prometheus.Summary
}

func NewMetricsDemoCollector(logger *slog.Logger) *MetricsDemoCollector {
	histogram := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: "demo",
		Name:      "request_duration_seconds",
		Help:      "Request duration distribution.",
		Buckets:   prometheus.DefBuckets,
	})

	summary := prometheus.NewSummary(prometheus.SummaryOpts{
		Namespace:  namespace,
		Subsystem:  "demo",
		Name:       "response_size_bytes",
		Help:       "Response size distribution.",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	})

	return &MetricsDemoCollector{
		logger: logger,
		counterDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "demo", "requests_total"),
			"Total number of requests.",
			[]string{"method", "status"}, nil,
		),
		gaugeDesc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "demo", "active_connections"),
			"Current number of active connections.",
			nil, nil,
		),
		counter:   0,
		histogram: histogram,
		summary:   summary,
	}
}

func (c *MetricsDemoCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.counterDesc
	ch <- c.gaugeDesc
	c.histogram.Describe(ch)
	c.summary.Describe(ch)
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

func (c *MetricsDemoCollector) Collect(ch chan<- prometheus.Metric) {
	begin := time.Now()
	success := 1.0

	// Synthetic counters
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

	// Update internal histogram & summary
	for i := 0; i < 10; i++ {
		c.histogram.Observe(rand.Float64() * 2)
	}

	for i := 0; i < 10; i++ {
		c.summary.Observe(float64(rand.Intn(10000)))
	}

	// Export them
	c.histogram.Collect(ch)
	c.summary.Collect(ch)

	duration := time.Since(begin)

	ch <- prometheus.MustNewConstMetric(
		scrapeDurationDesc,
		prometheus.GaugeValue,
		duration.Seconds(),
		"metrics_demo",
	)

	ch <- prometheus.MustNewConstMetric(
		scrapeSuccessDesc,
		prometheus.GaugeValue,
		success,
		"metrics_demo",
	)
}
