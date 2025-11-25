package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Allowed values for configuration fields
const (
	DefaultClusterName    = "default-cluster"
	DefaultStorageEngine  = "memory"
	DefaultDeploymentMode = "single"

	DefaultPort        = 9224
	DefaultMetricsPath = "/metrics"

	MinTimeout = 1 * time.Second
	MaxTimeout = 5 * time.Minute

	MinPort        = 1
	MaxPort        = 65535
	PrivilegedPort = 1024
)

var (
	AllowedStorageEngines  = []string{"memory", "rocksdb", "tikv"}
	AllowedDeploymentModes = []string{"single", "distributed", "cloud"}

	// metricsPathRegex validates metrics path format
	metricsPathRegex = regexp.MustCompile(`^/[a-zA-Z0-9_\-/]*$`)

	// tableFilterPatternRegex validates table filter patterns (namespace:database:table with wildcards)
	tableFilterPatternRegex = regexp.MustCompile(`^[a-zA-Z0-9_*]+:[a-zA-Z0-9_*]+:[a-zA-Z0-9_*]+$`)
)

// Config interface for external packages
type Config interface {
	OTLPBatchingEnabled() bool
	OTLPBatchTimeoutMs() int
	OTLPBatchSize() int
	OTLPGRPCEndpoint() string
	OTLPMaxRecvSize() int
	OTLPTranslationStrategy() string
	ClusterName() string
	StorageEngine() string
	DeploymentMode() string
}

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
	LiveQuery     liveQueryConfig     `yaml:"live_query"`
	RecordCount   recordCountConfig   `yaml:"record_count"`
	StatsTable    statsTableConfig    `yaml:"stats_table"`
	OpenTelemetry openTelemetryConfig `yaml:"open_telemetry"`
	Go            collectorConfig     `yaml:"go"`
	Process       collectorConfig     `yaml:"process"`
}

type collectorConfig struct {
	Enabled bool `yaml:"enabled"`
}

type recordCountConfig struct {
	Enabled bool `yaml:"enabled"`
}

type liveQueryConfig struct {
	Enabled              bool          `yaml:"enabled"`
	Tables               tableConfig   `yaml:"tables"`
	ReconnectDelay       time.Duration `yaml:"reconnect_delay"`
	MaxReconnectAttempts int           `yaml:"max_reconnect_attempts"`
}

type statsTableConfig struct {
	Enabled             bool        `yaml:"enabled"`
	Tables              tableConfig `yaml:"tables"`
	RemoveOrphanTables  bool        `yaml:"remove_orphan_tables"`
	SideTableNamePrefix string      `yaml:"side_table_name_prefix"`
}

type tableConfig struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

type openTelemetryConfig struct {
	Enabled             bool   `yaml:"enabled"`
	GRPCEndpoint        string `yaml:"grpc_endpoint"`
	MaxRecvSize         int    `yaml:"max_recv_size"` // in MB
	TranslationStrategy string `yaml:"translation_strategy"`
	EnableBatching      bool   `yaml:"enable_batching"`
	BatchSize           int    `yaml:"batch_size"`
	BatchTimeoutMs      int    `yaml:"batch_timeout_ms"`
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

	// Validate and fix configuration with warnings
	validateAndFix(cfg)

	return cfg, nil
}

// validateAndFix validates configuration and fixes misconfigurations with warnings
func validateAndFix(cfg *config) {
	validateExporterConfig(cfg)
	validateSurrealDBConfig(cfg)
	validateCollectorsConfig(cfg)
}

// validateExporterConfig validates exporter settings
func validateExporterConfig(cfg *config) {
	// Validate port range
	if cfg.Exporter.Port < MinPort || cfg.Exporter.Port > MaxPort {
		slog.Warn("exporter port is out of valid range, using default",
			"provided", cfg.Exporter.Port,
			"valid_range", fmt.Sprintf("%d-%d", MinPort, MaxPort),
			"default", DefaultPort)
		cfg.Exporter.Port = DefaultPort
	} else if cfg.Exporter.Port < PrivilegedPort {
		slog.Warn("exporter port is in privileged range, may require elevated permissions",
			"port", cfg.Exporter.Port,
			"privileged_range", fmt.Sprintf("%d-%d", MinPort, PrivilegedPort-1))
	}

	// Validate metrics path
	if cfg.Exporter.MetricsPath == "" {
		slog.Warn("metrics_path is empty, using default",
			"default", DefaultMetricsPath)
		cfg.Exporter.MetricsPath = DefaultMetricsPath
	} else if !strings.HasPrefix(cfg.Exporter.MetricsPath, "/") {
		slog.Warn("metrics_path must start with '/', adding prefix",
			"provided", cfg.Exporter.MetricsPath,
			"corrected", "/"+cfg.Exporter.MetricsPath)
		cfg.Exporter.MetricsPath = "/" + cfg.Exporter.MetricsPath
	}

	if !metricsPathRegex.MatchString(cfg.Exporter.MetricsPath) {
		slog.Warn("metrics_path contains invalid characters, using default",
			"provided", cfg.Exporter.MetricsPath,
			"allowed_pattern", "^/[a-zA-Z0-9_\\-/]*$",
			"default", DefaultMetricsPath)
		cfg.Exporter.MetricsPath = DefaultMetricsPath
	}
}

// validateSurrealDBConfig validates SurrealDB connection settings
func validateSurrealDBConfig(cfg *config) {
	// Validate cluster_name
	if strings.TrimSpace(cfg.SurrealDB.ClusterName) == "" {
		slog.Warn("cluster_name is empty, using default value",
			"default", DefaultClusterName)
		cfg.SurrealDB.ClusterName = DefaultClusterName
	}

	// Validate storage_engine
	if strings.TrimSpace(cfg.SurrealDB.StorageEngine) == "" {
		slog.Warn("storage_engine is empty, using default value",
			"default", DefaultStorageEngine,
			"allowed_values", AllowedStorageEngines)
		cfg.SurrealDB.StorageEngine = DefaultStorageEngine
	} else if !slices.Contains(AllowedStorageEngines, cfg.SurrealDB.StorageEngine) {
		slog.Warn("storage_engine has invalid value, using default value",
			"provided", cfg.SurrealDB.StorageEngine,
			"default", DefaultStorageEngine,
			"allowed_values", AllowedStorageEngines)
		cfg.SurrealDB.StorageEngine = DefaultStorageEngine
	}

	// Validate deployment_mode
	if strings.TrimSpace(cfg.SurrealDB.DeploymentMode) == "" {
		slog.Warn("deployment_mode is empty, using default value",
			"default", DefaultDeploymentMode,
			"allowed_values", AllowedDeploymentModes)
		cfg.SurrealDB.DeploymentMode = DefaultDeploymentMode
	} else if !slices.Contains(AllowedDeploymentModes, cfg.SurrealDB.DeploymentMode) {
		slog.Warn("deployment_mode has invalid value, using default value",
			"provided", cfg.SurrealDB.DeploymentMode,
			"default", DefaultDeploymentMode,
			"allowed_values", AllowedDeploymentModes)
		cfg.SurrealDB.DeploymentMode = DefaultDeploymentMode
	}

	// Validate timeout
	if cfg.SurrealDB.Timeout < MinTimeout {
		slog.Warn("surrealdb timeout is too short, using minimum value",
			"provided", cfg.SurrealDB.Timeout,
			"minimum", MinTimeout)
		cfg.SurrealDB.Timeout = MinTimeout
	} else if cfg.SurrealDB.Timeout > MaxTimeout {
		slog.Warn("surrealdb timeout is too long, using maximum value",
			"provided", cfg.SurrealDB.Timeout,
			"maximum", MaxTimeout)
		cfg.SurrealDB.Timeout = MaxTimeout
	}
}

// validateCollectorsConfig validates collectors settings
func validateCollectorsConfig(cfg *config) {
	// Validate live_query collector - only available for single deployment_mode
	if cfg.Collectors.LiveQuery.Enabled && cfg.SurrealDB.DeploymentMode != "single" {
		slog.Warn("live_query collector is only available for 'single' deployment_mode, disabling it",
			"current_deployment_mode", cfg.SurrealDB.DeploymentMode)
		cfg.Collectors.LiveQuery.Enabled = false
	}

	// Validate table filter patterns for live_query
	validateTablePatterns("live_query.tables.include", &cfg.Collectors.LiveQuery.Tables.Include)
	validateTablePatterns("live_query.tables.exclude", &cfg.Collectors.LiveQuery.Tables.Exclude)

	// Validate table filter patterns for stats_table
	validateTablePatterns("stats_table.tables.include", &cfg.Collectors.StatsTable.Tables.Include)
	validateTablePatterns("stats_table.tables.exclude", &cfg.Collectors.StatsTable.Tables.Exclude)

	// Validate OpenTelemetry settings
	validateOpenTelemetryConfig(cfg)
}

// validateTablePatterns validates and filters invalid table patterns
func validateTablePatterns(fieldName string, patterns *[]string) {
	if patterns == nil || len(*patterns) == 0 {
		return
	}

	validPatterns := make([]string, 0, len(*patterns))
	for _, pattern := range *patterns {
		if tableFilterPatternRegex.MatchString(pattern) {
			validPatterns = append(validPatterns, pattern)
		} else {
			slog.Warn("invalid table filter pattern, removing from list",
				"field", fieldName,
				"pattern", pattern,
				"expected_format", "namespace:database:table (wildcards allowed: *)")
		}
	}
	*patterns = validPatterns
}

// validateOpenTelemetryConfig validates OpenTelemetry collector settings
func validateOpenTelemetryConfig(cfg *config) {
	otel := &cfg.Collectors.OpenTelemetry

	// Validate gRPC endpoint
	if otel.Enabled && otel.GRPCEndpoint == "" {
		slog.Warn("open_telemetry is enabled but grpc_endpoint is empty, using default",
			"default", ":4317")
		otel.GRPCEndpoint = ":4317"
	}

	// Validate batch size
	if otel.BatchSize <= 0 {
		slog.Warn("open_telemetry batch_size must be positive, using default",
			"provided", otel.BatchSize,
			"default", 100)
		otel.BatchSize = 100
	}

	// Validate batch timeout
	if otel.BatchTimeoutMs <= 0 {
		slog.Warn("open_telemetry batch_timeout_ms must be positive, using default",
			"provided", otel.BatchTimeoutMs,
			"default", 1000)
		otel.BatchTimeoutMs = 1000
	}

	// Validate max recv size
	if otel.MaxRecvSize <= 0 {
		slog.Warn("open_telemetry max_recv_size must be positive, using default",
			"provided", otel.MaxRecvSize,
			"default", 4)
		otel.MaxRecvSize = 4
	}

	// Validate translation strategy
	validStrategies := []string{"UnderscoreEscapingWithSuffixes", "NoTranslation"}
	if otel.TranslationStrategy == "" {
		slog.Warn("open_telemetry translation_strategy is empty, using default",
			"default", "UnderscoreEscapingWithSuffixes")
		otel.TranslationStrategy = "UnderscoreEscapingWithSuffixes"
	} else if !slices.Contains(validStrategies, otel.TranslationStrategy) {
		slog.Warn("open_telemetry translation_strategy has invalid value, using default",
			"provided", otel.TranslationStrategy,
			"allowed_values", validStrategies,
			"default", "UnderscoreEscapingWithSuffixes")
		otel.TranslationStrategy = "UnderscoreEscapingWithSuffixes"
	}
}

// everything below stays unexported

func defaultConfig() *config {
	return &config{
		Exporter: exporterConfig{
			Port:        DefaultPort,
			MetricsPath: DefaultMetricsPath,
		},
		SurrealDB: surrealDBConfig{
			Scheme:         "ws",
			Host:           "localhost",
			Port:           "8000",
			Username:       "root",
			Password:       "root",
			Timeout:        10 * time.Second,
			ClusterName:    DefaultClusterName,
			StorageEngine:  DefaultStorageEngine,
			DeploymentMode: DefaultDeploymentMode,
		},
		Collectors: collectorsConfig{
			LiveQuery: liveQueryConfig{
				Enabled:              false,
				ReconnectDelay:       5 * time.Second,
				MaxReconnectAttempts: 10,
				Tables: tableConfig{
					Include: []string{},
					Exclude: []string{},
				},
			},
			RecordCount: recordCountConfig{
				Enabled: true,
			},
			StatsTable: statsTableConfig{
				Enabled:             false,
				RemoveOrphanTables:  false,
				SideTableNamePrefix: "_stats_",
				Tables: tableConfig{
					Include: []string{},
					Exclude: []string{},
				},
			},
			OpenTelemetry: openTelemetryConfig{
				Enabled:             false,
				GRPCEndpoint:        ":4317",
				MaxRecvSize:         4,
				TranslationStrategy: "UnderscoreEscapingWithSuffixes",
				EnableBatching:      true,
				BatchSize:           100,
				BatchTimeoutMs:      1000,
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

// InfoCollectorEnabled always returns true - info collector is always active
func (c *config) InfoCollectorEnabled() bool {
	return true
}

func (c *config) RecordCountCollectorEnabled() bool {
	return c.Collectors.RecordCount.Enabled
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

func (c *config) StatsTableEnabled() bool {
	return c.Collectors.StatsTable.Enabled
}

func (c *config) StatsTableIncludePatterns() []string {
	return c.Collectors.StatsTable.Tables.Include
}

func (c *config) StatsTableExcludePatterns() []string {
	return c.Collectors.StatsTable.Tables.Exclude
}

func (c *config) StatsTableRemoveOrphanTables() bool {
	return c.Collectors.StatsTable.RemoveOrphanTables
}

func (c *config) StatsTableNamePrefix() string {
	return c.Collectors.StatsTable.SideTableNamePrefix
}

// OpenTelemetry configuration accessors

func (c *config) OTLPReceiverEnabled() bool {
	return c.Collectors.OpenTelemetry.Enabled
}

func (c *config) OTLPGRPCEndpoint() string {
	return c.Collectors.OpenTelemetry.GRPCEndpoint
}

func (c *config) OTLPMaxRecvSize() int {
	return c.Collectors.OpenTelemetry.MaxRecvSize
}

func (c *config) OTLPTranslationStrategy() string {
	return c.Collectors.OpenTelemetry.TranslationStrategy
}

func (c *config) OTLPBatchingEnabled() bool {
	return c.Collectors.OpenTelemetry.EnableBatching
}

func (c *config) OTLPBatchSize() int {
	return c.Collectors.OpenTelemetry.BatchSize
}

func (c *config) OTLPBatchTimeoutMs() int {
	return c.Collectors.OpenTelemetry.BatchTimeoutMs
}
