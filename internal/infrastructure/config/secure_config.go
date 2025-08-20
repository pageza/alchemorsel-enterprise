// Package config provides secure configuration management with secret integration
// Replaces hardcoded secrets with secure secret management
package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
	"internal/infrastructure/security/secrets"
)

// SecureConfig extends Config with secure secret management
type SecureConfig struct {
	*Config
	secretManager *secrets.SecretManager
	secretLoader  *secrets.SecretLoader
	logger        *slog.Logger
	initialized   bool
}

// SecureConfigOptions contains options for secure configuration
type SecureConfigOptions struct {
	ConfigPath           string
	EnableSecretManager  bool
	EnableAuditLogging  bool
	SecretManagerConfig *secrets.ManagerConfig
	LoaderConfig        *secrets.LoaderConfig
	Logger              *slog.Logger
}

// SecretField represents a field that should be loaded from secret manager
type SecretField struct {
	ConfigPath   string                `yaml:"config_path"`   // Path in configuration (e.g., "auth.jwt_secret")
	SecretName   string                `yaml:"secret_name"`   // Name of secret to load
	SecretType   secrets.SecretType    `yaml:"secret_type"`   // Type of secret
	Required     bool                  `yaml:"required"`      // Whether the secret is required
	DefaultValue string                `yaml:"default_value"` // Default value if secret not found
	Environment  string                `yaml:"environment"`   // Environment variable name
	Transform    string                `yaml:"transform"`     // Transformation to apply
}

// SecretMapping defines mapping between config fields and secrets
var DefaultSecretMappings = []SecretField{
	{
		ConfigPath:  "auth.jwt_secret",
		SecretName:  "jwt_secret",
		SecretType:  secrets.SecretTypeJWTKey,
		Required:    true,
		Environment: "ALCHEMORSEL_AUTH_JWT_SECRET",
	},
	{
		ConfigPath:  "auth.session_secret",
		SecretName:  "session_secret",
		SecretType:  secrets.SecretTypeSession,
		Required:    true,
		Environment: "ALCHEMORSEL_AUTH_SESSION_SECRET",
	},
	{
		ConfigPath:  "database.password",
		SecretName:  "database_password",
		SecretType:  secrets.SecretTypeDatabase,
		Required:    true,
		Environment: "ALCHEMORSEL_DATABASE_PASSWORD",
	},
	{
		ConfigPath:  "redis.password",
		SecretName:  "redis_password",
		SecretType:  secrets.SecretTypeDatabase,
		Required:    false,
		Environment: "ALCHEMORSEL_REDIS_PASSWORD",
	},
	{
		ConfigPath:  "auth.google_client_secret",
		SecretName:  "google_client_secret",
		SecretType:  secrets.SecretTypeOAuth,
		Required:    false,
		Environment: "ALCHEMORSEL_AUTH_GOOGLE_CLIENT_SECRET",
	},
	{
		ConfigPath:  "auth.facebook_app_secret",
		SecretName:  "facebook_app_secret",
		SecretType:  secrets.SecretTypeOAuth,
		Required:    false,
		Environment: "ALCHEMORSEL_AUTH_FACEBOOK_APP_SECRET",
	},
	{
		ConfigPath:  "aws.secret_access_key",
		SecretName:  "aws_secret_access_key",
		SecretType:  secrets.SecretTypeAPI,
		Required:    false,
		Environment: "ALCHEMORSEL_AWS_SECRET_ACCESS_KEY",
	},
	{
		ConfigPath:  "ai.openai_key",
		SecretName:  "openai_api_key",
		SecretType:  secrets.SecretTypeAPI,
		Required:    false,
		Environment: "ALCHEMORSEL_AI_OPENAI_KEY",
	},
	{
		ConfigPath:  "ai.anthropic_key",
		SecretName:  "anthropic_api_key",
		SecretType:  secrets.SecretTypeAPI,
		Required:    false,
		Environment: "ALCHEMORSEL_AI_ANTHROPIC_KEY",
	},
	{
		ConfigPath:  "kafka.sasl_password",
		SecretName:  "kafka_sasl_password",
		SecretType:  secrets.SecretTypeDatabase,
		Required:    false,
		Environment: "ALCHEMORSEL_KAFKA_SASL_PASSWORD",
	},
	{
		ConfigPath:  "monitoring.newrelic_license",
		SecretName:  "newrelic_license",
		SecretType:  secrets.SecretTypeAPI,
		Required:    false,
		Environment: "ALCHEMORSEL_MONITORING_NEWRELIC_LICENSE",
	},
	{
		ConfigPath:  "email.smtp_password",
		SecretName:  "smtp_password",
		SecretType:  secrets.SecretTypeAPI,
		Required:    false,
		Environment: "ALCHEMORSEL_EMAIL_SMTP_PASSWORD",
	},
	{
		ConfigPath:  "email.sendgrid_api_key",
		SecretName:  "sendgrid_api_key",
		SecretType:  secrets.SecretTypeAPI,
		Required:    false,
		Environment: "ALCHEMORSEL_EMAIL_SENDGRID_API_KEY",
	},
}

// LoadSecureConfig loads configuration with secure secret management
func LoadSecureConfig(ctx context.Context, options *SecureConfigOptions) (*SecureConfig, error) {
	if options == nil {
		options = getDefaultSecureConfigOptions()
	}

	// Load base configuration first
	baseConfig, err := Load(options.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load base configuration: %w", err)
	}

	// Create secure config wrapper
	secureConfig := &SecureConfig{
		Config: baseConfig,
		logger: options.Logger,
	}

	if secureConfig.logger == nil {
		secureConfig.logger = slog.Default()
	}

	// Initialize secret management if enabled
	if options.EnableSecretManager {
		if err := secureConfig.initializeSecretManager(ctx, options); err != nil {
			return nil, fmt.Errorf("failed to initialize secret manager: %w", err)
		}

		// Load secrets and replace hardcoded values
		if err := secureConfig.loadSecrets(ctx); err != nil {
			return nil, fmt.Errorf("failed to load secrets: %w", err)
		}
	}

	secureConfig.initialized = true
	secureConfig.logger.Info("Secure configuration loaded successfully")

	return secureConfig, nil
}

// initializeSecretManager initializes the secret manager and loader
func (sc *SecureConfig) initializeSecretManager(ctx context.Context, options *SecureConfigOptions) error {
	// Initialize secret manager
	managerConfig := options.SecretManagerConfig
	if managerConfig == nil {
		managerConfig = getDefaultManagerConfig()
	}

	var err error
	sc.secretManager, err = secrets.NewSecretManager(managerConfig)
	if err != nil {
		return fmt.Errorf("failed to create secret manager: %w", err)
	}

	// Initialize secret loader
	loaderConfig := options.LoaderConfig
	if loaderConfig == nil {
		loaderConfig = getDefaultLoaderConfig()
	}

	sc.secretLoader, err = secrets.NewSecretLoader(loaderConfig)
	if err != nil {
		return fmt.Errorf("failed to create secret loader: %w", err)
	}

	sc.logger.Info("Secret management initialized")
	return nil
}

// loadSecrets loads all secrets and replaces hardcoded configuration values
func (sc *SecureConfig) loadSecrets(ctx context.Context) error {
	for _, mapping := range DefaultSecretMappings {
		if err := sc.loadAndSetSecret(ctx, mapping); err != nil {
			if mapping.Required {
				return fmt.Errorf("failed to load required secret %s: %w", mapping.SecretName, err)
			}
			
			sc.logger.Warn("Failed to load optional secret",
				"secret_name", mapping.SecretName,
				"config_path", mapping.ConfigPath,
				"error", err,
			)
		}
	}

	return nil
}

// loadAndSetSecret loads a single secret and sets it in the configuration
func (sc *SecureConfig) loadAndSetSecret(ctx context.Context, mapping SecretField) error {
	// Create secret request
	req := &secrets.SecretRequest{
		Name:    mapping.SecretName,
		Type:    mapping.SecretType,
		Context: ctx,
		Tags: map[string]string{
			"config_path": mapping.ConfigPath,
			"required":    fmt.Sprintf("%t", mapping.Required),
		},
	}

	// Try to load from secret manager
	response, err := sc.secretManager.GetSecret(ctx, req)
	if err != nil {
		// If secret manager fails, try environment variable
		if mapping.Environment != "" {
			if envValue := getEnvValue(mapping.Environment); envValue != "" {
				return sc.setConfigValue(mapping.ConfigPath, envValue)
			}
		}

		// Use default value if available
		if mapping.DefaultValue != "" && !mapping.Required {
			return sc.setConfigValue(mapping.ConfigPath, mapping.DefaultValue)
		}

		return fmt.Errorf("secret not found in any source: %w", err)
	}

	// Transform the secret value if needed
	secretValue := string(response.Value)
	if mapping.Transform != "" {
		secretValue = sc.transformSecretValue(secretValue, mapping.Transform)
	}

	// Set the value in configuration
	if err := sc.setConfigValue(mapping.ConfigPath, secretValue); err != nil {
		return fmt.Errorf("failed to set config value: %w", err)
	}

	sc.logger.Debug("Secret loaded and set",
		"secret_name", mapping.SecretName,
		"config_path", mapping.ConfigPath,
		"secret_type", mapping.SecretType,
	)

	return nil
}

// setConfigValue sets a value in the configuration using reflection
func (sc *SecureConfig) setConfigValue(path, value string) error {
	pathParts := strings.Split(path, ".")
	if len(pathParts) != 2 {
		return fmt.Errorf("invalid config path format: %s", path)
	}

	configValue := reflect.ValueOf(sc.Config).Elem()
	
	// Get the struct field for the first part (e.g., "auth")
	structField := configValue.FieldByName(strings.Title(pathParts[0]))
	if !structField.IsValid() {
		return fmt.Errorf("invalid config section: %s", pathParts[0])
	}

	// Get the field within the struct (e.g., "jwt_secret")
	fieldName := ""
	structType := structField.Type()
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		tag := field.Tag.Get("mapstructure")
		if tag == pathParts[1] {
			fieldName = field.Name
			break
		}
	}

	if fieldName == "" {
		return fmt.Errorf("invalid config field: %s", pathParts[1])
	}

	targetField := structField.FieldByName(fieldName)
	if !targetField.IsValid() || !targetField.CanSet() {
		return fmt.Errorf("cannot set config field: %s", path)
	}

	// Set the value based on field type
	switch targetField.Kind() {
	case reflect.String:
		targetField.SetString(value)
	case reflect.Int, reflect.Int32, reflect.Int64:
		// Handle duration fields specially
		if strings.Contains(strings.ToLower(fieldName), "timeout") ||
		   strings.Contains(strings.ToLower(fieldName), "expiration") ||
		   strings.Contains(strings.ToLower(fieldName), "interval") {
			duration, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("invalid duration value %s for field %s: %w", value, path, err)
			}
			targetField.SetInt(int64(duration))
		} else {
			// Parse as regular integer
			intValue, err := parseInt(value)
			if err != nil {
				return fmt.Errorf("invalid integer value %s for field %s: %w", value, path, err)
			}
			targetField.SetInt(intValue)
		}
	case reflect.Bool:
		boolValue, err := parseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value %s for field %s: %w", value, path, err)
		}
		targetField.SetBool(boolValue)
	default:
		return fmt.Errorf("unsupported field type %s for path %s", targetField.Kind(), path)
	}

	return nil
}

// transformSecretValue applies transformations to secret values
func (sc *SecureConfig) transformSecretValue(value, transform string) string {
	switch transform {
	case "trim":
		return strings.TrimSpace(value)
	case "upper":
		return strings.ToUpper(value)
	case "lower":
		return strings.ToLower(value)
	case "base64_decode":
		// For now, return as-is. In production, implement proper base64 decoding
		return value
	default:
		return value
	}
}

// GetSecret retrieves a secret by name through the secret manager
func (sc *SecureConfig) GetSecret(ctx context.Context, name string, secretType secrets.SecretType) (string, error) {
	if !sc.initialized || sc.secretManager == nil {
		return "", fmt.Errorf("secret manager not initialized")
	}

	req := &secrets.SecretRequest{
		Name:    name,
		Type:    secretType,
		Context: ctx,
	}

	response, err := sc.secretManager.GetSecret(ctx, req)
	if err != nil {
		return "", err
	}

	return string(response.Value), nil
}

// StoreSecret stores a secret through the secret manager
func (sc *SecureConfig) StoreSecret(ctx context.Context, name string, value string, secretType secrets.SecretType) error {
	if !sc.initialized || sc.secretManager == nil {
		return fmt.Errorf("secret manager not initialized")
	}

	return sc.secretManager.StoreSecret(ctx, name, []byte(value), secretType)
}

// RotateSecret rotates a secret
func (sc *SecureConfig) RotateSecret(ctx context.Context, name string) error {
	if !sc.initialized || sc.secretManager == nil {
		return fmt.Errorf("secret manager not initialized")
	}

	return sc.secretManager.RotateSecret(ctx, name)
}

// ValidateSecrets validates all loaded secrets
func (sc *SecureConfig) ValidateSecrets(ctx context.Context) error {
	if !sc.initialized {
		return fmt.Errorf("secure config not initialized")
	}

	var validationErrors []string

	for _, mapping := range DefaultSecretMappings {
		if mapping.Required {
			value := sc.getConfigValue(mapping.ConfigPath)
			if value == "" {
				validationErrors = append(validationErrors, 
					fmt.Sprintf("required secret %s (%s) is empty", mapping.SecretName, mapping.ConfigPath))
			}
		}
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("secret validation failed: %s", strings.Join(validationErrors, "; "))
	}

	sc.logger.Info("All secrets validated successfully")
	return nil
}

// getConfigValue retrieves a configuration value by path
func (sc *SecureConfig) getConfigValue(path string) string {
	pathParts := strings.Split(path, ".")
	if len(pathParts) != 2 {
		return ""
	}

	configValue := reflect.ValueOf(sc.Config).Elem()
	structField := configValue.FieldByName(strings.Title(pathParts[0]))
	if !structField.IsValid() {
		return ""
	}

	// Find field by mapstructure tag
	structType := structField.Type()
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		tag := field.Tag.Get("mapstructure")
		if tag == pathParts[1] {
			targetField := structField.FieldByName(field.Name)
			if targetField.IsValid() && targetField.Kind() == reflect.String {
				return targetField.String()
			}
			break
		}
	}

	return ""
}

// Shutdown gracefully shuts down the secure configuration
func (sc *SecureConfig) Shutdown(ctx context.Context) error {
	if sc.secretManager != nil {
		if err := sc.secretManager.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown secret manager: %w", err)
		}
	}

	sc.logger.Info("Secure configuration shutdown completed")
	return nil
}

// Helper functions

// getEnvValue gets environment variable value
func getEnvValue(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

// parseInt parses integer from string
func parseInt(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

// parseBool parses boolean from string
func parseBool(s string) (bool, error) {
	return strconv.ParseBool(strings.TrimSpace(s))
}

// getDefaultSecureConfigOptions returns default options
func getDefaultSecureConfigOptions() *SecureConfigOptions {
	return &SecureConfigOptions{
		ConfigPath:          "",
		EnableSecretManager: true,
		EnableAuditLogging:  true,
		Logger:              slog.Default(),
	}
}

// getDefaultManagerConfig returns default secret manager config
func getDefaultManagerConfig() *secrets.ManagerConfig {
	return &secrets.ManagerConfig{
		EnableCache:           true,
		CacheTTL:             time.Hour,
		MaxCacheSize:         1000,
		EnableAuditLogging:   true,
		RotationInterval:     24 * time.Hour,
		EncryptionAlgorithm:  "AES-256-GCM",
		EnableZeroization:    true,
		MasterKeyDerivation: secrets.KeyDerivationConfig{
			Time:    1,
			Memory:  64 * 1024,
			Threads: 4,
			KeyLen:  32,
		},
	}
}

// getDefaultLoaderConfig returns default secret loader config
func getDefaultLoaderConfig() *secrets.LoaderConfig {
	return &secrets.LoaderConfig{
		EnableEnvVars:       true,
		EnableVault:         false,
		EnableKubernetes:    false,
		EnableAWSSM:         false,
		EnvPrefix:           "ALCHEMORSEL",
		DefaultProvider:     "env",
		ProviderConfigs:     make(map[string]interface{}),
		ValidationRules:     getDefaultValidationRules(),
		CacheTTL:           time.Hour,
		EnableValidation:   true,
		EnableTransformation: true,
		SecurityPolicy: secrets.SecurityPolicy{
			RequireEncryption:    false,
			AllowedProviders:     []string{"env", "vault", "kubernetes", "aws-sm"},
			RequireValidation:    true,
			MaxSecretSize:        1024 * 1024,
			EnforceAccessControl: true,
		},
	}
}

// getDefaultValidationRules returns default validation rules
func getDefaultValidationRules() map[string]secrets.ValidationRule {
	return map[string]secrets.ValidationRule{
		"jwt_key": {
			MinLength: 32,
			MaxLength: 512,
			Pattern:   `^[A-Za-z0-9+/=_-]+$`,
			Required:  true,
		},
		"database": {
			MinLength: 8,
			MaxLength: 128,
			Required:  true,
		},
		"api_key": {
			MinLength: 16,
			MaxLength: 512,
			Pattern:   `^[A-Za-z0-9_-]+$`,
			Required:  true,
		},
		"session": {
			MinLength: 32,
			MaxLength: 128,
			Required:  true,
		},
		"oauth": {
			MinLength: 16,
			MaxLength: 256,
			Required:  false,
		},
		"generic": {
			MinLength: 1,
			MaxLength: 1024,
			Required:  false,
		},
	}
}