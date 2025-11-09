package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/api"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/config"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/logger"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/registry"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/surrealdb"
)

var configFile = flag.String("config.file", "./config.yaml", "Path to configuration file")

func main() {
	flag.Parse()

	//ctx := context.Background()

	cfg, err := config.Load(*configFile)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	logger.Configure(cfg)

	var dbConnManager surrealdb.ConnectionManager

	if cfg.SurrealConnectionPool() {
		dbConnManager = surrealdb.NewMultiConnectionManager(cfg)
	} else if !cfg.SurrealConnectionPool() {
		dbConnManager = surrealdb.NewSingleConnectionManager(cfg)
	}

	metricsReader, err := surrealdb.NewMetricsReader(dbConnManager)
	if err != nil {
		slog.Error("Failed to create surrealdb metrics reader", "error", err)
	}

	metricsRegistry, err := registry.New(cfg, metricsReader)
	if err != nil {
		slog.Error("Failed to initialize registry", "error", err)
		os.Exit(1)
	}

	if err = api.StartServer(cfg, metricsRegistry); err != nil {
		slog.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}
