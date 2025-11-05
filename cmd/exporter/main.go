package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/api"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/client"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/config"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/logger"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/registry"
)

var configFile = flag.String("config.file", "./config.yaml", "Path to configuration file")

func main() {
	flag.Parse()

	cfg, err := config.Load(*configFile)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	logger.Configure(cfg)

	surrealDBClient, err := client.New(cfg)
	if err != nil {
		slog.Error("Failed to create client", "error", err)
		os.Exit(1)
	}

	metricsRegistry, err := registry.New(cfg, surrealDBClient)
	if err != nil {
		slog.Error("Failed to initialize registry", "error", err)
		os.Exit(1)
	}

	if err = api.StartServer(cfg, metricsRegistry); err != nil {
		slog.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}
