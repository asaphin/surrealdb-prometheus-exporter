package surrealcollectors

import (
	"log/slog"

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

	// Prometheus gauges - these represent current values from stats tables
	operations *prometheus.GaugeVec
}

// NewStatsTableCollector creates a new stats table collector
func NewStatsTableCollector(
	statsTableProvider StatsTableInfoProvider,
	filter TableFilter,
) *StatsTableCollector {
	return &StatsTableCollector{
		statsTableProvider: statsTableProvider,
		tableCache:         getTableInfoCache(),
		filter:             filter,

		operations: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: domain.Namespace,
				Subsystem: SubsystemStatsTable,
				Name:      "operations",
				Help:      "Total number of operations by type from side stats tables",
			},
			[]string{"namespace", "database", "table", "operation", "operation_type"},
		),
	}
}

// Describe implements prometheus.Collector
func (c *StatsTableCollector) Describe(ch chan<- *prometheus.Desc) {
	c.operations.Describe(ch)
}

// Collect implements prometheus.Collector
func (c *StatsTableCollector) Collect(ch chan<- prometheus.Metric) {
	// Get tables from cache
	tables := c.tableCache.get()
	if len(tables) == 0 {
		slog.Debug("No tables in cache for stats table monitoring")
		return
	}

	// Filter tables based on config
	filteredTableIDs := c.filter.FilterTables(tables)
	if len(filteredTableIDs) == 0 {
		slog.Debug("No tables match filter patterns for stats table")
		return
	}

	// Get metrics from stats tables
	statsData, err := c.statsTableProvider.StatsTableInfo(filteredTableIDs)
	if err != nil {
		slog.Error("Failed to get stats table metrics", "error", err)
		return
	}

	// Set gauges with current values from stats tables
	for _, data := range statsData {
		// Create operations
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

		// Update operations
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

		// Delete operations
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

	// Collect the actual metric values
	c.operations.Collect(ch)
}
