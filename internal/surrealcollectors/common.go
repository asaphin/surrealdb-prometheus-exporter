package surrealcollectors

import "github.com/prometheus/client_golang/prometheus"

const namespace = "surrealdb"

// Common descriptors shared by collectors for scrape instrumentation and "up" metric.
var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_duration_seconds"),
		"Duration of a collector scrape.",
		[]string{"collector"}, nil,
	)

	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_success"),
		"Whether a collector succeeded.",
		[]string{"collector"}, nil,
	)

	upDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Whether the SurrealDB server is up.",
		nil, nil,
	)
)
