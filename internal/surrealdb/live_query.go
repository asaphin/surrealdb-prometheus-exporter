package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
	sdk "github.com/surrealdb/surrealdb.go"
	sconn "github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// LiveQueryManager manages live queries and accumulates metrics.
type LiveQueryManager struct {
	connManager          ConnectionManager
	accumulator          *OperationAccumulator
	detector             *OperationTypeDetector
	reconnectDelay       time.Duration
	maxReconnectAttempts int

	activeQueries map[string]*liveQueryState
	mu            sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// liveQueryState tracks state for a single live query.
type liveQueryState struct {
	tableID   domain.TableIdentifier
	db        *sdk.DB
	liveID    string
	cancelCtx context.CancelFunc
}

// NewLiveQueryManager creates a new live query manager.
func NewLiveQueryManager(
	connManager ConnectionManager,
	reconnectDelay time.Duration,
	maxReconnectAttempts int,
) *LiveQueryManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &LiveQueryManager{
		connManager:          connManager,
		accumulator:          NewOperationAccumulator(),
		detector:             NewOperationTypeDetector(),
		reconnectDelay:       reconnectDelay,
		maxReconnectAttempts: maxReconnectAttempts,
		activeQueries:        make(map[string]*liveQueryState),
		ctx:                  ctx,
		cancel:               cancel,
	}
}

// LiveQueryInfo returns accumulated metrics and reconciles live queries.
// This is the main entry point called by the collector on each scrape.
func (m *LiveQueryManager) LiveQueryInfo(tableIDs []domain.TableIdentifier) ([]*domain.TableOperationMetrics, error) {
	metrics := m.accumulator.GetAndClear()

	go m.reconcileQueries(tableIDs)

	return metrics, nil
}

// Stop gracefully shuts down all live queries.
func (m *LiveQueryManager) Stop() {
	slog.Info("Stopping live query manager")
	m.cancel()
	m.wg.Wait()
	slog.Info("Live query manager stopped")
}

// reconcileQueries updates active queries to match desired table list.
func (m *LiveQueryManager) reconcileQueries(desiredTables []domain.TableIdentifier) {
	m.mu.Lock()
	defer m.mu.Unlock()

	desired := make(map[string]domain.TableIdentifier)
	for _, table := range desiredTables {
		desired[table.String()] = table
	}

	for tableKey, state := range m.activeQueries {
		if _, exists := desired[tableKey]; !exists {
			slog.Info("Stopping live query for removed table", "table", tableKey)
			state.cancelCtx()
			delete(m.activeQueries, tableKey)
		}
	}

	for tableKey, tableID := range desired {
		if _, exists := m.activeQueries[tableKey]; !exists {
			slog.Info("Starting live query for new table", "table", tableKey)
			m.wg.Add(1)
			go m.manageLiveQuery(tableID)
		}
	}
}

// manageLiveQuery manages a single live query with reconnection.
func (m *LiveQueryManager) manageLiveQuery(tableID domain.TableIdentifier) {
	defer m.wg.Done()

	attempts := 0
	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		attempts++
		if attempts > m.maxReconnectAttempts {
			slog.Error("Max reconnection attempts reached", "table", tableID.String())
			return
		}

		if attempts > 1 {
			slog.Info("Reconnecting live query", "table", tableID.String(), "attempt", attempts)
			select {
			case <-m.ctx.Done():
				return
			case <-time.After(m.reconnectDelay):
			}
		}

		if err := m.runLiveQuery(tableID); err != nil {
			slog.Error("Live query error", "table", tableID.String(), "error", err)

			if m.ctx.Err() != nil {
				m.mu.Lock()
				delete(m.activeQueries, tableID.String())
				m.mu.Unlock()
				return
			}
			continue
		}

		m.mu.Lock()
		delete(m.activeQueries, tableID.String())
		m.mu.Unlock()
		return
	}
}

// runLiveQuery executes a single live query.
func (m *LiveQueryManager) runLiveQuery(tableID domain.TableIdentifier) error {
	ctx, cancel := context.WithCancel(m.ctx)
	defer cancel()

	db, err := m.connManager.Get(ctx, tableID.Namespace, tableID.Database)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}

	live, err := sdk.Live(ctx, db, models.Table(tableID.Table), false)
	if err != nil {
		return fmt.Errorf("failed to create live query: %w", err)
	}

	liveID := live.String()
	slog.Info("Live query registered",
		"namespace", tableID.Namespace,
		"database", tableID.Database,
		"table", tableID.Table,
		"live_id", liveID)

	m.mu.Lock()
	m.activeQueries[tableID.String()] = &liveQueryState{
		tableID:   tableID,
		db:        db,
		liveID:    liveID,
		cancelCtx: cancel,
	}
	m.mu.Unlock()

	notifications, err := db.LiveNotifications(liveID)
	if err != nil {
		return fmt.Errorf("failed to get notifications: %w", err)
	}

	if notifications == nil {
		return errors.New("notifications channel is nil")
	}

	for {
		select {
		case <-ctx.Done():
			return nil

		case notification, ok := <-notifications:
			if !ok {
				return errors.New("notifications channel closed")
			}
			m.processNotification(tableID, notification)
		}
	}
}

// processNotification handles a live query notification.
func (m *LiveQueryManager) processNotification(
	tableID domain.TableIdentifier,
	notification sconn.Notification,
) {
	var action domain.OperationAction
	switch notification.Action {
	case sconn.CreateAction:
		action = domain.ActionCreate
	case sconn.UpdateAction:
		action = domain.ActionUpdate
	case sconn.DeleteAction:
		action = domain.ActionDelete
	default:
		action = domain.ActionUnknown
		slog.Warn("Unknown action type",
			"action", notification.Action,
			"table", tableID.String(),
		)
		return
	}

	opType := domain.OperationTypeUnknown
	switch res := notification.Result.(type) {
	case map[string]any:
		opType = m.detector.DetectFromRecord(res)
	case nil:
		slog.Debug("Live notification with nil result",
			"table", tableID.String(),
			"action", notification.Action,
		)
	default:
		slog.Warn("Unexpected live notification result type",
			"type", fmt.Sprintf("%T", res),
			"table", tableID.String(),
			"action", notification.Action,
		)
	}

	m.accumulator.Record(tableID, opType, action)

	slog.Debug("Operation recorded",
		"namespace", tableID.Namespace,
		"database", tableID.Database,
		"table", tableID.Table,
		"action", action,
		"operation_type", opType,
	)
}

// OperationTypeDetector analyzes record data to determine operation type.
type OperationTypeDetector struct{}

// NewOperationTypeDetector creates a new detector.
func NewOperationTypeDetector() *OperationTypeDetector {
	return &OperationTypeDetector{}
}

// DetectFromRecord analyzes a record's structure to determine operation type.
func (d *OperationTypeDetector) DetectFromRecord(record any) domain.OperationType {
	if record == nil {
		return domain.OperationTypeUnknown
	}

	recordMap, ok := record.(map[string]any)
	if !ok {
		return domain.OperationTypeUnknown
	}

	if d.isGraphRecord(recordMap) {
		return domain.OperationTypeGraph
	}

	if d.isKeyValueRecord(recordMap) {
		return domain.OperationTypeKeyValue
	}

	if d.isRelationalRecord(recordMap) {
		return domain.OperationTypeRelational
	}

	return domain.OperationTypeDocument
}

// isGraphRecord checks if record has graph edge characteristics.
func (d *OperationTypeDetector) isGraphRecord(record map[string]any) bool {
	hasIn := false
	hasOut := false

	for key := range record {
		if key == "in" {
			hasIn = true
		}
		if key == "out" {
			hasOut = true
		}
	}

	return hasIn && hasOut
}

// isKeyValueRecord checks if record has key-value characteristics.
func (d *OperationTypeDetector) isKeyValueRecord(record map[string]any) bool {
	fieldCount := 0
	for key := range record {
		if key != "id" {
			fieldCount++
		}
	}

	return fieldCount <= 2 && fieldCount > 0
}

// isRelationalRecord checks if record has relational characteristics.
func (d *OperationTypeDetector) isRelationalRecord(record map[string]any) bool {
	scalarCount := 0
	complexCount := 0

	for key, value := range record {
		if key == "id" {
			continue
		}

		switch value.(type) {
		case map[string]any, []any:
			complexCount++
		default:
			scalarCount++
		}
	}

	return scalarCount >= 3 && complexCount <= 1
}

// OperationAccumulator thread-safely accumulates operation counts.
type OperationAccumulator struct {
	metrics map[string]*domain.TableOperationMetrics
	mu      sync.RWMutex
}

// NewOperationAccumulator creates a new accumulator.
func NewOperationAccumulator() *OperationAccumulator {
	return &OperationAccumulator{
		metrics: make(map[string]*domain.TableOperationMetrics),
	}
}

// Record records an operation.
func (a *OperationAccumulator) Record(
	tableID domain.TableIdentifier,
	opType domain.OperationType,
	action domain.OperationAction,
) {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := makeKey(tableID, opType)

	metrics, exists := a.metrics[key]
	if !exists {
		metrics = &domain.TableOperationMetrics{
			Namespace:     tableID.Namespace,
			Database:      tableID.Database,
			Table:         tableID.Table,
			OperationType: opType,
		}
		a.metrics[key] = metrics
	}

	switch action {
	case domain.ActionCreate:
		metrics.Creates++
	case domain.ActionUpdate:
		metrics.Updates++
	case domain.ActionDelete:
		metrics.Deletes++
	}
}

// GetAndClear returns all metrics and clears the accumulator.
func (a *OperationAccumulator) GetAndClear() []*domain.TableOperationMetrics {
	a.mu.Lock()
	defer a.mu.Unlock()

	result := make([]*domain.TableOperationMetrics, 0, len(a.metrics))
	for _, m := range a.metrics {
		metricsCopy := &domain.TableOperationMetrics{
			Namespace:     m.Namespace,
			Database:      m.Database,
			Table:         m.Table,
			OperationType: m.OperationType,
			Creates:       m.Creates,
			Updates:       m.Updates,
			Deletes:       m.Deletes,
		}
		result = append(result, metricsCopy)
	}

	a.metrics = make(map[string]*domain.TableOperationMetrics)

	return result
}

// makeKey creates a unique key for table + operation type.
func makeKey(tableID domain.TableIdentifier, opType domain.OperationType) string {
	return tableID.String() + ":" + string(opType)
}
