package domain

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

const Namespace = "surrealdb"

// SurrealDBInfo represents the complete hierarchical information about a SurrealDB instance
type SurrealDBInfo struct {
	System         SystemMetrics
	Namespaces     map[string]*NamespaceInfo
	RootUsers      int
	RootAccesses   int
	Nodes          int
	ScrapeDuration time.Duration
}

// SystemMetrics contains system-level performance metrics
type SystemMetrics struct {
	AvailableParallelism int
	CpuUsage             float64
	LoadAverage          []float64
	MemoryAllocated      int64
	MemoryUsage          int64
	PhysicalCores        int
	Threads              int
}

// NamespaceInfo contains information about a single namespace
type NamespaceInfo struct {
	Name      string
	Databases map[string]*DatabaseInfo
	Users     int
	Accesses  int
}

// DatabaseInfo contains information about a single database
type DatabaseInfo struct {
	Name      string
	Namespace string
	Tables    map[string]*TableInfo
	Users     int
	Accesses  int
	Analyzers int
	Apis      int
	Configs   int
	Functions int
	Models    int
	Params    int
}

// TableInfo contains information about a single table
type TableInfo struct {
	Name      string
	Database  string
	Namespace string
	Indexes   map[string]*IndexInfo
	Events    int
	Fields    int
	Lives     int
	Tables    int
}

// IndexInfo contains information about a single index
type IndexInfo struct {
	Name      string
	Table     string
	Database  string
	Namespace string
	Building  IndexBuildingMetrics
}

// IndexBuildingMetrics contains index building status metrics
type IndexBuildingMetrics struct {
	Initial int
	Pending int
	Status  string
	Updated int
}

// TableRecordCount contains table record count metric
type TableRecordCount struct {
	Name        string
	Database    string
	Namespace   string
	RecordCount int
}

// RecordCountMetrics contains the complete set of record count metrics
type RecordCountMetrics struct {
	Tables         []*TableRecordCount
	ScrapeDuration time.Duration
}

// TotalNamespaces returns the total number of namespaces
func (i *SurrealDBInfo) TotalNamespaces() int {
	return len(i.Namespaces)
}

// TotalDatabases returns the total number of databases across all namespaces
func (i *SurrealDBInfo) TotalDatabases() int {
	total := 0
	for _, ns := range i.Namespaces {
		total += len(ns.Databases)
	}
	return total
}

// TotalTables returns the total number of tables across all databases
func (i *SurrealDBInfo) TotalTables() int {
	total := 0
	for _, ns := range i.Namespaces {
		for _, db := range ns.Databases {
			total += len(db.Tables)
		}
	}
	return total
}

// TotalIndexes returns the total number of indexes across all tables
func (i *SurrealDBInfo) TotalIndexes() int {
	total := 0
	for _, ns := range i.Namespaces {
		for _, db := range ns.Databases {
			for _, table := range db.Tables {
				total += len(table.Indexes)
			}
		}
	}
	return total
}

// TotalUsers returns the total number of users at all levels
func (i *SurrealDBInfo) TotalUsers() int {
	total := i.RootUsers
	for _, ns := range i.Namespaces {
		total += ns.Users
		for _, db := range ns.Databases {
			total += db.Users
		}
	}
	return total
}

// TotalAccesses returns the total number of accesses at all levels
func (i *SurrealDBInfo) TotalAccesses() int {
	total := i.RootAccesses
	for _, ns := range i.Namespaces {
		total += ns.Accesses
		for _, db := range ns.Databases {
			total += db.Accesses
		}
	}
	return total
}

// BuildingIndexes returns all indexes that are currently building
func (i *SurrealDBInfo) BuildingIndexes() []*IndexInfo {
	var building []*IndexInfo
	for _, ns := range i.Namespaces {
		for _, db := range ns.Databases {
			for _, table := range db.Tables {
				for _, idx := range table.Indexes {
					if idx.Building.Status == "building" || idx.Building.Pending > 0 {
						building = append(building, idx)
					}
				}
			}
		}
	}
	return building
}

// AllDatabases returns a flat list of all databases with their hierarchical context preserved
func (i *SurrealDBInfo) AllDatabases() []*DatabaseInfo {
	var databases []*DatabaseInfo
	for _, ns := range i.Namespaces {
		for _, db := range ns.Databases {
			databases = append(databases, db)
		}
	}
	return databases
}

// AllTables returns a flat list of all tables with their hierarchical context preserved
func (i *SurrealDBInfo) AllTables() []*TableInfo {
	var tables []*TableInfo
	for _, ns := range i.Namespaces {
		for _, db := range ns.Databases {
			for _, table := range db.Tables {
				tables = append(tables, table)
			}
		}
	}
	return tables
}

// AllIndexes returns a flat list of all indexes with their hierarchical context preserved
func (i *SurrealDBInfo) AllIndexes() []*IndexInfo {
	var indexes []*IndexInfo
	for _, ns := range i.Namespaces {
		for _, db := range ns.Databases {
			for _, table := range db.Tables {
				for _, idx := range table.Indexes {
					indexes = append(indexes, idx)
				}
			}
		}
	}
	return indexes
}

// DatabasesByNamespace returns databases grouped by namespace name for metric labeling
func (i *SurrealDBInfo) DatabasesByNamespace() map[string][]*DatabaseInfo {
	result := make(map[string][]*DatabaseInfo)
	for nsName, ns := range i.Namespaces {
		databases := make([]*DatabaseInfo, 0, len(ns.Databases))
		for _, db := range ns.Databases {
			databases = append(databases, db)
		}
		result[nsName] = databases
	}
	return result
}

// TablesByNamespace returns tables grouped by namespace name for metric labeling
func (i *SurrealDBInfo) TablesByNamespace() map[string][]*TableInfo {
	result := make(map[string][]*TableInfo)
	for nsName, ns := range i.Namespaces {
		var tables []*TableInfo
		for _, db := range ns.Databases {
			for _, table := range db.Tables {
				tables = append(tables, table)
			}
		}
		result[nsName] = tables
	}
	return result
}

// IndexesByNamespace returns indexes grouped by namespace name for metric labeling
func (i *SurrealDBInfo) IndexesByNamespace() map[string][]*IndexInfo {
	result := make(map[string][]*IndexInfo)
	for nsName, ns := range i.Namespaces {
		var indexes []*IndexInfo
		for _, db := range ns.Databases {
			for _, table := range db.Tables {
				for _, idx := range table.Indexes {
					indexes = append(indexes, idx)
				}
			}
		}
		result[nsName] = indexes
	}
	return result
}

// DatabaseCount returns the count of databases for a specific namespace
func (ns *NamespaceInfo) DatabaseCount() int {
	return len(ns.Databases)
}

// TableCount returns the count of tables across all databases in this namespace
func (ns *NamespaceInfo) TableCount() int {
	total := 0
	for _, db := range ns.Databases {
		total += len(db.Tables)
	}
	return total
}

// IndexCount returns the count of indexes across all tables in this namespace
func (ns *NamespaceInfo) IndexCount() int {
	total := 0
	for _, db := range ns.Databases {
		for _, table := range db.Tables {
			total += len(table.Indexes)
		}
	}
	return total
}

// AllTables returns all tables in this namespace as a flat list
func (ns *NamespaceInfo) AllTables() []*TableInfo {
	var tables []*TableInfo
	for _, db := range ns.Databases {
		for _, table := range db.Tables {
			tables = append(tables, table)
		}
	}
	return tables
}

// AllIndexes returns all indexes in this namespace as a flat list
func (ns *NamespaceInfo) AllIndexes() []*IndexInfo {
	var indexes []*IndexInfo
	for _, db := range ns.Databases {
		for _, table := range db.Tables {
			for _, idx := range table.Indexes {
				indexes = append(indexes, idx)
			}
		}
	}
	return indexes
}

// TableCount returns the count of tables in this database
func (db *DatabaseInfo) TableCount() int {
	return len(db.Tables)
}

// IndexCount returns the count of indexes across all tables in this database
func (db *DatabaseInfo) IndexCount() int {
	total := 0
	for _, table := range db.Tables {
		total += len(table.Indexes)
	}
	return total
}

// AllIndexes returns all indexes in this database as a flat list
func (db *DatabaseInfo) AllIndexes() []*IndexInfo {
	var indexes []*IndexInfo
	for _, table := range db.Tables {
		for _, idx := range table.Indexes {
			indexes = append(indexes, idx)
		}
	}
	return indexes
}

// IndexCount returns the count of indexes in this table
func (t *TableInfo) IndexCount() int {
	return len(t.Indexes)
}

// BuildingIndexes returns indexes that are currently building for this table
func (t *TableInfo) BuildingIndexes() []*IndexInfo {
	var building []*IndexInfo
	for _, idx := range t.Indexes {
		if idx.Building.Status == "building" || idx.Building.Pending > 0 {
			building = append(building, idx)
		}
	}
	return building
}

// IsBuilding returns true if this index is currently building
func (idx *IndexInfo) IsBuilding() bool {
	return idx.Building.Status == "building" || idx.Building.Pending > 0
}

// FullPath returns the complete hierarchical path for this index
func (idx *IndexInfo) FullPath() string {
	return fmt.Sprintf("%s.%s.%s.%s", idx.Namespace, idx.Database, idx.Table, idx.Name)
}

// FullPath returns the complete hierarchical path for this table
func (t *TableInfo) FullPath() string {
	return fmt.Sprintf("%s.%s.%s", t.Namespace, t.Database, t.Name)
}

// FullPath returns the complete hierarchical path for this database
func (db *DatabaseInfo) FullPath() string {
	return fmt.Sprintf("%s.%s", db.Namespace, db.Name)
}

// MemoryUsagePercent returns memory usage as a percentage of allocated memory
func (m *SystemMetrics) MemoryUsagePercent() float64 { // TODO check this
	if m.MemoryAllocated == 0 {
		return 0
	}
	return (float64(m.MemoryUsage) / float64(m.MemoryAllocated)) * 100
}

// Namespace retrieves a specific namespace by name
func (i *SurrealDBInfo) Namespace(name string) (*NamespaceInfo, bool) {
	ns, exists := i.Namespaces[name]
	return ns, exists
}

// Database retrieves a specific database by namespace and database name
func (i *SurrealDBInfo) Database(namespace, database string) (*DatabaseInfo, bool) {
	ns, exists := i.Namespaces[namespace]
	if !exists {
		return nil, false
	}
	db, exists := ns.Databases[database]
	return db, exists
}

// Table retrieves a specific table by namespace, database, and table name
func (i *SurrealDBInfo) Table(namespace, database, table string) (*TableInfo, bool) {
	db, exists := i.Database(namespace, database)
	if !exists {
		return nil, false
	}
	tbl, exists := db.Tables[table]
	return tbl, exists
}

// TableIdentifier represents a unique table reference
type TableIdentifier struct {
	Namespace string
	Database  string
	Table     string
}

// String returns the colon-separated identifier
func (t TableIdentifier) String() string {
	return t.Namespace + ":" + t.Database + ":" + t.Table
}

// ParseTableIdentifier parses a colon-separated table identifier
func ParseTableIdentifier(s string) (TableIdentifier, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return TableIdentifier{}, fmt.Errorf("invalid table identifier: %s", s)
	}
	return TableIdentifier{
		Namespace: parts[0],
		Database:  parts[1],
		Table:     parts[2],
	}, nil
}

// OperationType represents the data model type detected from actual data
type OperationType string

const (
	OperationTypeGraph      OperationType = "graph"
	OperationTypeRelational OperationType = "relational"
	OperationTypeKeyValue   OperationType = "key_value"
	OperationTypeDocument   OperationType = "document"
	OperationTypeUnknown    OperationType = "unknown"
)

// OperationAction represents the type of database operation
type OperationAction string

const (
	ActionCreate  OperationAction = "CREATE"
	ActionUpdate  OperationAction = "UPDATE"
	ActionDelete  OperationAction = "DELETE"
	ActionUnknown OperationAction = "UNKNOWN"
)

// TableOperationMetrics contains operation counts for a specific table and type
type TableOperationMetrics struct {
	Namespace     string
	Database      string
	Table         string
	OperationType OperationType
	Creates       int64
	Updates       int64
	Deletes       int64
}

// LiveQueryMetrics contains all accumulated metrics
type LiveQueryMetrics struct {
	Tables    map[string]*TableOperationMetrics // key: tableID:operationType
	Timestamp time.Time
}

// Key returns a unique key for this metric
func (t *TableOperationMetrics) Key() string {
	return fmt.Sprintf("%s:%s:%s:%s", t.Namespace, t.Database, t.Table, t.OperationType)
}

// StatsTableData contains operation counts from a side stats table for a specific table
type StatsTableData struct {
	Namespace        string
	Database         string
	Table            string
	CreateRelational int64
	CreateKV         int64
	CreateGraph      int64
	CreateDocument   int64
	UpdateRelational int64
	UpdateKV         int64
	UpdateGraph      int64
	UpdateDocument   int64
	DeleteRelational int64
	DeleteKV         int64
	DeleteGraph      int64
	DeleteDocument   int64
	LastUpdate       time.Time
}

// OTel related structures

// MetricType represents different Prometheus metric types
type MetricType int

const (
	MetricTypeUnknown MetricType = iota
	MetricTypeGauge
	MetricTypeCounter
	MetricTypeHistogram
	MetricTypeSummary
)

// String returns the string representation of MetricType
func (m MetricType) String() string {
	switch m {
	case MetricTypeGauge:
		return "gauge"
	case MetricTypeCounter:
		return "counter"
	case MetricTypeHistogram:
		return "histogram"
	case MetricTypeSummary:
		return "summary"
	default:
		return "unknown"
	}
}

// Metric represents a domain metric independent of OTLP or Prometheus format
type Metric struct {
	Name        string
	Type        MetricType
	Value       float64
	Labels      map[string]string
	Timestamp   time.Time
	Description string
	Unit        string

	// For histograms
	HistogramData *HistogramData
}

// HistogramData contains histogram-specific data with cumulative bucket counts
type HistogramData struct {
	Count       uint64
	Sum         float64
	Buckets     []HistogramBucket
	CreatedTime time.Time
}

// HistogramBucket represents a single histogram bucket with cumulative count
type HistogramBucket struct {
	UpperBound float64 // +Inf for the last bucket
	Count      uint64  // Cumulative count up to this boundary
}

// MetricBatch represents a collection of metrics received together
type MetricBatch struct {
	Metrics       []Metric
	ReceivedAt    time.Time
	ResourceAttrs map[string]string
}

// NewMetric creates a new metric with validation
func NewMetric(name string, metricType MetricType) (*Metric, error) {
	if name == "" {
		return nil, errors.New("metric name cannot be empty")
	}

	return &Metric{
		Name:   name,
		Type:   metricType,
		Labels: make(map[string]string),
	}, nil
}

// AddLabel adds a label to the metric
func (m *Metric) AddLabel(key, value string) {
	if m.Labels == nil {
		m.Labels = make(map[string]string)
	}
	m.Labels[key] = value
}

// IsValid checks if the metric has required fields
func (m *Metric) IsValid() bool {
	return m.Name != "" && m.Type != MetricTypeUnknown
}

// HasHistogramData returns true if this metric has histogram data
func (m *Metric) HasHistogramData() bool {
	return m.Type == MetricTypeHistogram && m.HistogramData != nil
}

var invalidLabelCharRegex = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// SanitizeLabelName converts OTEL attribute names to valid Prometheus label names
// Replaces invalid characters with underscores and ensures the name doesn't start with a number
func SanitizeLabelName(name string) string {
	// Replace dots and invalid characters with underscores
	sanitized := invalidLabelCharRegex.ReplaceAllString(name, "_")

	// Ensure it doesn't start with a number
	if len(sanitized) > 0 && sanitized[0] >= '0' && sanitized[0] <= '9' {
		sanitized = "_" + sanitized
	}

	return sanitized
}

// SanitizeMetricName converts OTEL metric names to Prometheus naming conventions
func SanitizeMetricName(name string, strategy string) string {
	switch strategy {
	case "UnderscoreEscapingWithSuffixes":
		return underscoreEscaping(name)
	case "NoTranslation":
		return name
	default:
		return underscoreEscaping(name)
	}
}

// underscoreEscaping replaces dots and invalid characters with underscores
func underscoreEscaping(name string) string {
	// Replace dots with underscores
	name = strings.ReplaceAll(name, ".", "_")

	// Replace other invalid characters
	name = invalidLabelCharRegex.ReplaceAllString(name, "_")

	return name
}

// UnitConversion defines how to convert a unit to Prometheus base units
type UnitConversion struct {
	TargetUnit string  // The Prometheus base unit name
	Multiplier float64 // Multiply value by this to convert
}

// unitConversions maps OTLP units to Prometheus base units
// Following Prometheus naming conventions: https://prometheus.io/docs/practices/naming/
var unitConversions = map[string]UnitConversion{
	// Time units -> seconds
	"ms":           {TargetUnit: "seconds", Multiplier: 0.001},
	"milliseconds": {TargetUnit: "seconds", Multiplier: 0.001},
	"us":           {TargetUnit: "seconds", Multiplier: 0.000001},
	"microseconds": {TargetUnit: "seconds", Multiplier: 0.000001},
	"ns":           {TargetUnit: "seconds", Multiplier: 0.000000001},
	"nanoseconds":  {TargetUnit: "seconds", Multiplier: 0.000000001},
	"s":            {TargetUnit: "seconds", Multiplier: 1},
	"seconds":      {TargetUnit: "seconds", Multiplier: 1},
	"m":            {TargetUnit: "seconds", Multiplier: 60},
	"minutes":      {TargetUnit: "seconds", Multiplier: 60},
	"h":            {TargetUnit: "seconds", Multiplier: 3600},
	"hours":        {TargetUnit: "seconds", Multiplier: 3600},

	// Size units -> bytes
	"By":        {TargetUnit: "bytes", Multiplier: 1},
	"bytes":     {TargetUnit: "bytes", Multiplier: 1},
	"b":         {TargetUnit: "bytes", Multiplier: 1},
	"kb":        {TargetUnit: "bytes", Multiplier: 1024},
	"kilobytes": {TargetUnit: "bytes", Multiplier: 1024},
	"KiBy":      {TargetUnit: "bytes", Multiplier: 1024},
	"mb":        {TargetUnit: "bytes", Multiplier: 1024 * 1024},
	"megabytes": {TargetUnit: "bytes", Multiplier: 1024 * 1024},
	"MiBy":      {TargetUnit: "bytes", Multiplier: 1024 * 1024},
	"gb":        {TargetUnit: "bytes", Multiplier: 1024 * 1024 * 1024},
	"gigabytes": {TargetUnit: "bytes", Multiplier: 1024 * 1024 * 1024},
	"GiBy":      {TargetUnit: "bytes", Multiplier: 1024 * 1024 * 1024},

	// Ratio/percentage units
	"1":       {TargetUnit: "ratio", Multiplier: 1},
	"ratio":   {TargetUnit: "ratio", Multiplier: 1},
	"%":       {TargetUnit: "ratio", Multiplier: 0.01},
	"percent": {TargetUnit: "ratio", Multiplier: 0.01},
}

// metricsAlreadyInBaseUnits lists OTEL metric name patterns that are known to already
// be in Prometheus base units according to OTEL semantic conventions, regardless of
// what unit label the source may incorrectly send.
// Reference: https://opentelemetry.io/docs/specs/semconv/http/http-metrics/
var metricsAlreadyInBaseUnits = map[string]string{
	// HTTP metrics - OTEL specifies these are in bytes
	"http.server.request.size":  "bytes",
	"http.server.response.size": "bytes",
	"http.client.request.size":  "bytes",
	"http.client.response.size": "bytes",
	// RPC metrics - OTEL specifies these are in bytes
	"rpc.server.request.size":  "bytes",
	"rpc.server.response.size": "bytes",
	"rpc.client.request.size":  "bytes",
	"rpc.client.response.size": "bytes",
}

// GetEffectiveUnit returns the correct unit for a metric, handling cases where
// the source sends an incorrect unit label. For known OTEL metrics, we use
// the unit specified by OTEL semantic conventions instead of the declared unit.
func GetEffectiveUnit(metricName, declaredUnit string) string {
	// Check if this metric has a known correct unit per OTEL conventions
	if correctUnit, ok := metricsAlreadyInBaseUnits[metricName]; ok {
		return correctUnit
	}
	return declaredUnit
}

// GetUnitConversion returns the conversion for a given unit, or nil if no conversion needed
func GetUnitConversion(unit string) *UnitConversion {
	if conv, ok := unitConversions[strings.ToLower(unit)]; ok {
		return &conv
	}
	return nil
}

// GetUnitConversionForMetric returns the conversion for a metric, taking into account
// that some metrics may have incorrectly labeled units that should be overridden
func GetUnitConversionForMetric(metricName, declaredUnit string) *UnitConversion {
	effectiveUnit := GetEffectiveUnit(metricName, declaredUnit)
	return GetUnitConversion(effectiveUnit)
}

// ConvertValue converts a value from the source unit to the Prometheus base unit
func ConvertValue(value float64, unit string) float64 {
	if conv := GetUnitConversion(unit); conv != nil {
		return value * conv.Multiplier
	}
	return value
}

// ConvertValueForMetric converts a value taking into account metric-specific unit corrections
func ConvertValueForMetric(value float64, metricName, declaredUnit string) float64 {
	if conv := GetUnitConversionForMetric(metricName, declaredUnit); conv != nil {
		return value * conv.Multiplier
	}
	return value
}

// GetTargetUnit returns the Prometheus base unit for a given OTLP unit
func GetTargetUnit(unit string) string {
	if conv := GetUnitConversion(unit); conv != nil {
		return conv.TargetUnit
	}
	return unit
}

// GetTargetUnitForMetric returns the Prometheus base unit, handling metric-specific corrections
func GetTargetUnitForMetric(metricName, declaredUnit string) string {
	effectiveUnit := GetEffectiveUnit(metricName, declaredUnit)
	if conv := GetUnitConversion(effectiveUnit); conv != nil {
		return conv.TargetUnit
	}
	return effectiveUnit
}

// AddSuffixByType adds appropriate Prometheus suffix based on metric type and unit
// It converts units to Prometheus base units (e.g., ms -> seconds, mb -> bytes)
func AddSuffixByType(name string, metricType MetricType, unit string) string {
	// Convert to Prometheus base unit
	targetUnit := GetTargetUnit(unit)

	// Add unit suffix if present and not already included
	if targetUnit != "" && !strings.Contains(name, targetUnit) {
		name = name + "_" + targetUnit
	}

	// Add type suffix for counters
	switch metricType {
	case MetricTypeCounter:
		if !strings.HasSuffix(name, "_total") {
			name = name + "_total"
		}
	}

	return name
}

// AddSuffixByTypeForMetric is like AddSuffixByType but handles metric-specific unit corrections
func AddSuffixByTypeForMetric(name, originalMetricName string, metricType MetricType, declaredUnit string) string {
	// Get the correct target unit considering metric-specific overrides
	targetUnit := GetTargetUnitForMetric(originalMetricName, declaredUnit)

	// Add unit suffix if present and not already included
	if targetUnit != "" && !strings.Contains(name, targetUnit) {
		name = name + "_" + targetUnit
	}

	// Add type suffix for counters
	switch metricType {
	case MetricTypeCounter:
		if !strings.HasSuffix(name, "_total") {
			name = name + "_total"
		}
	}

	return name
}

// BucketsFromBounds creates histogram buckets from explicit bounds with cumulative counts
func BucketsFromBounds(bounds []float64, counts []uint64) []HistogramBucket {
	buckets := make([]HistogramBucket, 0, len(bounds)+1)

	// Create buckets for each bound
	for i := 0; i < len(bounds) && i < len(counts); i++ {
		buckets = append(buckets, HistogramBucket{
			UpperBound: bounds[i],
			Count:      counts[i],
		})
	}

	// Add +Inf bucket if counts has one more element than bounds
	if len(counts) > len(bounds) {
		buckets = append(buckets, HistogramBucket{
			UpperBound: math.Inf(1),
			Count:      counts[len(counts)-1],
		})
	}

	return buckets
}

// MetricsByType groups metrics by their type
func (mb *MetricBatch) MetricsByType() map[MetricType][]Metric {
	result := make(map[MetricType][]Metric)

	for _, metric := range mb.Metrics {
		result[metric.Type] = append(result[metric.Type], metric)
	}

	return result
}

// Count returns the number of metrics in the batch
func (mb *MetricBatch) Count() int {
	return len(mb.Metrics)
}

// AddMetric adds a metric to the batch
func (mb *MetricBatch) AddMetric(metric Metric) {
	mb.Metrics = append(mb.Metrics, metric)
}

// Filter filters metrics by a predicate function
func (mb *MetricBatch) Filter(predicate func(Metric) bool) *MetricBatch {
	filtered := &MetricBatch{
		ReceivedAt:    mb.ReceivedAt,
		ResourceAttrs: mb.ResourceAttrs,
		Metrics:       make([]Metric, 0),
	}

	for _, metric := range mb.Metrics {
		if predicate(metric) {
			filtered.Metrics = append(filtered.Metrics, metric)
		}
	}

	return filtered
}
