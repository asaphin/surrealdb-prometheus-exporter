package surrealdb

import (
	"context"
	"errors"
	"fmt"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
)

type recordCountReader struct {
	conn ConnectionManager
}

func NewRecordCountReader(conn ConnectionManager) (*recordCountReader, error) {
	if conn == nil {
		return nil, errors.New("conn argument cannot be nil")
	}

	return &recordCountReader{conn: conn}, nil
}

func (r *recordCountReader) RecordCount(ctx context.Context, tables []*domain.TableInfo) ([]*domain.TableRecordCount, error) {
	for i, table := range tables {
		db, err := r.conn.Get(ctx, table.Namespace, table.Database)
		if err != nil {
			return nil, fmt.Errorf("could not get DB connection: %w", err)
		}

	}

}
