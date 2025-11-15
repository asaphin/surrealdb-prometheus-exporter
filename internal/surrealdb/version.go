package surrealdb

import (
	"context"
	"errors"
	"sync"
	"time"
)

type versionReader struct {
	conn ConnectionManager

	mu            sync.RWMutex
	cachedVersion string
	cacheTime     time.Time
	cacheDuration time.Duration
}

func NewVersionReader(conn ConnectionManager) (*versionReader, error) {
	if conn == nil {
		return nil, errors.New("conn argument cannot be nil")
	}

	return &versionReader{
		conn:          conn,
		cacheDuration: 10 * time.Second,
	}, nil
}

func (r *versionReader) Version(ctx context.Context) (string, error) {
	r.mu.RLock()
	if time.Since(r.cacheTime) < r.cacheDuration && r.cachedVersion != "" {
		version := r.cachedVersion
		r.mu.RUnlock()
		return version, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if time.Since(r.cacheTime) < r.cacheDuration && r.cachedVersion != "" {
		return r.cachedVersion, nil
	}

	db, err := r.conn.Get(ctx, "", "")
	if err != nil {
		return "", err
	}

	v, err := db.Version(ctx)
	if err != nil {
		return "", err
	}

	r.cachedVersion = v.Version
	r.cacheTime = time.Now()

	return r.cachedVersion, nil
}
