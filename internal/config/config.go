package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// unexported root config type
type config struct {
	Exporter   exporterConfig   `yaml:"exporter"`
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
	Scheme         string        `yaml:"scheme"`
	Host           string        `yaml:"host"`
	Port           string        `yaml:"port"`
	Username       string        `yaml:"username"`
	Password       string        `yaml:"password"`
	Timeout        time.Duration `yaml:"timeout"`
	ClusterName    string        `yaml:"cluster_name"`
	StorageEngine  string        `yaml:"storage_engine"`
	DeploymentMode string        `yaml:"deployment_mode"`
}

type collectorsConfig struct {
	Info      collectorConfig `yaml:"info"`
	LiveQuery liveQueryConfig `yaml:"live_query"`
	Go        collectorConfig `yaml:"go"`
	Process   collectorConfig `yaml:"process"`
}

type collectorConfig struct {
	Enabled bool `yaml:"enabled"`
}

type liveQueryConfig struct {
	Enabled              bool          `yaml:"enabled"`
	Tables               tableConfig   `yaml:"tables"`
	ReconnectDelay       time.Duration `yaml:"reconnect_delay"`
	MaxReconnectAttempts int           `yaml:"max_reconnect_attempts"`
}

type tableConfig struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
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
			Scheme:   "ws",
			Host:     "localhost",
			Port:     "8000",
			Username: "root",
			Password: "root",
			Timeout:  10 * time.Second,
		},
		Collectors: collectorsConfig{
			Info: collectorConfig{Enabled: true},
			LiveQuery: liveQueryConfig{
				Enabled:              false,
				ReconnectDelay:       5 * time.Second,
				MaxReconnectAttempts: 10,
				Tables: tableConfig{
					Include: []string{},
					Exclude: []string{},
				},
			},
			Go:      collectorConfig{Enabled: false},
			Process: collectorConfig{Enabled: false},
		},
	}
}

func applyEnvironmentOverrides(cfg *config) {
	if uri := os.Getenv("SURREALDB_URI"); uri != "" {
		parsed, err := url.Parse(uri)
		if err == nil {
			cfg.SurrealDB.Scheme = parsed.Scheme
			cfg.SurrealDB.Host = parsed.Host
			cfg.SurrealDB.Port = parsed.Port()
		}

	}
	if username := os.Getenv("SURREALDB_USERNAME"); username != "" {
		cfg.SurrealDB.Username = username
	}
	if password := os.Getenv("SURREALDB_PASSWORD"); password != "" {
		cfg.SurrealDB.Password = password
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
	u := fmt.Sprintf("%s://%s", c.SurrealDB.Scheme, c.SurrealDB.Host)

	if c.SurrealDB.Port != "" {
		u = u + ":" + c.SurrealDB.Port
	}

	return u
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

func (c *config) ClusterName() string {
	return c.SurrealDB.ClusterName
}

func (c *config) StorageEngine() string {
	return c.SurrealDB.StorageEngine
}

func (c *config) DeploymentMode() string {
	return c.SurrealDB.DeploymentMode
}

func (c *config) InfoCollectorEnabled() bool {
	return c.Collectors.Info.Enabled
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

func (c *config) LiveQueryEnabled() bool {
	return c.Collectors.LiveQuery.Enabled
}

func (c *config) LiveQueryIncludePatterns() []string {
	return c.Collectors.LiveQuery.Tables.Include
}

func (c *config) LiveQueryExcludePatterns() []string {
	return c.Collectors.LiveQuery.Tables.Exclude
}

func (c *config) LiveQueryReconnectDelay() time.Duration {
	return c.Collectors.LiveQuery.ReconnectDelay
}

func (c *config) LiveQueryMaxReconnectAttempts() int {
	return c.Collectors.LiveQuery.MaxReconnectAttempts
}
