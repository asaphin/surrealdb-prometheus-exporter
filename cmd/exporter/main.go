package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/asaphin/surrealdb-prometheus-exporter/collector"
	"github.com/asaphin/surrealdb-prometheus-exporter/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	configFile      = flag.String("config.file", "./config.yaml", "Path to configuration file")
	listenAddress   = flag.String("web.listen-address", ":9224", "Address to listen on")
	metricsPath     = flag.String("web.telemetry-path", "/metrics", "Path for metrics")
	enableGoMetrics = flag.Bool("web.enable-go-metrics", false, "Enable Go runtime metrics")
)

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

	registry := prometheus.NewRegistry()

	exporter, err := collector.NewExporter(logger, cfg)
	if err != nil {
		logger.Error("Failed to create exporter", "error", err)
		os.Exit(1)
	}

	registry.MustRegister(exporter)

	if *enableGoMetrics {
		registry.MustRegister(prometheus.NewGoCollector())
		registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	}

	http.Handle(*metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
		ErrorLog:      slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<html>
<head><title>SurrealDB Exporter</title></head>
<body>
<h1>SurrealDB Exporter</h1>
<p><a href="%s">Metrics</a></p>
<h2>Enabled Collectors</h2>
<ul>%s</ul>
</body>
</html>`, *metricsPath, collector.GetEnabledCollectorsList())
	})

	logger.Info("Starting SurrealDB Exporter",
		"address", *listenAddress,
		"metrics_path", *metricsPath,
		"enabled_collectors", collector.GetEnabledCollectorsCount())

	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		logger.Error("HTTP server failed", "error", err)
		os.Exit(1)
	}
}
