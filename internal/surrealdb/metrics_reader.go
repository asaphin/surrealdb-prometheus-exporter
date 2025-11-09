package surrealdb

import (
	"context"
	"errors"
	"fmt"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
	sdk "github.com/surrealdb/surrealdb.go"
)

type metricsReader struct {
	conn ConnectionManager
}

func NewMetricsReader(conn ConnectionManager) (*metricsReader, error) {
	if conn == nil {
		return nil, errors.New("conn argument cannot be nil")
	}

	return &metricsReader{conn: conn}, nil
}

func (r *metricsReader) Info(ctx context.Context) (*domain.SurrealDBInfo, error) {
	db, err := r.conn.Get(ctx, "", "")
	if err != nil {
		return nil, fmt.Errorf("could not get DB connection from connection manager: %w", err)
	}

	results, err := sdk.Query[*domain.SurrealDBInfo](ctx, db, "INFO FOR ROOT", nil)
	if err != nil {
		return nil, fmt.Errorf("INFO FOR DB query failed: %w", err)
	}

	if results == nil || len(*results) == 0 {
		return nil, fmt.Errorf("INFO FOR DB returned no results")
	}

	rootInfoResult := (*results)[0]

	if rootInfoResult.Status != "OK" {
		return nil, fmt.Errorf("INFO FOR DB returned %s status: %w", rootInfoResult.Status, rootInfoResult.Error)
	}

	rootInfo := rootInfoResult.Result

	_ = rootInfo.ListNamespaces()

	return rootInfo, nil
}
