package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
	sdk "github.com/surrealdb/surrealdb.go"
)

type rootInfo struct {
	Accesses   map[string]any `json:"accesses"`
	Namespaces map[string]any `json:"namespaces"`
	Nodes      map[string]any `json:"nodes"`
	System     systemInfo     `json:"system"`
	Users      map[string]any `json:"users"`
}

type systemInfo struct {
	AvailableParallelism int       `json:"available_parallelism"`
	CpuUsage             float64   `json:"cpu_usage"`
	LoadAverage          []float64 `json:"load_average"`
	MemoryAllocated      int64     `json:"memory_allocated"`
	MemoryUsage          int64     `json:"memory_usage"`
	PhysicalCores        int       `json:"physical_cores"`
	Threads              int       `json:"threads"`
}

type namespaceInfo struct {
	Accesses  map[string]any `json:"accesses"`
	Databases map[string]any `json:"databases"`
	Users     map[string]any `json:"users"`
}

type databaseInfo struct {
	Accesses  map[string]any `json:"accesses"`
	Analyzers map[string]any `json:"analyzers"`
	Apis      map[string]any `json:"apis"`
	Configs   map[string]any `json:"configs"`
	Functions map[string]any `json:"functions"`
	Models    map[string]any `json:"models"`
	Params    map[string]any `json:"params"`
	Tables    map[string]any `json:"tables"`
	Users     map[string]any `json:"users"`
}

type tableInfo struct {
	Events  map[string]any `json:"events"`
	Fields  map[string]any `json:"fields"`
	Indexes map[string]any `json:"indexes"`
	Lives   map[string]any `json:"lives"`
	Tables  map[string]any `json:"tables"`
}

type indexInfo struct {
	Building indexBuildingInfo `json:"building"`
}

type indexBuildingInfo struct {
	Initial int    `json:"initial"`
	Pending int    `json:"pending"`
	Status  string `json:"status"`
	Updated int    `json:"updated"`
}

type infoReader struct {
	cfg  Config
	conn ConnectionManager
}

func NewInfoReader(cfg Config, conn ConnectionManager) (*infoReader, error) {
	if conn == nil {
		return nil, errors.New("conn argument cannot be nil")
	}

	return &infoReader{cfg: cfg, conn: conn}, nil
}

// Info retrieves complete hierarchical information about the SurrealDB instance.
func (r *infoReader) Info(ctx context.Context) (*domain.SurrealDBInfo, error) {
	start := time.Now()

	rootData, err := r.fetchRootInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch root info: %w", err)
	}

	result := &domain.SurrealDBInfo{
		System: domain.SystemMetrics{
			AvailableParallelism: rootData.System.AvailableParallelism,
			CpuUsage:             rootData.System.CpuUsage,
			LoadAverage:          rootData.System.LoadAverage,
			MemoryAllocated:      rootData.System.MemoryAllocated,
			MemoryUsage:          rootData.System.MemoryUsage,
			PhysicalCores:        rootData.System.PhysicalCores,
			Threads:              rootData.System.Threads,
		},
		Namespaces:   make(map[string]*domain.NamespaceInfo),
		RootUsers:    len(rootData.Users),
		RootAccesses: len(rootData.Accesses),
		Nodes:        len(rootData.Nodes),
	}

	namespaceNames := make([]string, 0, len(rootData.Namespaces))
	for name := range rootData.Namespaces {
		namespaceNames = append(namespaceNames, name)
	}

	if len(namespaceNames) > 0 {
		namespaces, err := r.fetchNamespacesParallel(ctx, namespaceNames)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch namespaces: %w", err)
		}
		result.Namespaces = namespaces
	}

	result.ScrapeDuration = time.Since(start)

	return result, nil
}

// fetchRootInfo retrieves root level information.
func (r *infoReader) fetchRootInfo(ctx context.Context) (*rootInfo, error) {
	db, err := r.conn.Get(ctx, "", "")
	if err != nil {
		return nil, fmt.Errorf("could not get DB connection: %w", err)
	}

	results, err := sdk.Query[*rootInfo](ctx, db, "INFO FOR ROOT", nil)
	if err != nil {
		return nil, fmt.Errorf("INFO FOR ROOT query failed: %w", err)
	}

	if results == nil || len(*results) == 0 {
		return nil, errors.New("INFO FOR ROOT returned no results")
	}

	rootResult := (*results)[0]
	if rootResult.Status != "OK" {
		return nil, fmt.Errorf("INFO FOR ROOT returned %s status: %w", rootResult.Status, rootResult.Error)
	}

	return rootResult.Result, nil
}

// fetchNamespacesParallel retrieves multiple namespaces in parallel.
func (r *infoReader) fetchNamespacesParallel(
	ctx context.Context,
	namespaceNames []string,
) (map[string]*domain.NamespaceInfo, error) {
	type nsResult struct {
		name string
		info *domain.NamespaceInfo
		err  error
	}

	resultChan := make(chan nsResult, len(namespaceNames))
	var wg sync.WaitGroup

	for _, nsName := range namespaceNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			nsInfo, err := r.fetchNamespace(ctx, name)
			resultChan <- nsResult{name: name, info: nsInfo, err: err}
		}(nsName)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	namespaces := make(map[string]*domain.NamespaceInfo)
	var errs []error

	for result := range resultChan {
		if result.err != nil {
			errs = append(errs, fmt.Errorf("namespace %s: %w", result.name, result.err))
			continue
		}
		namespaces[result.name] = result.info
	}

	if len(errs) > 0 {
		return namespaces, fmt.Errorf("errors fetching namespaces: %v", errs)
	}

	return namespaces, nil
}

// fetchNamespace retrieves information for a single namespace and its databases.
func (r *infoReader) fetchNamespace(ctx context.Context, namespaceName string) (*domain.NamespaceInfo, error) {
	db, err := r.conn.Get(ctx, "", "")
	if err != nil {
		return nil, fmt.Errorf("could not get DB connection: %w", err)
	}

	query := fmt.Sprintf("USE NS %s; INFO FOR NS;", namespaceName)
	results, err := sdk.Query[*namespaceInfo](ctx, db, query, nil)
	if err != nil {
		return nil, fmt.Errorf("INFO FOR NAMESPACE query failed: %w", err)
	}

	if results == nil || len(*results) < 2 {
		return nil, errors.New("INFO FOR NAMESPACE returned insufficient results")
	}

	nsResult := (*results)[1]
	if nsResult.Status != "OK" {
		return nil, fmt.Errorf("INFO FOR NAMESPACE returned %s status: %w", nsResult.Status, nsResult.Error)
	}

	nsData := nsResult.Result
	nsInfo := &domain.NamespaceInfo{
		Name:      namespaceName,
		Databases: make(map[string]*domain.DatabaseInfo),
		Users:     len(nsData.Users),
		Accesses:  len(nsData.Accesses),
	}

	databaseNames := make([]string, 0, len(nsData.Databases))
	for name := range nsData.Databases {
		databaseNames = append(databaseNames, name)
	}

	if len(databaseNames) > 0 {
		databases, err := r.fetchDatabasesParallel(ctx, namespaceName, databaseNames)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch databases: %w", err)
		}
		nsInfo.Databases = databases
	}

	return nsInfo, nil
}

// fetchDatabasesParallel retrieves multiple databases in parallel.
func (r *infoReader) fetchDatabasesParallel(
	ctx context.Context,
	namespace string,
	databaseNames []string,
) (map[string]*domain.DatabaseInfo, error) {
	type dbResult struct {
		name string
		info *domain.DatabaseInfo
		err  error
	}

	resultChan := make(chan dbResult, len(databaseNames))
	var wg sync.WaitGroup

	for _, dbName := range databaseNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			dbInfo, err := r.fetchDatabase(ctx, namespace, name)
			resultChan <- dbResult{name: name, info: dbInfo, err: err}
		}(dbName)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	databases := make(map[string]*domain.DatabaseInfo)
	var errs []error

	for result := range resultChan {
		if result.err != nil {
			errs = append(errs, fmt.Errorf("database %s: %w", result.name, result.err))
			continue
		}
		databases[result.name] = result.info
	}

	if len(errs) > 0 {
		return databases, fmt.Errorf("errors fetching databases: %v", errs)
	}

	return databases, nil
}

// fetchDatabase retrieves information for a single database and its tables.
func (r *infoReader) fetchDatabase(ctx context.Context, namespace, databaseName string) (*domain.DatabaseInfo, error) {
	db, err := r.conn.Get(ctx, namespace, databaseName)
	if err != nil {
		return nil, fmt.Errorf("could not get DB connection: %w", err)
	}

	query := "INFO FOR DB"
	results, err := sdk.Query[*databaseInfo](ctx, db, query, nil)
	if err != nil {
		return nil, fmt.Errorf("INFO FOR DATABASE query failed: %w", err)
	}

	if results == nil || len(*results) == 0 {
		return nil, errors.New("INFO FOR DATABASE returned no results")
	}

	dbResult := (*results)[0]
	if dbResult.Status != "OK" {
		return nil, fmt.Errorf("INFO FOR DATABASE returned %s status: %w", dbResult.Status, dbResult.Error)
	}

	dbData := dbResult.Result
	dbInfo := &domain.DatabaseInfo{
		Name:      databaseName,
		Namespace: namespace,
		Tables:    make(map[string]*domain.TableInfo),
		Users:     len(dbData.Users),
		Accesses:  len(dbData.Accesses),
		Analyzers: len(dbData.Analyzers),
		Apis:      len(dbData.Apis),
		Configs:   len(dbData.Configs),
		Functions: len(dbData.Functions),
		Models:    len(dbData.Models),
		Params:    len(dbData.Params),
	}

	tableNames := make([]string, 0, len(dbData.Tables))
	for name := range dbData.Tables {
		if !strings.HasPrefix(name, r.cfg.StatsTableNamePrefix()) {
			tableNames = append(tableNames, name)
		}
	}

	if len(tableNames) > 0 {
		tables, err := r.fetchTablesParallel(ctx, namespace, databaseName, tableNames)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch tables: %w", err)
		}
		dbInfo.Tables = tables
	}

	return dbInfo, nil
}

// fetchTablesParallel retrieves multiple tables in parallel.
func (r *infoReader) fetchTablesParallel(
	ctx context.Context,
	namespace, database string,
	tableNames []string,
) (map[string]*domain.TableInfo, error) {
	type tblResult struct {
		name string
		info *domain.TableInfo
		err  error
	}

	resultChan := make(chan tblResult, len(tableNames))
	var wg sync.WaitGroup

	for _, tblName := range tableNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			tblInfo, err := r.fetchTable(ctx, namespace, database, name)
			resultChan <- tblResult{name: name, info: tblInfo, err: err}
		}(tblName)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	tables := make(map[string]*domain.TableInfo)
	var errs []error

	for result := range resultChan {
		if result.err != nil {
			errs = append(errs, fmt.Errorf("table %s: %w", result.name, result.err))
			continue
		}
		tables[result.name] = result.info
	}

	if len(errs) > 0 {
		return tables, fmt.Errorf("errors fetching tables: %v", errs)
	}

	return tables, nil
}

// fetchTable retrieves information for a single table and its indexes.
func (r *infoReader) fetchTable(ctx context.Context, namespace, database, tableName string) (*domain.TableInfo, error) {
	db, err := r.conn.Get(ctx, namespace, database)
	if err != nil {
		return nil, fmt.Errorf("could not get DB connection: %w", err)
	}

	query := fmt.Sprintf("INFO FOR TABLE %s", tableName)
	results, err := sdk.Query[*tableInfo](ctx, db, query, nil)
	if err != nil {
		return nil, fmt.Errorf("INFO FOR TABLE query failed: %w", err)
	}

	if results == nil || len(*results) == 0 {
		return nil, errors.New("INFO FOR TABLE returned no results")
	}

	tblResult := (*results)[0]
	if tblResult.Status != "OK" {
		return nil, fmt.Errorf("INFO FOR TABLE returned %s status: %w", tblResult.Status, tblResult.Error)
	}

	tblData := tblResult.Result
	tblInfo := &domain.TableInfo{
		Name:      tableName,
		Database:  database,
		Namespace: namespace,
		Indexes:   make(map[string]*domain.IndexInfo),
		Events:    len(tblData.Events),
		Fields:    len(tblData.Fields),
		Lives:     len(tblData.Lives),
		Tables:    len(tblData.Tables),
	}

	indexNames := make([]string, 0, len(tblData.Indexes))
	for name := range tblData.Indexes {
		indexNames = append(indexNames, name)
	}

	if len(indexNames) > 0 {
		indexes, err := r.fetchIndexesParallel(ctx, namespace, database, tableName, indexNames)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch indexes: %w", err)
		}
		tblInfo.Indexes = indexes
	}

	return tblInfo, nil
}

// fetchIndexesParallel retrieves multiple indexes in parallel.
func (r *infoReader) fetchIndexesParallel(
	ctx context.Context,
	namespace, database, table string,
	indexNames []string,
) (map[string]*domain.IndexInfo, error) {
	type idxResult struct {
		name string
		info *domain.IndexInfo
		err  error
	}

	resultChan := make(chan idxResult, len(indexNames))
	var wg sync.WaitGroup

	for _, idxName := range indexNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			idxInfo, err := r.fetchIndex(ctx, namespace, database, table, name)
			resultChan <- idxResult{name: name, info: idxInfo, err: err}
		}(idxName)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	indexes := make(map[string]*domain.IndexInfo)
	var errs []error

	for result := range resultChan {
		if result.err != nil {
			errs = append(errs, fmt.Errorf("index %s: %w", result.name, result.err))
			continue
		}
		indexes[result.name] = result.info
	}

	if len(errs) > 0 {
		return indexes, fmt.Errorf("errors fetching indexes: %v", errs)
	}

	return indexes, nil
}

// fetchIndex retrieves information for a single index.
func (r *infoReader) fetchIndex(
	ctx context.Context,
	namespace, database, table, indexName string,
) (*domain.IndexInfo, error) {
	db, err := r.conn.Get(ctx, namespace, database)
	if err != nil {
		return nil, fmt.Errorf("could not get DB connection: %w", err)
	}

	query := fmt.Sprintf("INFO FOR INDEX %s ON %s", indexName, table)
	results, err := sdk.Query[*indexInfo](ctx, db, query, nil)
	if err != nil {
		return nil, fmt.Errorf("INFO FOR INDEX query failed: %w", err)
	}

	if results == nil || len(*results) == 0 {
		return nil, errors.New("INFO FOR INDEX returned no results")
	}

	idxResult := (*results)[0]
	if idxResult.Status != "OK" {
		return nil, fmt.Errorf("INFO FOR INDEX returned %s status: %w", idxResult.Status, idxResult.Error)
	}

	idxData := idxResult.Result
	return &domain.IndexInfo{
		Name:      indexName,
		Table:     table,
		Database:  database,
		Namespace: namespace,
		Building: domain.IndexBuildingMetrics{
			Initial: idxData.Building.Initial,
			Pending: idxData.Building.Pending,
			Status:  idxData.Building.Status,
			Updated: idxData.Building.Updated,
		},
	}, nil
}
