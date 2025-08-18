// Package config provides centralized configuration management
// using Viper for configuration loading and validation
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	App        AppConfig        `mapstructure:"app"`
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Redis      RedisConfig      `mapstructure:"redis"`
	Auth       AuthConfig       `mapstructure:"auth"`
	AWS        AWSConfig        `mapstructure:"aws"`
	AI         AIConfig         `mapstructure:"ai"`
	Kafka      KafkaConfig      `mapstructure:"kafka"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	Email      EmailConfig      `mapstructure:"email"`
	Storage    StorageConfig    `mapstructure:"storage"`
	RateLimit  RateLimitConfig  `mapstructure:"rate_limit"`
	Features   FeatureFlags     `mapstructure:"features"`
}

// AppConfig contains application-level configuration
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Version     string `mapstructure:"version"`
	Environment string `mapstructure:"environment"`
	Debug       bool   `mapstructure:"debug"`
	LogLevel    string `mapstructure:"log_level"`
	LogFormat   string `mapstructure:"log_format"`
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Host              string        `mapstructure:"host"`
	Port              int           `mapstructure:"port"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"`
	MaxHeaderBytes    int           `mapstructure:"max_header_bytes"`
	ShutdownTimeout   time.Duration `mapstructure:"shutdown_timeout"`
	EnableCORS        bool          `mapstructure:"enable_cors"`
	AllowedOrigins    []string      `mapstructure:"allowed_origins"`
	TrustedProxies    []string      `mapstructure:"trusted_proxies"`
	EnableCompression bool          `mapstructure:"enable_compression"`
	EnablePprof       bool          `mapstructure:"enable_pprof"`
}

// DatabaseConfig contains database configuration
type DatabaseConfig struct {
	Driver          string        `mapstructure:"driver"`
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Database        string        `mapstructure:"database"`
	Username        string        `mapstructure:"username"`
	Password        string        `mapstructure:"password"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	LogLevel        string        `mapstructure:"log_level"`
	SlowQueryThreshold time.Duration `mapstructure:"slow_query_threshold"`
	AutoMigrate     bool          `mapstructure:"auto_migrate"`
}

// RedisConfig contains Redis configuration
type RedisConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Password        string        `mapstructure:"password"`
	Database        int           `mapstructure:"database"`
	MaxRetries      int           `mapstructure:"max_retries"`
	MinIdleConns    int           `mapstructure:"min_idle_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	DialTimeout     time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	PoolSize        int           `mapstructure:"pool_size"`
	EnableCluster   bool          `mapstructure:"enable_cluster"`
	ClusterNodes    []string      `mapstructure:"cluster_nodes"`
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	JWTSecret           string        `mapstructure:"jwt_secret"`
	JWTExpiration       time.Duration `mapstructure:"jwt_expiration"`
	RefreshExpiration   time.Duration `mapstructure:"refresh_expiration"`
	BCryptCost          int           `mapstructure:"bcrypt_cost"`
	EnableOAuth         bool          `mapstructure:"enable_oauth"`
	GoogleClientID      string        `mapstructure:"google_client_id"`
	GoogleClientSecret  string        `mapstructure:"google_client_secret"`
	FacebookAppID       string        `mapstructure:"facebook_app_id"`
	FacebookAppSecret   string        `mapstructure:"facebook_app_secret"`
	SessionSecret       string        `mapstructure:"session_secret"`
	SessionMaxAge       int           `mapstructure:"session_max_age"`
}

// AWSConfig contains AWS service configuration
type AWSConfig struct {
	Region          string `mapstructure:"region"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	SessionToken    string `mapstructure:"session_token"`
	Endpoint        string `mapstructure:"endpoint"`
	S3Bucket        string `mapstructure:"s3_bucket"`
	CloudFrontURL   string `mapstructure:"cloudfront_url"`
}

// AIConfig contains AI service configuration
type AIConfig struct {
	Provider           string  `mapstructure:"provider"`
	OpenAIKey          string  `mapstructure:"openai_key"`
	OpenAIModel        string  `mapstructure:"openai_model"`
	AnthropicKey       string  `mapstructure:"anthropic_key"`
	AnthropicModel     string  `mapstructure:"anthropic_model"`
	MaxTokens          int     `mapstructure:"max_tokens"`
	Temperature        float64 `mapstructure:"temperature"`
	TimeoutSeconds     int     `mapstructure:"timeout_seconds"`
	EnableCache        bool    `mapstructure:"enable_cache"`
	CacheTTL           time.Duration `mapstructure:"cache_ttl"`
}

// KafkaConfig contains Kafka configuration
type KafkaConfig struct {
	Brokers       []string `mapstructure:"brokers"`
	GroupID       string   `mapstructure:"group_id"`
	ClientID      string   `mapstructure:"client_id"`
	EnableSASL    bool     `mapstructure:"enable_sasl"`
	SASLUsername  string   `mapstructure:"sasl_username"`
	SASLPassword  string   `mapstructure:"sasl_password"`
	EnableTLS     bool     `mapstructure:"enable_tls"`
	RetryMax      int      `mapstructure:"retry_max"`
	RequiredAcks  int      `mapstructure:"required_acks"`
}

// MonitoringConfig contains monitoring configuration
type MonitoringConfig struct {
	EnableMetrics     bool     `mapstructure:"enable_metrics"`
	MetricsPort       int      `mapstructure:"metrics_port"`
	EnableTracing     bool     `mapstructure:"enable_tracing"`
	JaegerEndpoint    string   `mapstructure:"jaeger_endpoint"`
	SamplingRate      float64  `mapstructure:"sampling_rate"`
	EnableNewRelic    bool     `mapstructure:"enable_newrelic"`
	NewRelicLicense   string   `mapstructure:"newrelic_license"`
	NewRelicAppName   string   `mapstructure:"newrelic_app_name"`
	SentryDSN         string   `mapstructure:"sentry_dsn"`
	SentryEnvironment string   `mapstructure:"sentry_environment"`
	HealthCheckPath   string   `mapstructure:"health_check_path"`
	ReadinessPath     string   `mapstructure:"readiness_path"`
}

// EmailConfig contains email service configuration
type EmailConfig struct {
	Provider       string `mapstructure:"provider"`
	SMTPHost       string `mapstructure:"smtp_host"`
	SMTPPort       int    `mapstructure:"smtp_port"`
	SMTPUsername   string `mapstructure:"smtp_username"`
	SMTPPassword   string `mapstructure:"smtp_password"`
	FromAddress    string `mapstructure:"from_address"`
	FromName       string `mapstructure:"from_name"`
	SendGridAPIKey string `mapstructure:"sendgrid_api_key"`
	EnableTLS      bool   `mapstructure:"enable_tls"`
}

// StorageConfig contains file storage configuration
type StorageConfig struct {
	Provider        string `mapstructure:"provider"`
	LocalPath       string `mapstructure:"local_path"`
	MaxFileSize     int64  `mapstructure:"max_file_size"`
	AllowedTypes    []string `mapstructure:"allowed_types"`
	ImageMaxWidth   int    `mapstructure:"image_max_width"`
	ImageMaxHeight  int    `mapstructure:"image_max_height"`
	EnableCDN       bool   `mapstructure:"enable_cdn"`
	CDNBaseURL      string `mapstructure:"cdn_base_url"`
}

// RateLimitConfig contains rate limiting configuration
type RateLimitConfig struct {
	Enable          bool          `mapstructure:"enable"`
	RequestsPerMin  int           `mapstructure:"requests_per_min"`
	BurstSize       int           `mapstructure:"burst_size"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
	UseRedis        bool          `mapstructure:"use_redis"`
}

// FeatureFlags contains feature toggles
type FeatureFlags struct {
	EnableAIRecipes      bool `mapstructure:"enable_ai_recipes"`
	EnableSocialFeatures bool `mapstructure:"enable_social_features"`
	EnablePremium        bool `mapstructure:"enable_premium"`
	EnableAnalytics      bool `mapstructure:"enable_analytics"`
	EnableExport         bool `mapstructure:"enable_export"`
	MaintenanceMode      bool `mapstructure:"maintenance_mode"`
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	v := viper.New()
	
	// Set default values
	setDefaults(v)
	
	// Set config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/alchemorsel")
	}
	
	// Enable environment variable override
	v.SetEnvPrefix("ALCHEMORSEL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	
	// Read config file
	if err := v.ReadInConfig(); err != nil {
		// It's okay if config file doesn't exist, we have defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}
	
	// Unmarshal configuration
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// App defaults
	v.SetDefault("app.name", "Alchemorsel")
	v.SetDefault("app.version", "3.0.0")
	v.SetDefault("app.environment", "development")
	v.SetDefault("app.debug", false)
	v.SetDefault("app.log_level", "info")
	v.SetDefault("app.log_format", "json")
	
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "15s")
	v.SetDefault("server.idle_timeout", "60s")
	v.SetDefault("server.max_header_bytes", 1<<20) // 1MB
	v.SetDefault("server.shutdown_timeout", "30s")
	v.SetDefault("server.enable_cors", true)
	v.SetDefault("server.enable_compression", true)
	
	// Database defaults
	v.SetDefault("database.driver", "postgres")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "1h")
	v.SetDefault("database.conn_max_idle_time", "10m")
	v.SetDefault("database.slow_query_threshold", "100ms")
	
	// Redis defaults
	v.SetDefault("redis.host", "localhost")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.database", 0)
	v.SetDefault("redis.max_retries", 3)
	v.SetDefault("redis.pool_size", 10)
	
	// Auth defaults
	v.SetDefault("auth.jwt_expiration", "24h")
	v.SetDefault("auth.refresh_expiration", "168h") // 7 days
	v.SetDefault("auth.bcrypt_cost", 10)
	
	// Monitoring defaults
	v.SetDefault("monitoring.metrics_port", 9090)
	v.SetDefault("monitoring.sampling_rate", 0.1)
	v.SetDefault("monitoring.health_check_path", "/health")
	v.SetDefault("monitoring.readiness_path", "/ready")
	
	// Rate limit defaults
	v.SetDefault("rate_limit.requests_per_min", 60)
	v.SetDefault("rate_limit.burst_size", 10)
	v.SetDefault("rate_limit.cleanup_interval", "1m")
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate required fields
	if c.App.Name == "" {
		return fmt.Errorf("app.name is required")
	}
	
	if c.Database.Database == "" {
		return fmt.Errorf("database.database is required")
	}
	
	if c.Auth.JWTSecret == "" && c.App.Environment == "production" {
		return fmt.Errorf("auth.jwt_secret is required in production")
	}
	
	// Validate port ranges
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}
	
	return nil
}

// IsProduction returns true if running in production
func (c *Config) IsProduction() bool {
	return c.App.Environment == "production"
}

// IsDevelopment returns true if running in development
func (c *Config) IsDevelopment() bool {
	return c.App.Environment == "development"
}

// GetDSN returns the database connection string
func (c *Config) GetDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.Username,
		c.Database.Password,
		c.Database.Database,
		c.Database.SSLMode,
	)
}