package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// unexported root config type
type config struct {
	Exporter   exporterConfig   `yaml:"Exporter"`
	SurrealDB  surrealDBConfig  `yaml:"surrealdb"`
	Collectors collectorsConfig `yaml:"collectors"`
	Logging    loggingConfig    `yaml:"logging"`
}

// all nested types are also unexported, but their fields stay exported

type exporterConfig struct {
	Port        int    `yaml:"port"`
	MetricsPath string `yaml:"metrics_path"`
}

type surrealDBConfig struct {
	URI       string        `yaml:"uri"`
	Username  string        `yaml:"username"`
	Password  string        `yaml:"password"`
	Namespace string        `yaml:"namespace"`
	Database  string        `yaml:"database"`
	Timeout   time.Duration `yaml:"timeout"`
}

type collectorsConfig struct {
	ServerInfo  collectorConfig `yaml:"server_info"`
	MetricsDemo collectorConfig `yaml:"metrics_demo"`
	Go          collectorConfig `yaml:"go"`
	Process     collectorConfig `yaml:"process"`
}

type collectorConfig struct {
	Enabled bool `yaml:"enabled"`
}

type loggingConfig struct {
	Format           string         `yaml:"format"`
	Level            string         `yaml:"level"`
	CustomAttributes map[string]any `yaml:"custom_attributes"`
}

// Load is the only exported symbol
func Load(path string) (*config, error) {
	cfg := defaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read Config file: %w", err)
		}

		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse Config: %w", err)
		}
	}

	applyEnvironmentOverrides(cfg)

	return cfg, nil
}

// everything below stays unexported

func defaultConfig() *config {
	return &config{
		Exporter: exporterConfig{
			Port:        9224,
			MetricsPath: "/metrics",
		},
		SurrealDB: surrealDBConfig{
			URI:       "ws://localhost:8000",
			Username:  "root",
			Password:  "root",
			Namespace: "test",
			Database:  "test",
			Timeout:   10 * time.Second,
		},
		Collectors: collectorsConfig{
			ServerInfo:  collectorConfig{Enabled: true},
			MetricsDemo: collectorConfig{Enabled: true},
			Go:          collectorConfig{Enabled: false},
			Process:     collectorConfig{Enabled: false},
		},
	}
}

func applyEnvironmentOverrides(cfg *config) {
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

// methods can stay exported if you still want to use them from outside

func (c *config) Port() int {
	return c.Exporter.Port
}

func (c *config) MetricsPath() string {
	return c.Exporter.MetricsPath
}

func (c *config) SurrealURL() string {
	return c.SurrealDB.URI
}

func (c *config) SurrealNamespace() string {
	return c.SurrealDB.Namespace
}

func (c *config) SurrealDatabase() string {
	return c.SurrealDB.Database
}

func (c *config) SurrealUsername() string {
	return c.SurrealDB.Username
}

func (c *config) SurrealPassword() string {
	return c.SurrealDB.Password
}

func (c *config) SurrealTimeout() time.Duration {
	return c.SurrealDB.Timeout
}

func (c *config) GoCollectorEnabled() bool {
	return c.Collectors.Go.Enabled
}

func (c *config) ProcessCollectorEnabled() bool {
	return c.Collectors.Process.Enabled
}

func (c *config) Format() string {
	return strings.ToLower(c.Logging.Format)
}

func (c *config) Level() string {
	return strings.ToLower(c.Logging.Level)
}

func (c *config) CustomAttributes() map[string]any {
	return c.Logging.CustomAttributes
}
