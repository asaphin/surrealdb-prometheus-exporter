# SurrealDB Prometheus Exporter

[![Alpha](https://img.shields.io/badge/status-alpha-orange)](https://github.com/asaphin/surrealdb-prometheus-exporter)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)](LICENSE)

Prometheus exporter for [SurrealDB](https://surrealdb.com/) metrics.

> **⚠️ Alpha Version**: This project is in early development. Configuration and APIs may change.

## Prerequisites

- [Go 1.25+](https://go.dev/dl/) (for building from source)
- [Docker](https://docs.docker.com/get-started/get-docker/) (for containerized deployment)

## Running

### Docker

```bash
docker run -d \
  -p 9224:9224 \
  -v /path/to/config.yaml:/config/config.yaml \
  asaphin/surrealdb-prometheus-exporter
```

### Binary

Build from source:
```bash
go build -o exporter ./cmd/exporter
```

Run:
```bash
./exporter -config.file=./config.yaml
```

## Configuration

Configuration is done via YAML file. See [config.yaml](config.yaml) for all options.

```yaml
exporter:
  port: 9224
  metrics_path: /metrics

surrealdb:
  scheme: ws
  host: localhost
  port: 8000
  username: root
  password: root
  cluster_name: my-cluster
  storage_engine: memory      # memory, rocksdb, tikv
  deployment_mode: single     # single, distributed, cloud
```

## Collectors

| Collector | Description | Default |
|-----------|-------------|---------|
| `info` | Database info and version | always enabled |
| `record_count` | Record counts per table | enabled |
| `live_query` | Live query metrics (single mode only) | disabled |
| `stats_table` | Custom stats table metrics | disabled |
| `open_telemetry` | OTLP/gRPC receiver on `:4317` | disabled |
| `go` | Go runtime metrics | disabled |
| `process` | Process metrics | disabled |

## Prometheus Configuration

```yaml
scrape_configs:
  - job_name: 'surrealdb'
    static_configs:
      - targets: ['localhost:9224']
```

## Endpoints

| Path | Description |
|------|-------------|
| `/` | Landing page |
| `/metrics` | Prometheus metrics |

## Development

Development utilities: [surrealdb-exporter-utils](https://github.com/AntonChubarov/surrealdb-exporter-utils)

## License

Apache License 2.0
