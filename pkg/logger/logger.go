// Package logger provides structured logging functionality
// Using Uber Zap for high-performance, structured logging
package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Config holds logger configuration
type Config struct {
	Level       string
	Format      string
	Development bool
	OutputPaths []string
}

// New creates a new logger instance
func New(cfg Config) (*zap.Logger, error) {
	// Parse log level
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = zapcore.InfoLevel
	}
	
	// Configure encoder
	var encoderConfig zapcore.EncoderConfig
	if cfg.Development {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		encoderConfig = zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}
	
	// Choose encoder format
	var encoder zapcore.Encoder
	switch cfg.Format {
	case "console":
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	default:
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}
	
	// Configure output
	outputPaths := cfg.OutputPaths
	if len(outputPaths) == 0 {
		outputPaths = []string{"stdout"}
	}
	
	writeSyncer := zapcore.NewMultiWriteSyncer(
		zapcore.AddSync(os.Stdout),
	)
	
	// Create core
	core := zapcore.NewCore(encoder, writeSyncer, level)
	
	// Add caller info for development
	var options []zap.Option
	if cfg.Development {
		options = append(options, zap.Development(), zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	} else {
		options = append(options, zap.AddCaller())
	}
	
	// Create logger
	logger := zap.New(core, options...)
	
	return logger, nil
}