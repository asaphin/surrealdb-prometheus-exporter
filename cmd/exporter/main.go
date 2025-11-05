package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/api"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/client"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/config"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/registry"
)

var configFile = flag.String("config.file", "./config.yaml", "Path to configuration file")

func main() {
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load(*configFile)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	surrealDBClient, err := client.New(cfg)
	if err != nil {
		logger.Error("Failed to create client", "error", err)
		os.Exit(1)
	}

	metricsRegistry, err := registry.New(logger, cfg, surrealDBClient)
	if err != nil {
		logger.Error("Failed to initialize registry", "error", err)
		os.Exit(1)
	}

	if err = api.StartServer(logger, cfg, metricsRegistry); err != nil {
		logger.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}
