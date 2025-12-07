package surrealdb

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
	sdk "github.com/surrealdb/surrealdb.go"
)

// statsRecord represents a record from the stats table.
type statsRecord struct {
	TargetTable      string    `json:"target_table"`
	CreateRelational int64     `json:"create_relational"`
	CreateKV         int64     `json:"create_kv"`
	CreateGraph      int64     `json:"create_graph"`
	CreateDocument   int64     `json:"create_document"`
	UpdateRelational int64     `json:"update_relational"`
	UpdateKV         int64     `json:"update_kv"`
	UpdateGraph      int64     `json:"update_graph"`
	UpdateDocument   int64     `json:"update_document"`
	DeleteRelational int64     `json:"delete_relational"`
	DeleteKV         int64     `json:"delete_kv"`
	DeleteGraph      int64     `json:"delete_graph"`
	DeleteDocument   int64     `json:"delete_document"`
	LastUpdate       time.Time `json:"last_update"`
}

// StatsTableManager manages side tables for collecting operation statistics.
type StatsTableManager struct {
	connManager        ConnectionManager
	removeOrphanTables bool
	sideTablePrefix    string

	activeTables map[string]*statsTableState
	mu           sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// statsTableState tracks state for a single stats table.
type statsTableState struct {
	targetTableID  domain.TableIdentifier
	statsTableName string
}

// NewStatsTableManager creates a new stats table manager.
func NewStatsTableManager(
	connManager ConnectionManager,
	removeOrphanTables bool,
	sideTablePrefix string,
) *StatsTableManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &StatsTableManager{
		connManager:        connManager,
		removeOrphanTables: removeOrphanTables,
		sideTablePrefix:    sideTablePrefix,
		activeTables:       make(map[string]*statsTableState),
		ctx:                ctx,
		cancel:             cancel,
	}
}

// StatsTableInfo returns stats from all side tables and reconciles tables.
func (m *StatsTableManager) StatsTableInfo(tableIDs []domain.TableIdentifier) ([]*domain.StatsTableData, error) {
	statsData, err := m.queryAllStatsTables(tableIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query stats tables: %w", err)
	}

	go m.reconcileTables(tableIDs)

	return statsData, nil
}

// Stop gracefully shuts down the manager.
func (m *StatsTableManager) Stop() {
	slog.Info("Stopping stats table manager")
	m.cancel()
	slog.Info("Stats table manager stopped")
}

// queryAllStatsTables queries all stats tables for the given table IDs.
func (m *StatsTableManager) queryAllStatsTables(tableIDs []domain.TableIdentifier) ([]*domain.StatsTableData, error) {
	var result []*domain.StatsTableData
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(tableIDs))

	for _, tableID := range tableIDs {
		wg.Add(1)
		go func(tid domain.TableIdentifier) {
			defer wg.Done()

			data, err := m.queryStatsTable(tid)
			if err != nil {
				errChan <- fmt.Errorf("failed to query stats for %s: %w", tid.String(), err)
				return
			}

			if data != nil {
				mu.Lock()
				result = append(result, data)
				mu.Unlock()
			}
		}(tableID)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		slog.Warn("Error querying stats table", "error", err)
	}

	return result, nil
}

// queryStatsTable queries a single stats table.
func (m *StatsTableManager) queryStatsTable(tableID domain.TableIdentifier) (*domain.StatsTableData, error) {
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	db, err := m.connManager.Get(ctx, tableID.Namespace, tableID.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	statsTableName := m.getStatsTableName(tableID.Table)

	query := fmt.Sprintf("SELECT * FROM %s LIMIT 1", statsTableName)
	results, err := sdk.Query[[]*statsRecord](ctx, db, query, nil)
	if err != nil {
		slog.Debug("Stats table query failed", "table", tableID.String(), "error", err)
		return nil, nil
	}

	if results == nil || len(*results) == 0 {
		return nil, nil
	}

	queryResult := (*results)[0]
	if queryResult.Status != "OK" {
		slog.Debug("Stats table query returned non-OK status",
			"table", tableID.String(),
			"status", queryResult.Status,
			"error", queryResult.Error)
		return nil, nil
	}

	if queryResult.Result == nil || len(queryResult.Result) == 0 {
		return nil, nil
	}

	record := queryResult.Result[0]

	data := &domain.StatsTableData{
		Namespace:        tableID.Namespace,
		Database:         tableID.Database,
		Table:            tableID.Table,
		CreateRelational: record.CreateRelational,
		CreateKV:         record.CreateKV,
		CreateGraph:      record.CreateGraph,
		CreateDocument:   record.CreateDocument,
		UpdateRelational: record.UpdateRelational,
		UpdateKV:         record.UpdateKV,
		UpdateGraph:      record.UpdateGraph,
		UpdateDocument:   record.UpdateDocument,
		DeleteRelational: record.DeleteRelational,
		DeleteKV:         record.DeleteKV,
		DeleteGraph:      record.DeleteGraph,
		DeleteDocument:   record.DeleteDocument,
		LastUpdate:       record.LastUpdate,
	}

	return data, nil
}

// reconcileTables creates new stats tables and removes orphans.
func (m *StatsTableManager) reconcileTables(desiredTables []domain.TableIdentifier) {
	m.mu.Lock()
	defer m.mu.Unlock()

	desired := make(map[string]domain.TableIdentifier)
	for _, table := range desiredTables {
		desired[table.String()] = table
	}

	if m.removeOrphanTables {
		for tableKey, state := range m.activeTables {
			if _, exists := desired[tableKey]; !exists {
				slog.Info("Removing orphan stats table", "table", tableKey)
				if err := m.removeStatsTable(state); err != nil {
					slog.Error("Failed to remove orphan stats table", "table", tableKey, "error", err)
				}
				delete(m.activeTables, tableKey)
			}
		}
	}

	for tableKey, tableID := range desired {
		if _, exists := m.activeTables[tableKey]; !exists {
			slog.Info("Creating stats table for new table", "table", tableKey)
			if err := m.createStatsTable(tableID); err != nil {
				slog.Error("Failed to create stats table", "table", tableKey, "error", err)
				continue
			}

			statsTableName := m.getStatsTableName(tableID.Table)
			m.activeTables[tableKey] = &statsTableState{
				targetTableID:  tableID,
				statsTableName: statsTableName,
			}
		}
	}
}

// createStatsTable creates a side stats table and sets up events.
func (m *StatsTableManager) createStatsTable(tableID domain.TableIdentifier) error {
	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	db, err := m.connManager.Get(ctx, tableID.Namespace, tableID.Database)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}

	statsTableName := m.getStatsTableName(tableID.Table)

	createTableQuery := fmt.Sprintf(`
	IF !record::exists(%[1]s:stats) THEN
		CREATE %[1]s:stats SET
			target_table = "%[2]s",
			create_relational = 0,
			create_kv = 0,
			create_graph = 0,
			create_document = 0,
			update_relational = 0,
			update_kv = 0,
			update_graph = 0,
			update_document = 0,
			delete_relational = 0,
			delete_kv = 0,
			delete_graph = 0,
			delete_document = 0,
			last_update = time::now()
	END;
    `, statsTableName, tableID.Table)

	results, err := sdk.Query[any](ctx, db, createTableQuery, nil)
	if err != nil {
		return fmt.Errorf("failed to create stats table: %w", err)
	}

	if results != nil && len(*results) > 0 {
		result := (*results)[0]
		if result.Status != "OK" {
			return fmt.Errorf("create stats table returned %s status: %w", result.Status, result.Error)
		}
	}

	createEventQuery := fmt.Sprintf(`
		DEFINE EVENT stats_create ON TABLE %s WHEN $event = "CREATE" THEN {
			LET $op_type = IF $after.in AND $after.out THEN "graph"
				ELSE IF $after.keys().len() <= 3 THEN "kv"
				ELSE IF $after.values().flatten().len() = $after.values().len() 
					AND $after.keys().len() >= 4 THEN "relational"
				ELSE "document"
			END;
			UPDATE %s:stats SET
				create_relational += IF $op_type = "relational" THEN 1 ELSE 0 END,
				create_kv += IF $op_type = "kv" THEN 1 ELSE 0 END,
				create_graph += IF $op_type = "graph" THEN 1 ELSE 0 END,
				create_document += IF $op_type = "document" THEN 1 ELSE 0 END,
				last_update = time::now()
		};
	`, tableID.Table, statsTableName)

	results, err = sdk.Query[any](ctx, db, createEventQuery, nil)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("failed to create CREATE event: %w", err)
	}

	if results != nil && len(*results) > 0 {
		result := (*results)[0]
		if result.Status != "OK" && !strings.Contains(result.Error.Error(), "already exists") {
			return fmt.Errorf("create CREATE event returned %s status: %w", result.Status, result.Error)
		}
	}

	updateEventQuery := fmt.Sprintf(`
		DEFINE EVENT stats_update ON TABLE %s WHEN $event = "UPDATE" THEN {
			LET $op_type = IF $after.in AND $after.out THEN "graph"
				ELSE IF $after.keys().len() <= 3 THEN "kv"
				ELSE IF $after.values().flatten().len() = $after.values().len() 
					AND $after.keys().len() >= 4 THEN "relational"
				ELSE "document"
			END;
			UPDATE %s:stats SET
				update_relational += IF $op_type = "relational" THEN 1 ELSE 0 END,
				update_kv += IF $op_type = "kv" THEN 1 ELSE 0 END,
				update_graph += IF $op_type = "graph" THEN 1 ELSE 0 END,
				update_document += IF $op_type = "document" THEN 1 ELSE 0 END,
				last_update = time::now()
		};
	`, tableID.Table, statsTableName)

	results, err = sdk.Query[any](ctx, db, updateEventQuery, nil)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("failed to create UPDATE event: %w", err)
	}

	if results != nil && len(*results) > 0 {
		result := (*results)[0]
		if result.Status != "OK" && !strings.Contains(result.Error.Error(), "already exists") {
			return fmt.Errorf("create UPDATE event returned %s status: %w", result.Status, result.Error)
		}
	}

	deleteEventQuery := fmt.Sprintf(`
		DEFINE EVENT stats_delete ON TABLE %s WHEN $event = "DELETE" THEN {
			LET $op_type = IF $before.in AND $before.out THEN "graph"
				ELSE IF $before.keys().len() <= 3 THEN "kv"
				ELSE IF $before.values().flatten().len() = $before.values().len() 
					AND $before.keys().len() >= 4 THEN "relational"
				ELSE "document"
			END;
			UPDATE %s:stats SET
				delete_relational += IF $op_type = "relational" THEN 1 ELSE 0 END,
				delete_kv += IF $op_type = "kv" THEN 1 ELSE 0 END,
				delete_graph += IF $op_type = "graph" THEN 1 ELSE 0 END,
				delete_document += IF $op_type = "document" THEN 1 ELSE 0 END,
				last_update = time::now()
		};
	`, tableID.Table, statsTableName)

	results, err = sdk.Query[any](ctx, db, deleteEventQuery, nil)
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		return fmt.Errorf("failed to create DELETE event: %w", err)
	}

	if results != nil && len(*results) > 0 {
		result := (*results)[0]
		if result.Status != "OK" && !strings.Contains(result.Error.Error(), "already exists") {
			return fmt.Errorf("create DELETE event returned %s status: %w", result.Status, result.Error)
		}
	}

	slog.Info("Stats table created successfully",
		"namespace", tableID.Namespace,
		"database", tableID.Database,
		"table", tableID.Table,
		"stats_table", statsTableName)

	return nil
}

// removeStatsTable removes a stats table and its events.
func (m *StatsTableManager) removeStatsTable(state *statsTableState) error {
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	db, err := m.connManager.Get(ctx, state.targetTableID.Namespace, state.targetTableID.Database)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}

	eventNames := []string{"stats_create", "stats_update", "stats_delete"}
	for _, eventName := range eventNames {
		query := fmt.Sprintf("REMOVE EVENT %s ON TABLE %s", eventName, state.targetTableID.Table)
		results, err := sdk.Query[any](ctx, db, query, nil)
		if err != nil {
			slog.Warn("Failed to remove event", "event", eventName, "error", err)
		} else if results != nil && len(*results) > 0 {
			result := (*results)[0]
			if result.Status != "OK" {
				slog.Warn("Remove event returned non-OK status",
					"event", eventName,
					"status", result.Status,
					"error", result.Error)
			}
		}
	}

	query := fmt.Sprintf("DELETE %s", state.statsTableName)
	results, err := sdk.Query[any](ctx, db, query, nil)
	if err != nil {
		return fmt.Errorf("failed to remove stats table: %w", err)
	}

	if results != nil && len(*results) > 0 {
		result := (*results)[0]
		if result.Status != "OK" {
			return fmt.Errorf("delete stats table returned %s status: %w", result.Status, result.Error)
		}
	}

	slog.Info("Stats table removed", "table", state.targetTableID.String())
	return nil
}

// getStatsTableName returns the stats table name for a given table.
func (m *StatsTableManager) getStatsTableName(tableName string) string {
	return m.sideTablePrefix + tableName
}
