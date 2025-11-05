package api

import (
	"fmt"
	"html/template"
	"log"
	"log/slog"
	"net/http"

	"github.com/asaphin/surrealdb-prometheus-exporter/internal/config"
	"github.com/asaphin/surrealdb-prometheus-exporter/static"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type PageData struct {
	MetricsPath           string
	EnabledCollectorsHTML template.HTML
}

func StartServer(logger *slog.Logger, cfg *config.Config, registry *prometheus.Registry) error {
	indexTmpl, err := template.ParseFS(static.Files, "index.html")
	if err != nil {
		log.Printf("unable to parse templates: %v", err)
		return fmt.Errorf("parse template: %w", err)
	}

	mux := http.NewServeMux()

	mux.Handle(cfg.Exporter.MetricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
		ErrorLog:      slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		data := PageData{
			MetricsPath:           cfg.Exporter.MetricsPath,
			EnabledCollectorsHTML: template.HTML(`<li>Go</li>`),
		}

		if err := indexTmpl.Execute(w, data); err != nil {
			http.Error(w, "template render error", http.StatusInternalServerError)
			log.Printf("index template error: %v", err)
			return
		}
	})

	listenAddress := fmt.Sprintf(":%d", cfg.Exporter.Port)

	logger.Info("Starting SurrealDB Exporter",
		"address", listenAddress,
		"metrics_path", cfg.Exporter.MetricsPath,
		"enabled_collectors", 1,
	)

	return http.ListenAndServe(listenAddress, mux)
}
