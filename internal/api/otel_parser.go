package api

import (
	"fmt"
	"math"
	"time"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// ConvertPmetricToDomain converts OTLP pmetric.Metrics to domain.MetricBatch
// This function only performs parsing and conversion - no business logic
func ConvertPmetricToDomain(md pmetric.Metrics) domain.MetricBatch {
	batch := domain.MetricBatch{
		ReceivedAt:    time.Now(),
		ResourceAttrs: make(map[string]string),
		Metrics:       []domain.Metric{},
	}

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		resource := rm.Resource()

		// Extract resource attributes
		resource.Attributes().Range(func(k string, v pcommon.Value) bool {
			batch.ResourceAttrs[k] = v.AsString()
			return true
		})

		// Process scope metrics
		ilms := rm.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)

			// Process each metric
			metrics := ilm.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)

				// Convert based on metric type
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					batch.Metrics = append(batch.Metrics, convertGauge(metric)...)
				case pmetric.MetricTypeSum:
					batch.Metrics = append(batch.Metrics, convertSum(metric)...)
				case pmetric.MetricTypeHistogram:
					batch.Metrics = append(batch.Metrics, convertHistogram(metric)...)
				case pmetric.MetricTypeSummary:
					batch.Metrics = append(batch.Metrics, convertSummary(metric)...)
				}
			}
		}
	}

	return batch
}

// convertGauge converts OTLP gauge metrics to domain metrics
func convertGauge(metric pmetric.Metric) []domain.Metric {
	var metrics []domain.Metric
	gauge := metric.Gauge()

	for i := 0; i < gauge.DataPoints().Len(); i++ {
		dp := gauge.DataPoints().At(i)

		m := domain.Metric{
			Name:        metric.Name(),
			Type:        domain.MetricTypeGauge,
			Description: metric.Description(),
			Unit:        metric.Unit(),
			Labels:      extractLabels(dp.Attributes()),
			Timestamp:   dp.Timestamp().AsTime(),
		}

		// Extract value based on type
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			m.Value = dp.DoubleValue()
		case pmetric.NumberDataPointValueTypeInt:
			m.Value = float64(dp.IntValue())
		}

		metrics = append(metrics, m)
	}

	return metrics
}

// convertSum converts OTLP sum metrics to domain metrics
// Determines if it's a counter (monotonic) or gauge (non-monotonic)
func convertSum(metric pmetric.Metric) []domain.Metric {
	var metrics []domain.Metric
	sum := metric.Sum()

	for i := 0; i < sum.DataPoints().Len(); i++ {
		dp := sum.DataPoints().At(i)

		// Determine metric type based on monotonicity
		metricType := domain.MetricTypeGauge
		if sum.IsMonotonic() {
			metricType = domain.MetricTypeCounter
		}

		m := domain.Metric{
			Name:        metric.Name(),
			Type:        metricType,
			Description: metric.Description(),
			Unit:        metric.Unit(),
			Labels:      extractLabels(dp.Attributes()),
			Timestamp:   dp.Timestamp().AsTime(),
		}

		// Extract value based on type
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			m.Value = dp.DoubleValue()
		case pmetric.NumberDataPointValueTypeInt:
			m.Value = float64(dp.IntValue())
		}

		metrics = append(metrics, m)
	}

	return metrics
}

// convertHistogram converts OTLP histogram metrics to domain metrics
func convertHistogram(metric pmetric.Metric) []domain.Metric {
	var metrics []domain.Metric
	hist := metric.Histogram()

	for i := 0; i < hist.DataPoints().Len(); i++ {
		dp := hist.DataPoints().At(i)

		histData := &domain.HistogramData{
			Count:       dp.Count(),
			Sum:         dp.Sum(),
			Buckets:     make([]domain.HistogramBucket, 0),
			CreatedTime: dp.StartTimestamp().AsTime(),
		}

		// Convert buckets - OTLP uses explicit bounds with cumulative counts
		bounds := dp.ExplicitBounds()
		counts := dp.BucketCounts()

		// Create buckets for each explicit bound
		for j := 0; j < bounds.Len(); j++ {
			histData.Buckets = append(histData.Buckets, domain.HistogramBucket{
				UpperBound: bounds.At(j),
				Count:      counts.At(j),
			})
		}

		// Add +Inf bucket if counts has one more element than bounds
		if counts.Len() > bounds.Len() {
			histData.Buckets = append(histData.Buckets, domain.HistogramBucket{
				UpperBound: math.Inf(1),
				Count:      counts.At(counts.Len() - 1),
			})
		}

		m := domain.Metric{
			Name:          metric.Name(),
			Type:          domain.MetricTypeHistogram,
			Description:   metric.Description(),
			Unit:          metric.Unit(),
			Labels:        extractLabels(dp.Attributes()),
			Timestamp:     dp.Timestamp().AsTime(),
			HistogramData: histData,
		}

		metrics = append(metrics, m)
	}

	return metrics
}

// convertSummary converts OTLP summary metrics to domain metrics
func convertSummary(metric pmetric.Metric) []domain.Metric {
	var metrics []domain.Metric
	summary := metric.Summary()

	for i := 0; i < summary.DataPoints().Len(); i++ {
		dp := summary.DataPoints().At(i)

		// Create a gauge-like metric for count
		countMetric := domain.Metric{
			Name:        metric.Name() + "_count",
			Type:        domain.MetricTypeGauge,
			Description: metric.Description() + " (count)",
			Unit:        metric.Unit(),
			Labels:      extractLabels(dp.Attributes()),
			Timestamp:   dp.Timestamp().AsTime(),
			Value:       float64(dp.Count()),
		}
		metrics = append(metrics, countMetric)

		// Create a gauge-like metric for sum
		sumMetric := domain.Metric{
			Name:        metric.Name() + "_sum",
			Type:        domain.MetricTypeGauge,
			Description: metric.Description() + " (sum)",
			Unit:        metric.Unit(),
			Labels:      extractLabels(dp.Attributes()),
			Timestamp:   dp.Timestamp().AsTime(),
			Value:       dp.Sum(),
		}
		metrics = append(metrics, sumMetric)

		// Create metrics for quantiles
		quantiles := dp.QuantileValues()
		for j := 0; j < quantiles.Len(); j++ {
			qv := quantiles.At(j)
			labels := extractLabels(dp.Attributes())
			labels["quantile"] = formatFloat(qv.Quantile())

			quantileMetric := domain.Metric{
				Name:        metric.Name(),
				Type:        domain.MetricTypeGauge,
				Description: metric.Description(),
				Unit:        metric.Unit(),
				Labels:      labels,
				Timestamp:   dp.Timestamp().AsTime(),
				Value:       qv.Value(),
			}
			metrics = append(metrics, quantileMetric)
		}
	}

	return metrics
}

// extractLabels extracts labels from OTLP attributes
func extractLabels(attrs pcommon.Map) map[string]string {
	labels := make(map[string]string)
	attrs.Range(func(k string, v pcommon.Value) bool {
		labels[k] = v.AsString()
		return true
	})
	return labels
}

// formatFloat formats a float64 value for use in label values
func formatFloat(f float64) string {
	// Use %g format to avoid unnecessary trailing zeros
	return fmt.Sprintf("%g", f)
}
