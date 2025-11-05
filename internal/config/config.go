package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Exporter   ExporterConfig   `yaml:"exporter"`
	SurrealDB  SurrealDBConfig  `yaml:"surrealdb"`
	Collectors CollectorsConfig `yaml:"collectors"`
}

type ExporterConfig struct {
	Port        int    `yaml:"port"`
	MetricsPath string `yaml:"metrics_path"`
}

type SurrealDBConfig struct {
	URI       string        `yaml:"uri"`
	Username  string        `yaml:"username"`
	Password  string        `yaml:"password"`
	Namespace string        `yaml:"namespace"`
	Database  string        `yaml:"database"`
	Timeout   time.Duration `yaml:"timeout"`
}

type CollectorsConfig struct {
	ServerInfo  CollectorConfig `yaml:"server_info"`
	MetricsDemo CollectorConfig `yaml:"metrics_demo"`
	Go          CollectorConfig `yaml:"go"`
	Process     CollectorConfig `yaml:"process"`
}

type CollectorConfig struct {
	Enabled bool `yaml:"enabled"`
}

func Load(path string) (*Config, error) {
	cfg := defaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config: %w", err)
		}
	}

	applyEnvironmentOverrides(cfg)

	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		Exporter: ExporterConfig{
			Port:        9224,
			MetricsPath: "/metrics",
		},
		SurrealDB: SurrealDBConfig{
			URI:       "ws://localhost:8000",
			Username:  "root",
			Password:  "root",
			Namespace: "test",
			Database:  "test",
			Timeout:   10 * time.Second,
		},
		Collectors: CollectorsConfig{
			ServerInfo:  CollectorConfig{Enabled: true},
			MetricsDemo: CollectorConfig{Enabled: true},
			Go:          CollectorConfig{Enabled: false},
			Process:     CollectorConfig{Enabled: false},
		},
	}
}

func applyEnvironmentOverrides(cfg *Config) {
	if uri := os.Getenv("SURREALDB_URI"); uri != "" {
		cfg.SurrealDB.URI = uri
	}
	if username := os.Getenv("SURREALDB_USERNAME"); username != "" {
		cfg.SurrealDB.Username = username
	}
	if password := os.Getenv("SURREALDB_PASSWORD"); password != "" {
		cfg.SurrealDB.Password = password
	}
	if namespace := os.Getenv("SURREALDB_NAMESPACE"); namespace != "" {
		cfg.SurrealDB.Namespace = namespace
	}
	if database := os.Getenv("SURREALDB_DATABASE"); database != "" {
		cfg.SurrealDB.Database = database
	}
}
