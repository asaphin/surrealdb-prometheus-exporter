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

	// Returns defensive cache copy
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
