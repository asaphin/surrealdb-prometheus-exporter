package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
	sdk "github.com/surrealdb/surrealdb.go"
)

type recordCountResult struct {
	Count int `json:"count"`
}

type recordCountReader struct {
	conn ConnectionManager
}

func NewRecordCountReader(conn ConnectionManager) (*recordCountReader, error) {
	if conn == nil {
		return nil, errors.New("conn argument cannot be nil")
	}

	return &recordCountReader{conn: conn}, nil
}

// RecordCount retrieves record counts for the provided tables in parallel
func (r *recordCountReader) RecordCount(ctx context.Context, tables []*domain.TableInfo) (*domain.RecordCountMetrics, error) {
	start := time.Now()

	if len(tables) == 0 {
		return &domain.RecordCountMetrics{
			Tables:         []*domain.TableRecordCount{},
			ScrapeDuration: time.Since(start),
		}, nil
	}

	tableCounts, err := r.fetchRecordCountsParallel(ctx, tables)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch record counts: %w", err)
	}

	return &domain.RecordCountMetrics{
		Tables:         tableCounts,
		ScrapeDuration: time.Since(start),
	}, nil
}

// fetchRecordCountsParallel retrieves record counts for multiple tables in parallel
func (r *recordCountReader) fetchRecordCountsParallel(ctx context.Context, tables []*domain.TableInfo) ([]*domain.TableRecordCount, error) {
	type countResult struct {
		tableCount *domain.TableRecordCount
		err        error
	}

	resultChan := make(chan countResult, len(tables))
	var wg sync.WaitGroup

	for _, table := range tables {
		wg.Add(1)
		go func(tbl *domain.TableInfo) {
			defer wg.Done()
			count, err := r.fetchTableRecordCount(ctx, tbl)
			resultChan <- countResult{tableCount: count, err: err}
		}(table)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	tableCounts := make([]*domain.TableRecordCount, 0, len(tables))
	var errs []error

	for result := range resultChan {
		if result.err != nil {
			errs = append(errs, result.err)
			continue
		}
		tableCounts = append(tableCounts, result.tableCount)
	}

	if len(errs) > 0 {
		return tableCounts, fmt.Errorf("errors fetching record counts: %v", errs)
	}

	return tableCounts, nil
}

// fetchTableRecordCount retrieves the record count for a single table
func (r *recordCountReader) fetchTableRecordCount(ctx context.Context, table *domain.TableInfo) (*domain.TableRecordCount, error) {
	db, err := r.conn.Get(ctx, table.Namespace, table.Database)
	if err != nil {
		return nil, fmt.Errorf("could not get DB connection for %s.%s.%s: %w",
			table.Namespace, table.Database, table.Name, err)
	}

	query := fmt.Sprintf("SELECT count() FROM %s GROUP ALL;", table.Name)
	results, err := sdk.Query[[]*recordCountResult](ctx, db, query, nil)
	if err != nil {
		return nil, fmt.Errorf("record count query failed for %s.%s.%s: %w",
			table.Namespace, table.Database, table.Name, err)
	}

	if results == nil || len(*results) == 0 {
		return nil, fmt.Errorf("record count query returned no results for %s.%s.%s",
			table.Namespace, table.Database, table.Name)
	}

	countResult := (*results)[0]
	if countResult.Status != "OK" {
		return nil, fmt.Errorf("record count query returned %s status for %s.%s.%s: %w",
			countResult.Status, table.Namespace, table.Database, table.Name, countResult.Error)
	}

	recordCount := 0
	if len(countResult.Result) != 0 {
		recordCount = countResult.Result[0].Count
	}

	return &domain.TableRecordCount{
		Name:        table.Name,
		Database:    table.Database,
		Namespace:   table.Namespace,
		RecordCount: recordCount,
	}, nil
}
