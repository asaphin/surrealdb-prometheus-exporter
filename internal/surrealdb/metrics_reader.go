package surrealdb

import (
	"context"
	"errors"
	"fmt"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
	sdk "github.com/surrealdb/surrealdb.go"
)

type metricsReader struct {
	db *sdk.DB
}

func NewMetricsReader(db *sdk.DB) (*metricsReader, error) {
	if db == nil {
		return nil, errors.New("db argument cannot be nil")
	}

	return &metricsReader{db: db}, nil
}

func (r *metricsReader) Info(ctx context.Context) (*domain.SurrealDBInfo, error) {
	results, err := sdk.Query[*domain.SurrealDBInfo](ctx, r.db, "INFO FOR ROOT", nil)
	if err != nil {
		return nil, fmt.Errorf("INFO FOR DB query failed: %w", err)
	}

	if results == nil || len(*results) == 0 {
		return nil, fmt.Errorf("INFO FOR DB returned no results")
	}

	return (*results)[0].Result, nil
}
