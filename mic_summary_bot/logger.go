package micsummarybot

import (
	"log/slog"
	"os"
)

var pkgLogger *slog.Logger
var currentHandlerOptions *slog.HandlerOptions

func init() {
	// Default logger for the package, can be overridden by SetLogger
	currentHandlerOptions = &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	pkgLogger = slog.New(slog.NewTextHandler(os.Stdout, currentHandlerOptions))
}

// SetLogger allows an external package to set the logger for this package.
func SetLogger(l *slog.Logger) {
	pkgLogger = l
}

// SetLogLevel allows setting the log level for the package logger.
func SetLogLevel(level slog.Level) {
	currentHandlerOptions.Level = level
	pkgLogger = slog.New(slog.NewTextHandler(os.Stdout, currentHandlerOptions))
}
