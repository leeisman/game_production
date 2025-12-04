package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// contextKey is the type for context keys
type contextKey string

const (
	// RequestIDKey is the context key for request ID
	RequestIDKey contextKey = "request_id"
	// LoggerKey is the context key for logger
	LoggerKey contextKey = "logger"
)

// globalLogger is the default logger
var (
	globalLogger zerolog.Logger
	globalWriter *SmartWriter
)

// Config holds logger configuration
type Config struct {
	Level      string // debug, info, warn, error
	Format     string // json, console
	Output     io.Writer
	CallerSkip int
}

// InitWithFile initializes logger with console and file output
// It handles directory creation, log rotation, and multi-output setup.
func InitWithFile(filename string, level string, format string) {
	// Ensure directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}

	logFile := &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    100,  // megabytes
		MaxBackups: 3,    // keep 3 old files
		MaxAge:     28,   // days
		Compress:   true, // compress old files
	}

	// Use MultiWriter to write to both Console and File
	multiOutput := io.MultiWriter(os.Stdout, logFile)

	Init(Config{
		Level:  level,
		Format: format,
		Output: multiOutput,
	})
}

// Init initializes the global logger
func Init(cfg Config) {
	// Set log level
	level := parseLevel(cfg.Level)
	zerolog.SetGlobalLevel(level)

	// Set output
	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}

	// Always use SmartWriter:
	// - Buffered I/O (256KB) for performance (non-blocking mostly)
	// - Auto flush every 1s
	// - Immediate flush on Error/Fatal
	// - Preserves log order
	sw := NewSmartWriter(output, 1*time.Second)
	globalWriter = sw // Store for Flush()
	output = sw

	// Note: We should ideally call sw.Close() on shutdown,
	// but since logger is global, we rely on OS flush or manual Sync if needed.
	// For now, the periodic flush handles most cases.

	// Customize caller marshal function to show shorter path
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		short := file
		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				short = file[i+1:]
				break
			}
		}
		// Try to find parent directory for better context (e.g. gamesvr/main.go)
		// This is a simple heuristic
		count := 0
		for i := len(file) - 1; i > 0; i-- {
			if file[i] == '/' {
				count++
				if count == 2 {
					short = file[i+1:]
					break
				}
			}
		}
		return fmt.Sprintf("%s:%d", short, line)
	}

	// Create logger
	var logger zerolog.Logger
	if cfg.Format == "console" {
		// Console writer with colors (for development)
		// Note: If Async is enabled, output is already an AsyncWriter wrapping stdout.
		// We wrap it again with ConsoleWriter which writes to the AsyncWriter.
		consoleWriter := zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: "2006-01-02 15:04:05.000",
			FormatLevel: func(i interface{}) string {
				return strings.ToUpper(fmt.Sprintf("%-7s", i))
			},
			FormatCaller: func(i interface{}) string {
				return fmt.Sprintf("%-20s", i)
			},
			PartsOrder: []string{
				zerolog.TimestampFieldName,
				zerolog.LevelFieldName,
				zerolog.CallerFieldName,
				zerolog.MessageFieldName,
			},
		}
		logger = zerolog.New(consoleWriter).With().Timestamp().Caller().Logger()
	} else {
		// JSON format (for production)
		logger = zerolog.New(output).With().Timestamp().Caller().Logger()
	}

	globalLogger = logger
}

// Flush forces all buffered logs to be written to the underlying writer
func Flush() {
	if globalWriter != nil {
		_ = globalWriter.Sync()
	}
}

// parseLevel converts string level to zerolog.Level
func parseLevel(level string) zerolog.Level {
	switch level {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

// WithRequestID creates a new context with request ID
func WithRequestID(ctx context.Context, requestID string) context.Context {
	// Create a logger with request ID
	logger := globalLogger.With().Str("request_id", requestID).Logger()

	// Store both request ID and logger in context
	ctx = context.WithValue(ctx, RequestIDKey, requestID)
	ctx = context.WithValue(ctx, LoggerKey, &logger)

	return ctx
}

// FromContext extracts logger from context
// If no logger in context, returns global logger
func FromContext(ctx context.Context) *zerolog.Logger {
	if ctx == nil {
		return &globalLogger
	}

	if logger, ok := ctx.Value(LoggerKey).(*zerolog.Logger); ok && logger != nil {
		return logger
	}

	// If no logger in context, try to get request ID and create one
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		logger := globalLogger.With().Str("request_id", requestID).Logger()
		return &logger
	}

	return &globalLogger
}

// GetRequestID extracts request ID from context
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}

	return ""
}

// Debug logs a debug message
func Debug(ctx context.Context) *zerolog.Event {
	return FromContext(ctx).Debug()
}

// Info logs an info message
func Info(ctx context.Context) *zerolog.Event {
	return FromContext(ctx).Info()
}

// Warn logs a warning message
func Warn(ctx context.Context) *zerolog.Event {
	return FromContext(ctx).Warn()
}

// Error logs an error message
func Error(ctx context.Context) *zerolog.Event {
	return FromContext(ctx).Error()
}

// Fatal logs a fatal message and exits
func Fatal(ctx context.Context) *zerolog.Event {
	return FromContext(ctx).Fatal()
}

// WithFields adds fields to the context logger
func WithFields(ctx context.Context, fields map[string]interface{}) context.Context {
	logger := FromContext(ctx)

	event := logger.With()
	for k, v := range fields {
		event = event.Interface(k, v)
	}

	newLogger := event.Logger()
	return context.WithValue(ctx, LoggerKey, &newLogger)
}

// Global logger methods (for backward compatibility or when context is not available)

// DebugGlobal logs a debug message without context
func DebugGlobal() *zerolog.Event {
	return globalLogger.Debug()
}

// InfoGlobal logs an info message without context
func InfoGlobal() *zerolog.Event {
	return globalLogger.Info()
}

// WarnGlobal logs a warning message without context
func WarnGlobal() *zerolog.Event {
	return globalLogger.Warn()
}

// ErrorGlobal logs an error message without context
func ErrorGlobal() *zerolog.Event {
	return globalLogger.Error()
}

// FatalGlobal logs a fatal message and exits
func FatalGlobal() *zerolog.Event {
	return globalLogger.Fatal()
}
