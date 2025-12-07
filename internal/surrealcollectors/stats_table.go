package surrealcollectors

import (
	"log/slog"
	"strings"
	"time"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
)

const SubsystemStatsTable = "stats_table"

// StatsTableInfoProvider provides stats table metrics
type StatsTableInfoProvider interface {
	StatsTableInfo(tableIDs []domain.TableIdentifier) ([]*domain.StatsTableData, error)
}

// StatsTableCollector collects metrics from side stats tables
type StatsTableCollector struct {
	statsTableProvider StatsTableInfoProvider
	tableCache         *tableInfoCache
	filter             TableFilter
	statsTablePrefix   string

	operations     *prometheus.GaugeVec
	scrapeDuration *prometheus.Desc
}

// NewStatsTableCollector creates a new stats table collector
func NewStatsTableCollector(
	statsTableProvider StatsTableInfoProvider,
	filter TableFilter,
	statsTablePrefix string,
) *StatsTableCollector {
	return &StatsTableCollector{
		statsTableProvider: statsTableProvider,
		tableCache:         getTableInfoCache(),
		filter:             filter,
		statsTablePrefix:   statsTablePrefix,

		operations: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: domain.Namespace,
				Subsystem: SubsystemStatsTable,
				Name:      "operations",
				Help:      "Total number of operations by type from side stats tables",
			},
			[]string{"namespace", "database", "table", "operation", "operation_type"},
		),
		scrapeDuration: prometheus.NewDesc(
			domain.Namespace+"_"+SubsystemStatsTable+"_scrape_duration_seconds",
			"Duration of the stats table scrape in seconds",
			nil,
			nil,
		),
	}
}

// Describe implements prometheus.Collector
func (c *StatsTableCollector) Describe(ch chan<- *prometheus.Desc) {
	c.operations.Describe(ch)
	ch <- c.scrapeDuration
}

// Collect implements prometheus.Collector
func (c *StatsTableCollector) Collect(ch chan<- prometheus.Metric) {
	startTime := time.Now()

	tables := c.tableCache.get()
	if len(tables) == 0 {
		slog.Debug("No tables in cache for stats table monitoring")

		ch <- prometheus.MustNewConstMetric(
			c.scrapeDuration,
			prometheus.GaugeValue,
			time.Since(startTime).Seconds(),
		)
		return
	}

	var nonStatsTables []*domain.TableInfo
	for _, table := range tables {
		if !strings.HasPrefix(table.Name, c.statsTablePrefix) {
			nonStatsTables = append(nonStatsTables, table)
		}
	}

	if len(nonStatsTables) == 0 {
		slog.Debug("No non-stats tables found for stats table monitoring")
		ch <- prometheus.MustNewConstMetric(
			c.scrapeDuration,
			prometheus.GaugeValue,
			time.Since(startTime).Seconds(),
		)
		return
	}

	filteredTableIDs := c.filter.FilterTables(nonStatsTables)
	if len(filteredTableIDs) == 0 {
		slog.Debug("No tables match filter patterns for stats table")

		ch <- prometheus.MustNewConstMetric(
			c.scrapeDuration,
			prometheus.GaugeValue,
			time.Since(startTime).Seconds(),
		)
		return
	}

	statsData, err := c.statsTableProvider.StatsTableInfo(filteredTableIDs)
	if err != nil {
		slog.Error("Failed to get stats table metrics", "error", err)

		ch <- prometheus.MustNewConstMetric(
			c.scrapeDuration,
			prometheus.GaugeValue,
			time.Since(startTime).Seconds(),
		)
		return
	}

	for _, data := range statsData {
		c.operations.With(prometheus.Labels{
			"namespace":      data.Namespace,
			"database":       data.Database,
			"table":          data.Table,
			"operation":      "create",
			"operation_type": string(domain.OperationTypeRelational),
		}).Set(float64(data.CreateRelational))

		c.operations.With(prometheus.Labels{
			"namespace":      data.Namespace,
			"database":       data.Database,
			"table":          data.Table,
			"operation":      "create",
			"operation_type": string(domain.OperationTypeKeyValue),
		}).Set(float64(data.CreateKV))

		c.operations.With(prometheus.Labels{
			"namespace":      data.Namespace,
			"database":       data.Database,
			"table":          data.Table,
			"operation":      "create",
			"operation_type": string(domain.OperationTypeGraph),
		}).Set(float64(data.CreateGraph))

		c.operations.With(prometheus.Labels{
			"namespace":      data.Namespace,
			"database":       data.Database,
			"table":          data.Table,
			"operation":      "create",
			"operation_type": string(domain.OperationTypeDocument),
		}).Set(float64(data.CreateDocument))

		c.operations.With(prometheus.Labels{
			"namespace":      data.Namespace,
			"database":       data.Database,
			"table":          data.Table,
			"operation":      "update",
			"operation_type": string(domain.OperationTypeRelational),
		}).Set(float64(data.UpdateRelational))

		c.operations.With(prometheus.Labels{
			"namespace":      data.Namespace,
			"database":       data.Database,
			"table":          data.Table,
			"operation":      "update",
			"operation_type": string(domain.OperationTypeKeyValue),
		}).Set(float64(data.UpdateKV))

		c.operations.With(prometheus.Labels{
			"namespace":      data.Namespace,
			"database":       data.Database,
			"table":          data.Table,
			"operation":      "update",
			"operation_type": string(domain.OperationTypeGraph),
		}).Set(float64(data.UpdateGraph))

		c.operations.With(prometheus.Labels{
			"namespace":      data.Namespace,
			"database":       data.Database,
			"table":          data.Table,
			"operation":      "update",
			"operation_type": string(domain.OperationTypeDocument),
		}).Set(float64(data.UpdateDocument))

		c.operations.With(prometheus.Labels{
			"namespace":      data.Namespace,
			"database":       data.Database,
			"table":          data.Table,
			"operation":      "delete",
			"operation_type": string(domain.OperationTypeRelational),
		}).Set(float64(data.DeleteRelational))

		c.operations.With(prometheus.Labels{
			"namespace":      data.Namespace,
			"database":       data.Database,
			"table":          data.Table,
			"operation":      "delete",
			"operation_type": string(domain.OperationTypeKeyValue),
		}).Set(float64(data.DeleteKV))

		c.operations.With(prometheus.Labels{
			"namespace":      data.Namespace,
			"database":       data.Database,
			"table":          data.Table,
			"operation":      "delete",
			"operation_type": string(domain.OperationTypeGraph),
		}).Set(float64(data.DeleteGraph))

		c.operations.With(prometheus.Labels{
			"namespace":      data.Namespace,
			"database":       data.Database,
			"table":          data.Table,
			"operation":      "delete",
			"operation_type": string(domain.OperationTypeDocument),
		}).Set(float64(data.DeleteDocument))
	}

	c.operations.Collect(ch)

	ch <- prometheus.MustNewConstMetric(
		c.scrapeDuration,
		prometheus.GaugeValue,
		time.Since(startTime).Seconds(),
	)
}
