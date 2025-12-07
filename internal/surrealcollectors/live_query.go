package surrealcollectors

import (
	"log/slog"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
)

const SubsystemLiveQuery = "live_query"

// LiveQueryInfoProvider provides live query metrics.
type LiveQueryInfoProvider interface {
	LiveQueryInfo(tableIDs []domain.TableIdentifier) ([]*domain.TableOperationMetrics, error)
}

type TableFilter interface {
	FilterTables(tables []*domain.TableInfo) []domain.TableIdentifier
}

// LiveQueryCollector collects metrics from live queries.
type LiveQueryCollector struct {
	liveQueryProvider LiveQueryInfoProvider
	tableCache        *tableInfoCache
	filter            TableFilter

	operations *prometheus.CounterVec
}

// NewLiveQueryCollector creates a new live query collector.
func NewLiveQueryCollector(
	liveQueryProvider LiveQueryInfoProvider,
	filter TableFilter,
) *LiveQueryCollector {
	return &LiveQueryCollector{
		liveQueryProvider: liveQueryProvider,
		tableCache:        getTableInfoCache(),
		filter:            filter,

		operations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: domain.Namespace,
				Subsystem: SubsystemLiveQuery,
				Name:      "operations_total",
				Help:      "Total number of operations by type (create, update, delete)",
			},
			[]string{"namespace", "database", "table", "operation", "operation_type"},
		),
	}
}

// Describe implements prometheus.Collector.
func (c *LiveQueryCollector) Describe(ch chan<- *prometheus.Desc) {
	c.operations.Describe(ch)
}

// Collect implements prometheus.Collector.
func (c *LiveQueryCollector) Collect(ch chan<- prometheus.Metric) {
	// ctx := context.Background() // TODO implement using context

	tables := c.tableCache.get()
	if len(tables) == 0 {
		slog.Debug("No tables in cache for live query monitoring")
		return
	}

	filteredTableIDs := c.filter.FilterTables(tables)
	if len(filteredTableIDs) == 0 {
		slog.Debug("No tables match filter patterns")
		return
	}

	metrics, err := c.liveQueryProvider.LiveQueryInfo(filteredTableIDs)
	if err != nil {
		slog.Error("Failed to get live query metrics", "error", err)
		return
	}

	for _, m := range metrics {
		if m.Creates > 0 {
			c.operations.With(prometheus.Labels{
				"namespace":      m.Namespace,
				"database":       m.Database,
				"table":          m.Table,
				"operation":      "create",
				"operation_type": string(m.OperationType),
			}).Add(float64(m.Creates))
		}

		if m.Updates > 0 {
			c.operations.With(prometheus.Labels{
				"namespace":      m.Namespace,
				"database":       m.Database,
				"table":          m.Table,
				"operation":      "update",
				"operation_type": string(m.OperationType),
			}).Add(float64(m.Updates))
		}

		if m.Deletes > 0 {
			c.operations.With(prometheus.Labels{
				"namespace":      m.Namespace,
				"database":       m.Database,
				"table":          m.Table,
				"operation":      "delete",
				"operation_type": string(m.OperationType),
			}).Add(float64(m.Deletes))
		}
	}

	c.operations.Collect(ch)
}
