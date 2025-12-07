package surrealcollectors

import (
	"context"
	"log/slog"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	SubsystemBuild  = "build"
	SubsystemInfo   = "info"
	SubsystemSystem = "system"
)

type VersionReader interface {
	Version(ctx context.Context) (string, error)
}

type InfoMetricsReader interface {
	Info(ctx context.Context) (*domain.SurrealDBInfo, error)
}

type InfoCollector struct {
	versionReader     VersionReader
	infoMetricsReader InfoMetricsReader
	constantLabels    prometheus.Labels

	tableInfoCache *tableInfoCache

	versionDesc *prometheus.Desc

	availableParallelismDesc *prometheus.Desc
	cpuUsageDesc             *prometheus.Desc
	loadAverageDesc          *prometheus.Desc
	memoryAllocatedDesc      *prometheus.Desc
	memoryUsageDesc          *prometheus.Desc
	memoryUsageRatioDesc     *prometheus.Desc
	physicalCoresDesc        *prometheus.Desc
	threadsDesc              *prometheus.Desc

	scrapeDurationDesc *prometheus.Desc

	rootAccessesDesc *prometheus.Desc
	rootUsersDesc    *prometheus.Desc
	nodesDesc        *prometheus.Desc

	namespaceAccessesDesc  *prometheus.Desc
	namespaceDatabasesDesc *prometheus.Desc
	namespaceUsersDesc     *prometheus.Desc

	databaseAccessesDesc  *prometheus.Desc
	databaseAnalyzersDesc *prometheus.Desc
	databaseApisDesc      *prometheus.Desc
	databaseConfigsDesc   *prometheus.Desc
	databaseFunctionsDesc *prometheus.Desc
	databaseModelsDesc    *prometheus.Desc
	databaseParamsDesc    *prometheus.Desc
	databaseTablesDesc    *prometheus.Desc
	databaseUsersDesc     *prometheus.Desc

	tableEventsDesc  *prometheus.Desc
	tableFieldsDesc  *prometheus.Desc
	tableIndexesDesc *prometheus.Desc
	tableLivesDesc   *prometheus.Desc
	tableTablesDesc  *prometheus.Desc

	indexBuildingDesc        *prometheus.Desc
	indexBuildingInitialDesc *prometheus.Desc
	indexBuildingPendingDesc *prometheus.Desc
	indexBuildingUpdatedDesc *prometheus.Desc
}

func NewInfoCollector(versionReader VersionReader, infoMetricsReader InfoMetricsReader) *InfoCollector {
	return &InfoCollector{
		versionReader:     versionReader,
		infoMetricsReader: infoMetricsReader,

		tableInfoCache: getTableInfoCache(),

		versionDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, SubsystemBuild, "info"),
			"SurrealDB build and version information",
			[]string{"version"},
			nil,
		),

		availableParallelismDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, SubsystemSystem, "available_parallelism"),
			"Available CPU parallelism for the SurrealDB instance",
			nil,
			nil,
		),

		cpuUsageDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, SubsystemSystem, "cpu_usage"),
			"Current CPU usage (0.0 to 1.0)",
			nil,
			nil,
		),

		loadAverageDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, SubsystemSystem, "load_average"),
			"System load average",
			[]string{"period"},
			nil,
		),

		memoryAllocatedDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, SubsystemSystem, "memory_allocated_bytes"),
			"Total allocated memory in bytes",
			nil,
			nil,
		),
		memoryUsageDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, SubsystemSystem, "memory_usage_bytes"),
			"Current memory usage in bytes",
			nil,
			nil,
		),

		memoryUsageRatioDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, SubsystemSystem, "memory_usage_ratio"),
			"Memory usage as ratio of allocated memory (0.0 to 1.0)",
			nil,
			nil,
		),
		physicalCoresDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, SubsystemSystem, "physical_cores"),
			"Number of physical CPU cores",
			nil,
			nil,
		),

		threadsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, SubsystemSystem, "threads"),
			"Number of threads",
			nil,
			nil,
		),

		scrapeDurationDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, SubsystemInfo, "scrape_duration_seconds"),
			"Duration of the INFO scrape in seconds",
			nil,
			nil,
		),

		rootAccessesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "root", "accesses"),
			"Number of accesses defined at root level",
			nil,
			nil,
		),

		rootUsersDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "root", "users"),
			"Number of users defined at root level",
			nil,
			nil,
		),

		nodesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "root", "nodes"),
			"Number of nodes in the deployment",
			nil,
			nil,
		),

		namespaceAccessesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "namespace", "accesses"),
			"Number of accesses defined in the namespace",
			[]string{"namespace"},
			nil,
		),

		namespaceDatabasesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "namespace", "databases"),
			"Number of databases in the namespace",
			[]string{"namespace"},
			nil,
		),

		namespaceUsersDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "namespace", "users"),
			"Number of users defined in the namespace",
			[]string{"namespace"},
			nil,
		),

		databaseAccessesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "database", "accesses"),
			"Number of accesses defined in the database",
			[]string{"namespace", "database"},
			nil,
		),

		databaseAnalyzersDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "database", "analyzers"),
			"Number of analyzers defined in the database",
			[]string{"namespace", "database"},
			nil,
		),

		databaseApisDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "database", "apis"),
			"Number of APIs defined in the database",
			[]string{"namespace", "database"},
			nil,
		),

		databaseConfigsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "database", "configs"),
			"Number of configs defined in the database",
			[]string{"namespace", "database"},
			nil,
		),

		databaseFunctionsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "database", "functions"),
			"Number of functions defined in the database",
			[]string{"namespace", "database"},
			nil,
		),
		databaseModelsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "database", "models"),
			"Number of models defined in the database",
			[]string{"namespace", "database"},
			nil,
		),

		databaseParamsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "database", "params"),
			"Number of params defined in the database",
			[]string{"namespace", "database"},
			nil,
		),

		databaseTablesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "database", "tables"),
			"Number of tables in the database",
			[]string{"namespace", "database"},
			nil,
		),

		databaseUsersDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "database", "users"),
			"Number of users defined in the database",
			[]string{"namespace", "database"},
			nil,
		),

		tableEventsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "table", "events"),
			"Number of events defined in the table",
			[]string{"namespace", "database", "table"},
			nil,
		),

		tableFieldsDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "table", "fields"),
			"Number of fields defined in the table",
			[]string{"namespace", "database", "table"},
			nil,
		),

		tableIndexesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "table", "indexes"),
			"Number of indexes defined in the table",
			[]string{"namespace", "database", "table"},
			nil,
		),

		tableLivesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "table", "lives"),
			"Number of live queries defined in the table",
			[]string{"namespace", "database", "table"},
			nil,
		),

		tableTablesDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "table", "tables"),
			"Number of sub-tables defined in the table",
			[]string{"namespace", "database", "table"},
			nil,
		),

		indexBuildingDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "index", "building"),
			"Whether the index is currently building (1) or not (0)",
			[]string{"namespace", "database", "table", "index", "status"},
			nil,
		),

		indexBuildingInitialDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "index", "building_initial"),
			"Initial count for index building process",
			[]string{"namespace", "database", "table", "index", "status"},
			nil,
		),

		indexBuildingPendingDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "index", "building_pending"),
			"Pending count for index building process",
			[]string{"namespace", "database", "table", "index", "status"},
			nil,
		),

		indexBuildingUpdatedDesc: prometheus.NewDesc(
			prometheus.BuildFQName(domain.Namespace, "index", "building_updated"),
			"Updated count for index building process",
			[]string{"namespace", "database", "table", "index", "status"},
			nil,
		),
	}
}

func (c *InfoCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.versionDesc

	ch <- c.availableParallelismDesc
	ch <- c.cpuUsageDesc
	ch <- c.loadAverageDesc
	ch <- c.memoryAllocatedDesc
	ch <- c.memoryUsageDesc
	ch <- c.memoryUsageRatioDesc
	ch <- c.physicalCoresDesc
	ch <- c.threadsDesc

	ch <- c.scrapeDurationDesc

	ch <- c.rootAccessesDesc
	ch <- c.rootUsersDesc
	ch <- c.nodesDesc

	ch <- c.namespaceAccessesDesc
	ch <- c.namespaceDatabasesDesc
	ch <- c.namespaceUsersDesc

	ch <- c.databaseAccessesDesc
	ch <- c.databaseAnalyzersDesc
	ch <- c.databaseApisDesc
	ch <- c.databaseConfigsDesc
	ch <- c.databaseFunctionsDesc
	ch <- c.databaseModelsDesc
	ch <- c.databaseParamsDesc
	ch <- c.databaseTablesDesc
	ch <- c.databaseUsersDesc

	ch <- c.tableEventsDesc
	ch <- c.tableFieldsDesc
	ch <- c.tableIndexesDesc
	ch <- c.tableLivesDesc
	ch <- c.tableTablesDesc

	ch <- c.indexBuildingDesc
	ch <- c.indexBuildingInitialDesc
	ch <- c.indexBuildingPendingDesc
	ch <- c.indexBuildingUpdatedDesc
}

func (c *InfoCollector) Collect(ch chan<- prometheus.Metric) {
	ctx := context.Background()

	c.collectVersion(ctx, ch)

	info, err := c.infoMetricsReader.Info(ctx)
	if err != nil {
		slog.Error("InfoCollector: failed to fetch server info", "error", err)
		return
	}

	c.tableInfoCache.set(info.AllTables())

	c.collectSystemMetrics(ch, info)
	c.collectScrapeDuration(ch, info)
	c.collectRootMetrics(ch, info)
	c.collectNamespaceMetrics(ch, info)
	c.collectDatabaseMetrics(ch, info)
	c.collectTableMetrics(ch, info)
	c.collectIndexMetrics(ch, info)
}

func (c *InfoCollector) collectVersion(ctx context.Context, ch chan<- prometheus.Metric) {
	version, err := c.versionReader.Version(ctx)
	if err != nil {
		slog.Error("InfoCollector: failed to fetch version info", "error", err)
		return
	}

	ch <- prometheus.MustNewConstMetric(
		c.versionDesc,
		prometheus.GaugeValue,
		1,
		version,
	)
}

func (c *InfoCollector) collectSystemMetrics(ch chan<- prometheus.Metric, info *domain.SurrealDBInfo) {
	ch <- prometheus.MustNewConstMetric(
		c.availableParallelismDesc,
		prometheus.GaugeValue,
		float64(info.System.AvailableParallelism),
	)

	ch <- prometheus.MustNewConstMetric(
		c.cpuUsageDesc,
		prometheus.GaugeValue,
		info.System.CpuUsage/100,
	)

	periods := []string{"1m", "5m", "15m"}
	for i, load := range info.System.LoadAverage {
		if i < len(periods) {
			ch <- prometheus.MustNewConstMetric(
				c.loadAverageDesc,
				prometheus.GaugeValue,
				load,
				periods[i],
			)
		}
	}

	ch <- prometheus.MustNewConstMetric(
		c.memoryAllocatedDesc,
		prometheus.GaugeValue,
		float64(info.System.MemoryAllocated),
	)

	ch <- prometheus.MustNewConstMetric(
		c.memoryUsageDesc,
		prometheus.GaugeValue,
		float64(info.System.MemoryUsage),
	)

	// TODO check this
	ch <- prometheus.MustNewConstMetric(
		c.memoryUsageRatioDesc,
		prometheus.GaugeValue,
		info.System.MemoryUsagePercent()/100.0,
	)

	ch <- prometheus.MustNewConstMetric(
		c.physicalCoresDesc,
		prometheus.GaugeValue,
		float64(info.System.PhysicalCores),
	)

	ch <- prometheus.MustNewConstMetric(
		c.threadsDesc,
		prometheus.GaugeValue,
		float64(info.System.Threads),
	)
}

func (c *InfoCollector) collectScrapeDuration(ch chan<- prometheus.Metric, info *domain.SurrealDBInfo) {
	ch <- prometheus.MustNewConstMetric(
		c.scrapeDurationDesc,
		prometheus.GaugeValue,
		info.ScrapeDuration.Seconds(),
	)
}

func (c *InfoCollector) collectRootMetrics(ch chan<- prometheus.Metric, info *domain.SurrealDBInfo) {
	ch <- prometheus.MustNewConstMetric(
		c.rootAccessesDesc,
		prometheus.GaugeValue,
		float64(info.RootAccesses),
	)

	ch <- prometheus.MustNewConstMetric(
		c.rootUsersDesc,
		prometheus.GaugeValue,
		float64(info.RootUsers),
	)

	ch <- prometheus.MustNewConstMetric(
		c.nodesDesc,
		prometheus.GaugeValue,
		float64(info.Nodes),
	)
}

func (c *InfoCollector) collectNamespaceMetrics(ch chan<- prometheus.Metric, info *domain.SurrealDBInfo) {
	for name, ns := range info.Namespaces {
		ch <- prometheus.MustNewConstMetric(
			c.namespaceAccessesDesc,
			prometheus.GaugeValue,
			float64(ns.Accesses),
			name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.namespaceDatabasesDesc,
			prometheus.GaugeValue,
			float64(ns.DatabaseCount()),
			name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.namespaceUsersDesc,
			prometheus.GaugeValue,
			float64(ns.Users),
			name,
		)
	}
}

func (c *InfoCollector) collectDatabaseMetrics(ch chan<- prometheus.Metric, info *domain.SurrealDBInfo) {
	for _, db := range info.AllDatabases() {
		ch <- prometheus.MustNewConstMetric(
			c.databaseAccessesDesc,
			prometheus.GaugeValue,
			float64(db.Accesses),
			db.Namespace, db.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.databaseAnalyzersDesc,
			prometheus.GaugeValue,
			float64(db.Analyzers),
			db.Namespace, db.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.databaseApisDesc,
			prometheus.GaugeValue,
			float64(db.Apis),
			db.Namespace, db.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.databaseConfigsDesc,
			prometheus.GaugeValue,
			float64(db.Configs),
			db.Namespace, db.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.databaseFunctionsDesc,
			prometheus.GaugeValue,
			float64(db.Functions),
			db.Namespace, db.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.databaseModelsDesc,
			prometheus.GaugeValue,
			float64(db.Models),
			db.Namespace, db.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.databaseParamsDesc,
			prometheus.GaugeValue,
			float64(db.Params),
			db.Namespace, db.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.databaseTablesDesc,
			prometheus.GaugeValue,
			float64(db.TableCount()),
			db.Namespace, db.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.databaseUsersDesc,
			prometheus.GaugeValue,
			float64(db.Users),
			db.Namespace, db.Name,
		)
	}
}

func (c *InfoCollector) collectTableMetrics(ch chan<- prometheus.Metric, info *domain.SurrealDBInfo) {
	for _, table := range info.AllTables() {
		ch <- prometheus.MustNewConstMetric(
			c.tableEventsDesc,
			prometheus.GaugeValue,
			float64(table.Events),
			table.Namespace, table.Database, table.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.tableFieldsDesc,
			prometheus.GaugeValue,
			float64(table.Fields),
			table.Namespace, table.Database, table.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.tableIndexesDesc,
			prometheus.GaugeValue,
			float64(table.IndexCount()),
			table.Namespace, table.Database, table.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.tableLivesDesc,
			prometheus.GaugeValue,
			float64(table.Lives),
			table.Namespace, table.Database, table.Name,
		)

		ch <- prometheus.MustNewConstMetric(
			c.tableTablesDesc,
			prometheus.GaugeValue,
			float64(table.Tables),
			table.Namespace, table.Database, table.Name,
		)
	}
}

func (c *InfoCollector) collectIndexMetrics(ch chan<- prometheus.Metric, info *domain.SurrealDBInfo) {
	for _, idx := range info.AllIndexes() {
		buildingValue := float64(0)
		if idx.IsBuilding() {
			buildingValue = 1
		}

		status := idx.Building.Status
		if status == "" {
			status = "none"
		}

		ch <- prometheus.MustNewConstMetric(
			c.indexBuildingDesc,
			prometheus.GaugeValue,
			buildingValue,
			idx.Namespace, idx.Database, idx.Table, idx.Name, status,
		)

		ch <- prometheus.MustNewConstMetric(
			c.indexBuildingInitialDesc,
			prometheus.GaugeValue,
			float64(idx.Building.Initial),
			idx.Namespace, idx.Database, idx.Table, idx.Name, status,
		)

		ch <- prometheus.MustNewConstMetric(
			c.indexBuildingPendingDesc,
			prometheus.GaugeValue,
			float64(idx.Building.Pending),
			idx.Namespace, idx.Database, idx.Table, idx.Name, status,
		)

		ch <- prometheus.MustNewConstMetric(
			c.indexBuildingUpdatedDesc,
			prometheus.GaugeValue,
			float64(idx.Building.Updated),
			idx.Namespace, idx.Database, idx.Table, idx.Name, status,
		)
	}
}
