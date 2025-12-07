package main //nolint:cyclop

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/api"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/config"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/converter"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/engine"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/logger"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/processor"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/registry"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/surrealcollectors"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/surrealdb"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
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

	dbConnManager := surrealdb.NewMultiConnectionManager(cfg)

	versionReader, err := surrealdb.NewVersionReader(dbConnManager)
	if err != nil {
		slog.Error("Failed to initialize version reader", "error", err)
		os.Exit(1)
	}

	infoReader, err := surrealdb.NewInfoReader(cfg, dbConnManager)
	if err != nil {
		slog.Error("Failed to create surrealdb metrics reader", "error", err)
		os.Exit(1)
	}

	recordCountReader, err := surrealdb.NewRecordCountReader(dbConnManager)
	if err != nil {
		slog.Error("Failed to create surrealdb record count reader", "error", err)
		os.Exit(1)
	}

	tableFilter := engine.NewTableFilter(cfg.LiveQueryIncludePatterns(), cfg.LiveQueryExcludePatterns())
	liveQueryProvider := surrealdb.NewLiveQueryManager(
		dbConnManager,
		cfg.LiveQueryReconnectDelay(),
		cfg.LiveQueryMaxReconnectAttempts(),
	)

	statsTableFilter := engine.NewTableFilter(cfg.StatsTableIncludePatterns(), cfg.StatsTableExcludePatterns())
	statsTableProvider := surrealdb.NewStatsTableManager(
		dbConnManager,
		cfg.StatsTableRemoveOrphanTables(),
		cfg.StatsTableNamePrefix(),
	)

	recordCountFilter := engine.NewTableFilter(cfg.RecordCountIncludePatterns(), cfg.RecordCountExcludePatterns())

	// Pre-warm the table cache
	if cfg.StatsTableEnabled() || cfg.LiveQueryEnabled() || cfg.RecordCountCollectorEnabled() {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.SurrealTimeout())
		info, err := infoReader.Info(ctx)
		cancel()
		if err != nil {
			slog.Warn("Failed to pre-warm table cache", "error", err)
		} else {
			surrealcollectors.PrewarmTableCache(info.AllTables())
			slog.Info("Table cache pre-warmed", "table_count", len(info.AllTables()))
		}
	}

	metricsRegistry, err := registry.New(
		cfg,
		versionReader,
		infoReader,
		recordCountReader,
		liveQueryProvider,
		statsTableProvider,
		tableFilter,
		statsTableFilter,
		recordCountFilter,
	)
	if err != nil {
		slog.Error("Failed to initialize registry", "error", err)
		os.Exit(1)
	}

	gatherers := prometheus.Gatherers{metricsRegistry}

	var otlpShutdown func()
	if cfg.OTLPReceiverEnabled() {
		var otlpRegistry *prometheus.Registry
		otlpRegistry, otlpShutdown = startOTLPReceiver(cfg)
		gatherers = append(gatherers, otlpRegistry)
	}

	serverErrChan := make(chan error, 1)
	go func() {
		if err := api.StartPrometheusServer(cfg, gatherers); err != nil {
			serverErrChan <- err
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrChan:
		slog.Error("HTTP server failed", "error", err)
	case sig := <-sigChan:
		slog.Info("Received shutdown signal", "signal", sig)
	}

	if otlpShutdown != nil {
		otlpShutdown()
	}

	slog.Info("Exporter shutdown complete")
}

// startOTLPReceiver starts the OTLP gRPC receiver and returns the registry.
func startOTLPReceiver(cfg config.Config) (*prometheus.Registry, func()) {
	slog.Info("Starting OpenTelemetry collector")

	otlpRegistry := prometheus.NewRegistry()

	conv := converter.NewConverter(cfg, otlpRegistry)

	var proc processor.Processor
	if cfg.OTLPBatchingEnabled() {
		batchTimeout := time.Duration(cfg.OTLPBatchTimeoutMs()) * time.Millisecond
		proc = processor.NewBatchProcessor(conv, cfg.OTLPBatchSize(), batchTimeout)
	} else {
		proc = processor.NewDirectProcessor(conv)
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.OTLPMaxRecvSize() * 1024 * 1024),
	)
	otlpGRPC := api.NewOTELGRPCServer(proc)
	otlpGRPC.RegisterWith(grpcServer)

	lis, err := net.Listen("tcp", cfg.OTLPGRPCEndpoint())
	if err != nil {
		slog.Error("Failed to listen on gRPC endpoint", "error", err, "endpoint", cfg.OTLPGRPCEndpoint())
	} else {
		go func() {
			slog.Info("OpenTelemetry gRPC receiver started", "endpoint", cfg.OTLPGRPCEndpoint())
			if err := grpcServer.Serve(lis); err != nil {
				slog.Error("OpenTelemetry gRPC server failed", "error", err)
			}
		}()
	}

	return otlpRegistry, func() {
		slog.Info("Shutting down OpenTelemetry collector")

		grpcServer.GracefulStop()

		if batchProc, ok := proc.(*processor.BatchProcessor); ok {
			if err := batchProc.Flush(); err != nil {
				slog.Error("Error flushing batch processor", "error", err)
			}
		}

		slog.Info("OpenTelemetry collector shutdown complete")
	}
}
