package logger

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// GormLogger adapts our zerolog logger to gorm logger interface
type GormLogger struct {
	SlowThreshold time.Duration
	LogLevel      gormlogger.LogLevel
}

// NewGormLogger creates a new GormLogger
func NewGormLogger() *GormLogger {
	return &GormLogger{
		SlowThreshold: 200 * time.Millisecond, // Default slow query threshold
		LogLevel:      gormlogger.Warn,        // Default log level
	}
}

// LogMode sets the log level
func (l *GormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info logs info messages
func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Info {
		Info(ctx).Msgf(msg, data...)
	}
}

// Warn logs warn messages
func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Warn {
		Warn(ctx).Msgf(msg, data...)
	}
}

// Error logs error messages
func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Error {
		Error(ctx).Msgf(msg, data...)
	}
}

// Trace logs sql queries
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	// Prepare the log event
	var event *zerolog.Event

	// Check for errors
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		if l.LogLevel >= gormlogger.Error {
			event = Error(ctx).Err(err)
		}
	} else if elapsed > l.SlowThreshold && l.SlowThreshold != 0 {
		// Check for slow queries
		if l.LogLevel >= gormlogger.Warn {
			event = Warn(ctx).Str("slow_query", "true")
		}
	} else {
		// Normal queries (only log if level is Info)
		if l.LogLevel >= gormlogger.Info {
			event = Info(ctx)
		}
	}

	// If event is set (meaning we should log this), add fields and send
	if event != nil {
		event.
			Str("sql", sql).
			Float64("elapsed_ms", float64(elapsed.Nanoseconds())/1e6).
			Int64("rows", rows).
			Msg("GORM query")
	}
}
