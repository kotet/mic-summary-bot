package micsummarybot

import (
	"log/slog"
	"os"
)

var pkgLogger *slog.Logger
var pkgLogLevel *slog.LevelVar

func init() {
	// Create a LevelVar for dynamic log level control
	pkgLogLevel = new(slog.LevelVar)
	pkgLogLevel.Set(slog.LevelInfo) // Default level

	// Default logger for the package, can be overridden by SetLogger
	handlerOptions := &slog.HandlerOptions{
		Level: pkgLogLevel,
	}
	pkgLogger = slog.New(slog.NewTextHandler(os.Stdout, handlerOptions))
}

// SetLogger allows an external package to set the logger for this package.
// Note: If you want to continue using SetLogLevel after calling SetLogger,
// you should create your custom logger with a LevelVar that can be controlled externally.
func SetLogger(l *slog.Logger) {
	pkgLogger = l
}

// SetLogLevel allows setting the log level dynamically without recreating the logger.
// This is safe to call concurrently. Note that this only works with the default
// package logger or custom loggers that were designed to work with this function.
func SetLogLevel(level slog.Level) {
	pkgLogLevel.Set(level)
}
