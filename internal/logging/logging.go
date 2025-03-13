package logging

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger to provide structured logging
type Logger struct {
	*zap.SugaredLogger
}

// New creates a new Logger instance configured for production use
func New() *Logger {
	config := zap.NewProductionEncoderConfig()
	config.TimeKey = "ts"
	config.LevelKey = "level"
	config.MessageKey = "msg"
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncodeLevel = zapcore.LowercaseLevelEncoder
	config.EncodeDuration = zapcore.StringDurationEncoder

	// Disable development mode features
	config.StacktraceKey = ""
	config.CallerKey = ""
	config.NameKey = ""
	config.FunctionKey = ""

	// Determine log level from environment
	level := zap.InfoLevel
	if strings.ToLower(os.Getenv("LOG_LEVEL")) == "debug" {
		level = zap.DebugLevel
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(config),
		zapcore.AddSync(os.Stdout),
		level,
	)

	// Create logger without caller or stacktrace
	logger := zap.New(core)

	return &Logger{
		SugaredLogger: logger.Sugar(),
	}
}

// Info logs an info message with structured fields
func (l *Logger) Info(msg string, fields ...interface{}) {
	l.Infow(strings.ToLower(msg), fields...)
}

// Warn logs a warning message with structured fields
func (l *Logger) Warn(msg string, fields ...interface{}) {
	l.Warnw(strings.ToLower(msg), fields...)
}

// Error logs an error message with structured fields
func (l *Logger) Error(msg string, fields ...interface{}) {
	l.Errorw(strings.ToLower(msg), fields...)
}

// Debug logs a debug message with structured fields
func (l *Logger) Debug(msg string, fields ...interface{}) {
	l.Debugw(strings.ToLower(msg), fields...)
}
