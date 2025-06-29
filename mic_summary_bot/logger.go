package micsummarybot

import (
	"log/slog"
	"os"
)

var pkgLogger *slog.Logger

func init() {
	// Default logger for the package, can be overridden by SetLogger
	handlerOptions := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	pkgLogger = slog.New(slog.NewTextHandler(os.Stdout, handlerOptions))
}

// SetLogger allows an external package to set the logger for this package.
func SetLogger(l *slog.Logger) {
	pkgLogger = l
}
