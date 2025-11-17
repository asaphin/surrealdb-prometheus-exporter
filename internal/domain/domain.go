package domain

import (
	"fmt"
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
