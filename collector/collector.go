package collector

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/client"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "surrealdb"

type Collector interface {
	Update(ctx context.Context, client client.Client, ch chan<- prometheus.Metric) error
}

type CollectorFactory func(*slog.Logger, *config.Config) (Collector, error)

var (
	factories       = make(map[string]CollectorFactory)
	collectorStates = make(map[string]bool)
)

func registerCollector(name string, defaultEnabled bool, factory CollectorFactory) {
	factories[name] = factory
	collectorStates[name] = defaultEnabled
}

func isCollectorEnabled(name string, cfg *config.Config) bool {
	switch name {
	case "server_info":
		return cfg.Collectors.ServerInfo.Enabled
	case "metrics_demo":
		return cfg.Collectors.MetricsDemo.Enabled
	default:
		return collectorStates[name]
	}
}

func GetEnabledCollectorsCount() int {
	count := 0
	for _, enabled := range collectorStates {
		if enabled {
			count++
		}
	}
	return count
}

func GetEnabledCollectorsList() string {
	var collectors []string
	for name, enabled := range collectorStates {
		if enabled {
			collectors = append(collectors, fmt.Sprintf("<li>%s</li>", name))
		}
	}
	return strings.Join(collectors, "\n")
}
