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
	ClusterName() string
	StorageEngine() string
	DeploymentMode() string
}

// Converter handles conversion of domain metrics to Prometheus format
type Converter struct {
	config      Config
	registry    *prometheus.Registry
	constLabels map[string]string

	// Metric collectors organized by type
	gauges     map[string]*prometheus.GaugeVec
	counters   map[string]*prometheus.CounterVec
	histograms map[string]*HistogramCollector

	// Track label names for each registered metric to ensure consistency
	metricLabelNames map[string][]string

	mu sync.RWMutex
}

// NewConverter creates a new converter instance
func NewConverter(cfg Config, registry *prometheus.Registry) *Converter {
	// Build constant labels from cluster configuration (same approach as registry)
	constLabels := map[string]string{
		"cluster":         cfg.ClusterName(),
		"storage_engine":  cfg.StorageEngine(),
		"deployment_mode": cfg.DeploymentMode(),
	}

	return &Converter{
		config:           cfg,
		registry:         registry,
		constLabels:      constLabels,
		gauges:           make(map[string]*prometheus.GaugeVec),
		counters:         make(map[string]*prometheus.CounterVec),
		histograms:       make(map[string]*HistogramCollector),
		metricLabelNames: make(map[string][]string),
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
	// Store original metric name for unit correction lookups
	originalName := metric.Name

	// Sanitize and prepare metric name
	promName := domain.SanitizeMetricName(metric.Name, c.config.OTLPTranslationStrategy())
	promName = domain.AddSuffixByTypeForMetric(promName, originalName, metric.Type, metric.Unit)

	// Add namespace prefix
	promName = domain.Namespace + "_" + promName

	// Prepare labels with consistency check
	promLabels, labelNames := c.prepareLabels(promName, metric.Labels)

	// Convert based on metric type
	switch metric.Type {
	case domain.MetricTypeGauge:
		return c.convertGauge(promName, originalName, metric, promLabels, labelNames)
	case domain.MetricTypeCounter:
		return c.convertCounter(promName, originalName, metric, promLabels, labelNames)
	case domain.MetricTypeHistogram:
		return c.convertHistogram(promName, originalName, metric, promLabels, labelNames)
	default:
		return fmt.Errorf("unsupported metric type: %v", metric.Type)
	}
}

// prepareLabels sanitizes labels and adds constant labels
// Now also accepts metric name to ensure label consistency
func (c *Converter) prepareLabels(metricName string, labels map[string]string) (map[string]string, []string) {
	// Check if we have previously registered label names for this metric
	if existingLabelNames, exists := c.metricLabelNames[metricName]; exists {
		// Use the existing label names to maintain consistency
		promLabels := make(map[string]string)
		for _, labelName := range existingLabelNames {
			if value, ok := labels[labelName]; ok {
				promLabels[labelName] = value
			} else {
				// Provide empty string for missing labels
				promLabels[labelName] = ""
			}
		}
		// Add constant labels
		for k, v := range c.constLabels {
			promLabels[k] = v
		}
		return promLabels, existingLabelNames
	}

	// First time seeing this metric - create new label set
	promLabels := make(map[string]string)
	labelNames := make([]string, 0, len(labels)+len(c.constLabels))

	// Sanitize metric labels
	for k, v := range labels {
		sanitizedKey := domain.SanitizeLabelName(k)
		promLabels[sanitizedKey] = v
		labelNames = append(labelNames, sanitizedKey)
	}

	// Add constant labels
	for k, v := range c.constLabels {
		promLabels[k] = v
		labelNames = append(labelNames, k)
	}

	// Store the label names for future consistency
	c.metricLabelNames[metricName] = labelNames

	return promLabels, labelNames
}

// convertGauge converts a gauge metric
func (c *Converter) convertGauge(name, originalName string, metric domain.Metric, labels map[string]string, labelNames []string) error {
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

	// Apply unit conversion to value, using metric-aware correction
	value := domain.ConvertValueForMetric(metric.Value, originalName, metric.Unit)
	gauge.With(labels).Set(value)
	return nil
}

// convertCounter converts a counter metric
func (c *Converter) convertCounter(name, originalName string, metric domain.Metric, labels map[string]string, labelNames []string) error {
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

	// Apply unit conversion to value, using metric-aware correction
	// For counters, we need to add the delta
	// Since OTLP typically sends cumulative values, we just set it
	// Note: This is a simplification - in production you may want to track state
	value := domain.ConvertValueForMetric(metric.Value, originalName, metric.Unit)
	counter.With(labels).Add(value)
	return nil
}

// convertHistogram converts a histogram metric
func (c *Converter) convertHistogram(name, originalName string, metric domain.Metric, labels map[string]string, labelNames []string) error {
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

	// Apply unit conversion to histogram data, using metric-aware correction
	convertedMetric := metric
	if metric.Unit != "" {
		convertedMetric = convertHistogramUnitsForMetric(metric, originalName)
	}

	// Update histogram with new data
	histCollector.Update(convertedMetric, labels)
	return nil
}

// convertHistogramUnitsForMetric applies unit conversion to histogram bucket bounds and sum,
// using metric-aware correction for known OTEL metrics
func convertHistogramUnitsForMetric(metric domain.Metric, originalName string) domain.Metric {
	conv := domain.GetUnitConversionForMetric(originalName, metric.Unit)
	if conv == nil || conv.Multiplier == 1 {
		return metric
	}

	// Create a copy of the histogram data with converted values
	convertedData := &domain.HistogramData{
		Count:       metric.HistogramData.Count,
		Sum:         metric.HistogramData.Sum * conv.Multiplier,
		CreatedTime: metric.HistogramData.CreatedTime,
		Buckets:     make([]domain.HistogramBucket, len(metric.HistogramData.Buckets)),
	}

	// Convert bucket upper bounds
	for i, bucket := range metric.HistogramData.Buckets {
		convertedData.Buckets[i] = domain.HistogramBucket{
			UpperBound: bucket.UpperBound * conv.Multiplier,
			Count:      bucket.Count,
		}
	}

	return domain.Metric{
		Name:          metric.Name,
		Type:          metric.Type,
		Value:         metric.Value,
		Labels:        metric.Labels,
		Timestamp:     metric.Timestamp,
		Description:   metric.Description,
		Unit:          metric.Unit,
		HistogramData: convertedData,
	}
}

// histogramData stores the data needed to create a histogram metric
type histogramData struct {
	count   uint64
	sum     float64
	buckets map[float64]uint64
}

// HistogramCollector is a custom Prometheus collector for histograms
// It uses ConstHistogram to allow setting bucket values directly
type HistogramCollector struct {
	name        string
	description string
	labelNames  []string

	mu sync.RWMutex
	// metrics stores histogram data keyed by sorted label string for deduplication
	metrics map[string]*histogramData
	// labelSets stores the actual labels for each key
	labelSets map[string]prometheus.Labels
}

// NewHistogramCollector creates a new histogram collector
func NewHistogramCollector(name, description string, labelNames []string) *HistogramCollector {
	return &HistogramCollector{
		name:        name,
		description: description,
		labelNames:  labelNames,
		metrics:     make(map[string]*histogramData),
		labelSets:   make(map[string]prometheus.Labels),
	}
}

// labelsToKey creates a deterministic string key from labels for deduplication
func labelsToKey(labels map[string]string) string {
	// Sort keys for consistent ordering
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	// Sort using simple bubble sort to avoid import
	for i := 0; i < len(keys)-1; i++ {
		for j := 0; j < len(keys)-i-1; j++ {
			if keys[j] > keys[j+1] {
				keys[j], keys[j+1] = keys[j+1], keys[j]
			}
		}
	}
	// Build key string
	var result string
	for _, k := range keys {
		result += k + "=" + labels[k] + ","
	}
	return result
}

// Update updates the histogram with new metric data
func (h *HistogramCollector) Update(metric domain.Metric, labels map[string]string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Convert labels to prometheus.Labels
	promLabels := prometheus.Labels(labels)

	// Create a key for this label combination
	key := labelsToKey(labels)

	// Convert buckets to map
	buckets := make(map[float64]uint64)
	for _, bucket := range metric.HistogramData.Buckets {
		buckets[bucket.UpperBound] = bucket.Count
	}

	// Store or update the histogram data - this replaces any existing data for the same labels
	h.metrics[key] = &histogramData{
		count:   metric.HistogramData.Count,
		sum:     metric.HistogramData.Sum,
		buckets: buckets,
	}
	h.labelSets[key] = promLabels

	// Limit the cache size to prevent memory issues
	if len(h.metrics) > 10000 {
		// Remove oldest entries (arbitrary selection since map is unordered)
		count := 0
		for k := range h.metrics {
			if count >= 5000 {
				break
			}
			delete(h.metrics, k)
			delete(h.labelSets, k)
			count++
		}
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

	for key, data := range h.metrics {
		promLabels := h.labelSets[key]

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
			data.count,
			data.sum,
			data.buckets,
		)

		if err != nil {
			slog.Error("failed to create const histogram",
				"metric", h.name,
				"error", err)
			continue
		}

		ch <- histMetric
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
