// Package loggingutil provides utilities for working with loggers in the application,
// including context-based logger retrieval and management.
package loggingutil

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"obsfind/src/pkg/contextutil"
)

// Logger is the interface that defines the common logging operations.
// It provides methods for logging at different severity levels.
type Logger interface {
	// Debug logs a debug message with optional key-value pairs.
	Debug(msg string, keysAndValues ...interface{})

	// Info logs an informational message with optional key-value pairs.
	Info(msg string, keysAndValues ...interface{})

	// Warn logs a warning message with optional key-value pairs.
	Warn(msg string, keysAndValues ...interface{})

	// Error logs an error message with optional key-value pairs.
	Error(msg string, keysAndValues ...interface{})

	// Fatal logs a fatal message with optional key-value pairs and then terminates the program.
	Fatal(msg string, keysAndValues ...interface{})

	// With returns a new logger with the given key-value pairs added to the logging context.
	With(keysAndValues ...interface{}) Logger
}

// DefaultLogger is a simple implementation of the Logger interface
// that uses the standard Go log package.
type DefaultLogger struct {
	logger *log.Logger
	prefix string
}

// NewDefaultLogger creates a new DefaultLogger that writes to the given writer.
// If writer is nil, os.Stderr is used.
func NewDefaultLogger(writer io.Writer, prefix string) *DefaultLogger {
	if writer == nil {
		writer = os.Stderr
	}
	return &DefaultLogger{
		logger: log.New(writer, "", log.LstdFlags),
		prefix: prefix,
	}
}

// Debug logs a debug message with the DEBUG level.
func (l *DefaultLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.log("DEBUG", msg, keysAndValues...)
}

// Info logs an informational message with the INFO level.
func (l *DefaultLogger) Info(msg string, keysAndValues ...interface{}) {
	l.log("INFO", msg, keysAndValues...)
}

// Warn logs a warning message with the WARN level.
func (l *DefaultLogger) Warn(msg string, keysAndValues ...interface{}) {
	l.log("WARN", msg, keysAndValues...)
}

// Error logs an error message with the ERROR level.
func (l *DefaultLogger) Error(msg string, keysAndValues ...interface{}) {
	l.log("ERROR", msg, keysAndValues...)
}

// Fatal logs a fatal message with the FATAL level and then terminates the program.
func (l *DefaultLogger) Fatal(msg string, keysAndValues ...interface{}) {
	l.log("FATAL", msg, keysAndValues...)
	os.Exit(1)
}

// With returns a new logger with the given key-value pairs added to the logging context.
func (l *DefaultLogger) With(keysAndValues ...interface{}) Logger {
	if len(keysAndValues) == 0 {
		return l
	}

	prefix := l.prefix
	if prefix != "" {
		prefix += " "
	}

	for i := 0; i < len(keysAndValues); i += 2 {
		key := fmt.Sprintf("%v", keysAndValues[i])
		var value interface{} = "MISSING"
		if i+1 < len(keysAndValues) {
			value = keysAndValues[i+1]
		}
		prefix += fmt.Sprintf("[%s=%v] ", key, value)
	}

	return &DefaultLogger{
		logger: l.logger,
		prefix: prefix,
	}
}

// log is an internal helper method that formats and writes the log message.
func (l *DefaultLogger) log(level, msg string, keysAndValues ...interface{}) {
	prefix := l.prefix
	if prefix != "" {
		prefix = " " + prefix
	}

	var kvStr string
	if len(keysAndValues) > 0 {
		kvStr = " "
		for i := 0; i < len(keysAndValues); i += 2 {
			key := fmt.Sprintf("%v", keysAndValues[i])
			var value interface{} = "MISSING"
			if i+1 < len(keysAndValues) {
				value = keysAndValues[i+1]
			}
			kvStr += fmt.Sprintf("[%s=%v] ", key, value)
		}
	}

	l.logger.Printf("[%s]%s %s%s", level, prefix, msg, kvStr)
}

var (
	// defaultLogger is the default logger used if none is specified in the context
	defaultLogger     Logger
	defaultLoggerOnce sync.Once
)

// getDefaultLogger returns the default logger, initializing it if necessary
func getDefaultLogger() Logger {
	defaultLoggerOnce.Do(func() {
		defaultLogger = NewDefaultLogger(os.Stderr, "")
	})
	return defaultLogger
}

// Set stores a logger in the given context and returns a new context with the logger.
func Set(ctx context.Context, logger Logger) context.Context {
	return contextutil.SetTyped(ctx, logger)
}

// Get retrieves the logger from the context.
// If no logger is found in the context, a default logger is returned.
func Get(ctx context.Context) Logger {
	logger, ok := contextutil.TryRetrieveTyped[Logger](ctx)
	if !ok {
		return getDefaultLogger()
	}
	return logger
}

// MustGet retrieves the logger from the context and panics if none is found.
// This should be used in cases where a logger is required to be present.
func MustGet(ctx context.Context) Logger {
	return contextutil.RetrieveTyped[Logger](ctx)
}

// WithLogger returns a new context with the given logger stored in it.
// This is a convenience wrapper around Set.
func WithLogger(ctx context.Context, logger Logger) context.Context {
	return Set(ctx, logger)
}

// WithLoggerFromContext returns a new context with the logger from the source context.
// If no logger is found in the source context, a default logger is used.
func WithLoggerFromContext(dstCtx, srcCtx context.Context) context.Context {
	return Set(dstCtx, Get(srcCtx))
}
