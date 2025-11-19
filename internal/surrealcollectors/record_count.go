package surrealcollectors

import (
	"context"
	"log"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
)

// RecordCountReader defines the interface for reading table record counts
type RecordCountReader interface {
	RecordCount(ctx context.Context, tables []*domain.TableInfo) (*domain.RecordCountMetrics, error)
}

// recordCountCollector collects metrics about table record counts
type recordCountCollector struct {
	reader RecordCountReader

	tableInfoCache *tableInfoCache

	// Metrics
	tableRecordCount *prometheus.Desc
	scrapeDuration   *prometheus.Desc
}

// NewRecordCountCollector creates a new record count collector
func NewRecordCountCollector(reader RecordCountReader) prometheus.Collector {
	return &recordCountCollector{
		reader:         reader,
		tableInfoCache: getTableInfoCache(),
		tableRecordCount: prometheus.NewDesc(
			"surrealdb_table_record_count",
			"Number of records in a table",
			[]string{"namespace", "database", "table"},
			nil,
		),
		scrapeDuration: prometheus.NewDesc(
			"surrealdb_record_count_scrape_duration_seconds",
			"Duration of the record count scrape in seconds",
			nil,
			nil,
		),
	}
}

// Describe implements prometheus.Collector
func (c *recordCountCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.tableRecordCount
	ch <- c.scrapeDuration
}

// Collect implements prometheus.Collector
func (c *recordCountCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	tables := c.tableInfoCache.get()

	if len(tables) == 0 {
		log.Println("No tables found to collect record counts")
		return
	}

	// Fetch record counts
	metrics, err := c.reader.RecordCount(ctx, tables)
	if err != nil {
		log.Printf("Error collecting record counts: %v", err)
		return
	}

	// Export table record count metrics
	for _, tableCount := range metrics.Tables {
		ch <- prometheus.MustNewConstMetric(
			c.tableRecordCount,
			prometheus.GaugeValue,
			float64(tableCount.RecordCount),
			tableCount.Namespace,
			tableCount.Database,
			tableCount.Name,
		)
	}

	// Export scrape duration metric
	ch <- prometheus.MustNewConstMetric(
		c.scrapeDuration,
		prometheus.GaugeValue,
		metrics.ScrapeDuration.Seconds(),
	)
}
