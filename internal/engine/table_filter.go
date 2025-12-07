package engine

import (
	"path/filepath"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/domain"
)

// tableFilter filters tables based on include/exclude patterns
type tableFilter struct {
	includePatterns []string
	excludePatterns []string
	hasIncludes     bool
}

// NewTableFilter creates a new table filter
func NewTableFilter(includePatterns, excludePatterns []string) *tableFilter {
	return &tableFilter{
		includePatterns: includePatterns,
		excludePatterns: excludePatterns,
		hasIncludes:     len(includePatterns) > 0,
	}
}

// shouldMonitor determines if a table should be monitored
func (f *tableFilter) shouldMonitor(tableID domain.TableIdentifier) bool {
	identifier := tableID.String()

	if !f.hasIncludes && len(f.excludePatterns) == 0 {
		return true
	}

	for _, pattern := range f.excludePatterns {
		if matchesPattern(identifier, pattern) {
			return false
		}
	}

	if f.hasIncludes {
		for _, pattern := range f.includePatterns {
			if matchesPattern(identifier, pattern) {
				return true
			}
		}
		return false
	}

	return true
}

// FilterTables returns tables that should be monitored
func (f *tableFilter) FilterTables(tables []*domain.TableInfo) []domain.TableIdentifier {
	var filtered []domain.TableIdentifier

	for _, table := range tables {
		tableID := domain.TableIdentifier{
			Namespace: table.Namespace,
			Database:  table.Database,
			Table:     table.Name,
		}

		if f.shouldMonitor(tableID) {
			filtered = append(filtered, tableID)
		}
	}

	return filtered
}

// matchesPattern checks if identifier matches glob pattern
func matchesPattern(identifier, pattern string) bool {
	matched, err := filepath.Match(pattern, identifier)
	if err != nil {
		return false
	}
	return matched
}
