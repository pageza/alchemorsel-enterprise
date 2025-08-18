package monitoring

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogConfig holds logging configuration
type LogConfig struct {
	Level       string
	Format      string // "json" or "console"
	Output      string // "stdout", "stderr", or file path
	ServiceName string
	Environment string
	Version     string
}

// Logger wraps zap logger with correlation ID support
type Logger struct {
	*zap.Logger
	config LogConfig
}

// NewLogger creates a new structured logger with correlation ID support
func NewLogger(config LogConfig) (*Logger, error) {
	// Parse log level
	level, err := zapcore.ParseLevel(config.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level '%s': %w", config.Level, err)
	}

	// Configure encoder
	var encoderConfig zapcore.EncoderConfig
	var encoder zapcore.Encoder

	if config.Format == "json" {
		encoderConfig = zap.NewProductionEncoderConfig()
		encoderConfig.TimeKey = "timestamp"
		encoderConfig.MessageKey = "message"
		encoderConfig.LevelKey = "level"
		encoderConfig.CallerKey = "caller"
		encoderConfig.StacktraceKey = "stacktrace"
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05")
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Configure output
	var writeSyncer zapcore.WriteSyncer
	switch config.Output {
	case "stdout", "":
		writeSyncer = zapcore.AddSync(os.Stdout)
	case "stderr":
		writeSyncer = zapcore.AddSync(os.Stderr)
	default:
		file, err := os.OpenFile(config.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file '%s': %w", config.Output, err)
		}
		writeSyncer = zapcore.AddSync(file)
	}

	// Create core
	core := zapcore.NewCore(encoder, writeSyncer, level)

	// Create logger with caller information
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	// Add global fields
	logger = logger.With(
		zap.String("service", config.ServiceName),
		zap.String("environment", config.Environment),
		zap.String("version", config.Version),
	)

	return &Logger{
		Logger: logger,
		config: config,
	}, nil
}

// WithCorrelationID adds correlation ID fields to logger
func (l *Logger) WithCorrelationID(ctx context.Context) *zap.Logger {
	traceID := TraceIDFromContext(ctx)
	spanID := SpanIDFromContext(ctx)
	
	fields := []zap.Field{}
	if traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	if spanID != "" {
		fields = append(fields, zap.String("span_id", spanID))
	}
	
	return l.Logger.With(fields...)
}

// WithRequestID adds request ID to logger (if available in context)
func (l *Logger) WithRequestID(ctx context.Context) *zap.Logger {
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		return l.Logger.With(zap.String("request_id", requestID))
	}
	return l.Logger
}

// WithUserID adds user ID to logger (if available in context)
func (l *Logger) WithUserID(ctx context.Context) *zap.Logger {
	if userID := UserIDFromContext(ctx); userID != "" {
		return l.Logger.With(zap.String("user_id", userID))
	}
	return l.Logger
}

// WithContext adds all available context fields to logger
func (l *Logger) WithContext(ctx context.Context) *zap.Logger {
	logger := l.Logger
	
	// Add trace/span IDs
	if traceID := TraceIDFromContext(ctx); traceID != "" {
		logger = logger.With(zap.String("trace_id", traceID))
	}
	if spanID := SpanIDFromContext(ctx); spanID != "" {
		logger = logger.With(zap.String("span_id", spanID))
	}
	
	// Add request ID
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		logger = logger.With(zap.String("request_id", requestID))
	}
	
	// Add user ID
	if userID := UserIDFromContext(ctx); userID != "" {
		logger = logger.With(zap.String("user_id", userID))
	}
	
	return logger
}

// HTTPRequestLogger logs HTTP request details
func (l *Logger) HTTPRequestLogger(ctx context.Context, method, path, userAgent, clientIP string, statusCode int, duration time.Duration, size int64) {
	logger := l.WithContext(ctx)
	
	fields := []zap.Field{
		zap.String("method", method),
		zap.String("path", path),
		zap.String("user_agent", userAgent),
		zap.String("client_ip", clientIP),
		zap.Int("status_code", statusCode),
		zap.Duration("duration", duration),
		zap.Int64("response_size", size),
	}
	
	if statusCode >= 500 {
		logger.Error("HTTP request completed with server error", fields...)
	} else if statusCode >= 400 {
		logger.Warn("HTTP request completed with client error", fields...)
	} else {
		logger.Info("HTTP request completed", fields...)
	}
}

// DatabaseQueryLogger logs database query details
func (l *Logger) DatabaseQueryLogger(ctx context.Context, operation, table, query string, duration time.Duration, err error) {
	logger := l.WithContext(ctx)
	
	fields := []zap.Field{
		zap.String("operation", operation),
		zap.String("table", table),
		zap.String("query", query),
		zap.Duration("duration", duration),
	}
	
	if err != nil {
		logger.Error("Database query failed", append(fields, zap.Error(err))...)
	} else {
		logger.Info("Database query completed", fields...)
	}
}

// AIRequestLogger logs AI service request details
func (l *Logger) AIRequestLogger(ctx context.Context, provider, model, operation string, duration time.Duration, tokensUsed int, err error) {
	logger := l.WithContext(ctx)
	
	fields := []zap.Field{
		zap.String("provider", provider),
		zap.String("model", model),
		zap.String("operation", operation),
		zap.Duration("duration", duration),
		zap.Int("tokens_used", tokensUsed),
	}
	
	if err != nil {
		logger.Error("AI request failed", append(fields, zap.Error(err))...)
	} else {
		logger.Info("AI request completed", fields...)
	}
}

// CacheOperationLogger logs cache operation details
func (l *Logger) CacheOperationLogger(ctx context.Context, operation, key string, hit bool, duration time.Duration, err error) {
	logger := l.WithContext(ctx)
	
	fields := []zap.Field{
		zap.String("operation", operation),
		zap.String("key", key),
		zap.Bool("hit", hit),
		zap.Duration("duration", duration),
	}
	
	if err != nil {
		logger.Error("Cache operation failed", append(fields, zap.Error(err))...)
	} else {
		logger.Debug("Cache operation completed", fields...)
	}
}

// BusinessEventLogger logs business events
func (l *Logger) BusinessEventLogger(ctx context.Context, event, entityType, entityID string, metadata map[string]interface{}) {
	logger := l.WithContext(ctx)
	
	fields := []zap.Field{
		zap.String("event", event),
		zap.String("entity_type", entityType),
		zap.String("entity_id", entityID),
		zap.Any("metadata", metadata),
	}
	
	logger.Info("Business event occurred", fields...)
}

// SecurityEventLogger logs security-related events
func (l *Logger) SecurityEventLogger(ctx context.Context, event, userID, clientIP, userAgent string, severity string, details map[string]interface{}) {
	logger := l.WithContext(ctx)
	
	fields := []zap.Field{
		zap.String("event", event),
		zap.String("user_id", userID),
		zap.String("client_ip", clientIP),
		zap.String("user_agent", userAgent),
		zap.String("severity", severity),
		zap.Any("details", details),
	}
	
	switch severity {
	case "critical", "high":
		logger.Error("Security event detected", fields...)
	case "medium":
		logger.Warn("Security event detected", fields...)
	default:
		logger.Info("Security event detected", fields...)
	}
}

// PerformanceLogger logs performance metrics
func (l *Logger) PerformanceLogger(ctx context.Context, operation string, duration time.Duration, metadata map[string]interface{}) {
	logger := l.WithContext(ctx)
	
	fields := []zap.Field{
		zap.String("operation", operation),
		zap.Duration("duration", duration),
		zap.Any("metadata", metadata),
	}
	
	if duration > time.Second {
		logger.Warn("Slow operation detected", fields...)
	} else {
		logger.Debug("Performance measurement", fields...)
	}
}

// Context helper functions for extracting IDs from context
func RequestIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value("request_id").(string); ok {
		return requestID
	}
	return ""
}

func UserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value("user_id").(string); ok {
		return userID
	}
	return ""
}

// Context creation helpers
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, "request_id", requestID)
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, "user_id", userID)
}