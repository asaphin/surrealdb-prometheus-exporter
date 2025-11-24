package converter

import (
	"fmt"
	"log/slog"
	"math"
	"sync"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
)

// Config holds converter configuration
type Config interface {
	OTLPTranslationStrategy() string
	OTLPConstLabels() map[string]string
}

// Converter handles conversion of domain metrics to Prometheus format
type Converter struct {
	config   Config
	registry *prometheus.Registry

	// Metric collectors organized by type
	gauges     map[string]*prometheus.GaugeVec
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*HistogramCollector

	mu sync.RWMutex
}

// NewConverter creates a new converter instance
func NewConverter(cfg Config, registry *prometheus.Registry) *Converter {
	return &Converter{
		config:     cfg,
		registry:   registry,
		gauges:     make(map[string]*prometheus.GaugeVec),
		counters:   make(map[string]*prometheus.CounterVec),
		histograms: make(map[string]*HistogramCollector),
	}
}

// Convert converts a batch of domain metrics to Prometheus format
func (c *Converter) Convert(batch domain.MetricBatch) error {
	for _, metric := range batch.Metrics {
		if err := c.convertMetric(metric); err != nil {
			slog.Warn("failed to convert metric",
				"metric", metric.Name,
				"type", metric.Type.String(),
				"error", err)
			// Continue processing other metrics
			continue
		}
	}
	return nil
}

// convertMetric converts a single metric to Prometheus format
func (c *Converter) convertMetric(metric domain.Metric) error {
	// Sanitize and prepare metric name
	promName := domain.SanitizeMetricName(metric.Name, c.config.OTLPTranslationStrategy())
	promName = domain.AddSuffixByType(promName, metric.Type, metric.Unit)

	// Add namespace prefix
	promName = domain.Namespace + "_" + promName

	// Prepare labels
	promLabels, labelNames := c.prepareLabels(metric.Labels)

	// Convert based on metric type
	switch metric.Type {
	case domain.MetricTypeGauge:
		return c.convertGauge(promName, metric, promLabels, labelNames)
	case domain.MetricTypeCounter:
		return c.convertCounter(promName, metric, promLabels, labelNames)
	case domain.MetricTypeHistogram:
		return c.convertHistogram(promName, metric, promLabels, labelNames)
	default:
		return fmt.Errorf("unsupported metric type: %v", metric.Type)
	}
}

// prepareLabels sanitizes labels and adds constant labels
func (c *Converter) prepareLabels(labels map[string]string) (map[string]string, []string) {
	promLabels := make(map[string]string)
	labelNames := make([]string, 0, len(labels)+len(c.config.OTLPConstLabels()))

	// Sanitize metric labels
	for k, v := range labels {
		sanitizedKey := domain.SanitizeLabelName(k)
		promLabels[sanitizedKey] = v
		labelNames = append(labelNames, sanitizedKey)
	}

	// Add constant labels
	for k, v := range c.config.OTLPConstLabels() {
		promLabels[k] = v
		labelNames = append(labelNames, k)
	}

	return promLabels, labelNames
}

// convertGauge converts a gauge metric
func (c *Converter) convertGauge(name string, metric domain.Metric, labels map[string]string, labelNames []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	gauge, exists := c.gauges[name]
	if !exists {
		gauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: name,
				Help: metric.Description,
			},
			labelNames,
		)

		if err := c.registry.Register(gauge); err != nil {
			// Check if already registered by another goroutine
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				gauge = are.ExistingCollector.(*prometheus.GaugeVec)
			} else {
				return fmt.Errorf("register gauge: %w", err)
			}
		}

		c.gauges[name] = gauge
	}

	gauge.With(labels).Set(metric.Value)
	return nil
}

// convertCounter converts a counter metric
func (c *Converter) convertCounter(name string, metric domain.Metric, labels map[string]string, labelNames []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	counter, exists := c.counters[name]
	if !exists {
		counter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: name,
				Help: metric.Description,
			},
			labelNames,
		)

		if err := c.registry.Register(counter); err != nil {
			// Check if already registered by another goroutine
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				counter = are.ExistingCollector.(*prometheus.CounterVec)
			} else {
				return fmt.Errorf("register counter: %w", err)
			}
		}

		c.counters[name] = counter
	}

	// For counters, we need to add the delta
	// Since OTLP typically sends cumulative values, we just set it
	// Note: This is a simplification - in production you may want to track state
	counter.With(labels).Add(metric.Value)
	return nil
}

// convertHistogram converts a histogram metric
func (c *Converter) convertHistogram(name string, metric domain.Metric, labels map[string]string, labelNames []string) error {
	if !metric.HasHistogramData() {
		return fmt.Errorf("histogram metric missing histogram data")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	histCollector, exists := c.histograms[name]
	if !exists {
		histCollector = NewHistogramCollector(name, metric.Description, labelNames)

		if err := c.registry.Register(histCollector); err != nil {
			// Check if already registered by another goroutine
			if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
				histCollector = are.ExistingCollector.(*HistogramCollector)
			} else {
				return fmt.Errorf("register histogram: %w", err)
			}
		}

		c.histograms[name] = histCollector
	}

	// Update histogram with new data
	histCollector.Update(metric, labels)
	return nil
}

// HistogramCollector is a custom Prometheus collector for histograms
// It uses ConstHistogram to allow setting bucket values directly
type HistogramCollector struct {
	name        string
	description string
	labelNames  []string

	mu      sync.RWMutex
	metrics []prometheus.Metric
}

// NewHistogramCollector creates a new histogram collector
func NewHistogramCollector(name, description string, labelNames []string) *HistogramCollector {
	return &HistogramCollector{
		name:        name,
		description: description,
		labelNames:  labelNames,
		metrics:     make([]prometheus.Metric, 0),
	}
}

// Update updates the histogram with new metric data
func (h *HistogramCollector) Update(metric domain.Metric, labels map[string]string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Convert labels to prometheus.Labels
	promLabels := prometheus.Labels(labels)

	// Convert buckets to map
	buckets := make(map[float64]uint64)
	for _, bucket := range metric.HistogramData.Buckets {
		buckets[bucket.UpperBound] = bucket.Count
	}

	// Create descriptor
	desc := prometheus.NewDesc(
		h.name,
		h.description,
		nil,
		promLabels,
	)

	// Create const histogram
	histMetric, err := prometheus.NewConstHistogram(
		desc,
		metric.HistogramData.Count,
		metric.HistogramData.Sum,
		buckets,
	)

	if err != nil {
		slog.Error("failed to create const histogram",
			"metric", h.name,
			"error", err)
		return
	}

	// Store the metric (replace if exists for same labels)
	// For simplicity, we'll just append - in production you might want
	// to implement a more sophisticated cache with expiration
	h.metrics = append(h.metrics, histMetric)

	// Limit the cache size to prevent memory issues
	if len(h.metrics) > 10000 {
		h.metrics = h.metrics[len(h.metrics)-5000:]
	}
}

// Describe implements prometheus.Collector
func (h *HistogramCollector) Describe(ch chan<- *prometheus.Desc) {
	// We use NewConstHistogram, so we don't pre-register descriptions
	// This is dynamic collection
}

// Collect implements prometheus.Collector
func (h *HistogramCollector) Collect(ch chan<- prometheus.Metric) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, metric := range h.metrics {
		ch <- metric
	}
}

// BucketsFromHistogramData extracts bucket boundaries from histogram data
func BucketsFromHistogramData(data *domain.HistogramData) []float64 {
	buckets := make([]float64, 0, len(data.Buckets))
	for _, bucket := range data.Buckets {
		if !math.IsInf(bucket.UpperBound, 1) {
			buckets = append(buckets, bucket.UpperBound)
		}
	}
	return buckets
}
