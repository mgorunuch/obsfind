// Package loggingutil provides utilities for working with loggers in the application,
// including context-based logger retrieval and management.
package loggingutil

import (
	"fmt"

	"github.com/rs/zerolog"
)

// ZerologAdapter adapts zerolog.Logger to our loggingutil.Logger interface
type ZerologAdapter struct {
	logger zerolog.Logger
}

// NewZerologAdapter creates a new adapter that wraps a zerolog.Logger
// and implements the Logger interface
func NewZerologAdapter(logger zerolog.Logger) *ZerologAdapter {
	return &ZerologAdapter{
		logger: logger,
	}
}

// Debug logs a debug message with optional key-value pairs
func (z *ZerologAdapter) Debug(msg string, keysAndValues ...interface{}) {
	logEvent := z.logger.Debug()
	z.addFields(logEvent, keysAndValues...)
	logEvent.Msg(msg)
}

// Info logs an informational message with optional key-value pairs
func (z *ZerologAdapter) Info(msg string, keysAndValues ...interface{}) {
	logEvent := z.logger.Info()
	z.addFields(logEvent, keysAndValues...)
	logEvent.Msg(msg)
}

// Warn logs a warning message with optional key-value pairs
func (z *ZerologAdapter) Warn(msg string, keysAndValues ...interface{}) {
	logEvent := z.logger.Warn()
	z.addFields(logEvent, keysAndValues...)
	logEvent.Msg(msg)
}

// Error logs an error message with optional key-value pairs
func (z *ZerologAdapter) Error(msg string, keysAndValues ...interface{}) {
	logEvent := z.logger.Error()
	z.addFields(logEvent, keysAndValues...)
	logEvent.Msg(msg)
}

// Fatal logs a fatal message with optional key-value pairs and then terminates the program
func (z *ZerologAdapter) Fatal(msg string, keysAndValues ...interface{}) {
	logEvent := z.logger.Fatal()
	z.addFields(logEvent, keysAndValues...)
	logEvent.Msg(msg)
}

// With returns a new logger with the given key-value pairs added to the logging context
func (z *ZerologAdapter) With(keysAndValues ...interface{}) Logger {
	newLogger := z.logger
	if len(keysAndValues) > 0 {
		ctx := newLogger.With()
		for i := 0; i < len(keysAndValues); i += 2 {
			key := fmt.Sprintf("%v", keysAndValues[i])
			if i+1 < len(keysAndValues) {
				ctx = ctx.Interface(key, keysAndValues[i+1])
			} else {
				ctx = ctx.Interface(key, "MISSING")
			}
		}
		newLogger = ctx.Logger()
	}
	return &ZerologAdapter{logger: newLogger}
}

// addFields adds the key-value pairs to the event
func (z *ZerologAdapter) addFields(event *zerolog.Event, keysAndValues ...interface{}) {
	for i := 0; i < len(keysAndValues); i += 2 {
		key := fmt.Sprintf("%v", keysAndValues[i])
		if i+1 < len(keysAndValues) {
			event.Interface(key, keysAndValues[i+1])
		} else {
			event.Interface(key, "MISSING")
		}
	}
}
