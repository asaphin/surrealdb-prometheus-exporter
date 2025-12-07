package surrealcollectors

import (
	"sync"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
)

type tableInfoCache struct {
	mu     sync.RWMutex
	tables []*domain.TableInfo
}

var (
	instance *tableInfoCache
	once     sync.Once
)

// getTableInfoCache returns the singleton instance of tableInfoCache
func getTableInfoCache() *tableInfoCache {
	once.Do(func() {
		instance = &tableInfoCache{
			tables: make([]*domain.TableInfo, 0),
		}
	})

	return instance
}

// set stores the table information in the cache
func (c *tableInfoCache) set(tables []*domain.TableInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.tables = tables
}

// get retrieves the table information from the cache
func (c *tableInfoCache) get() []*domain.TableInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*domain.TableInfo, len(c.tables))
	copy(result, c.tables)
	return result
}

// clear removes all table information from the cache
func (c *tableInfoCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.tables = make([]*domain.TableInfo, 0)
}

// PrewarmTableCache pre-populates the table cache with table information.
// This should be called at startup before the HTTP server starts to avoid
// race conditions between collectors that depend on the cache.
func PrewarmTableCache(tables []*domain.TableInfo) {
	cache := getTableInfoCache()
	cache.set(tables)
}
