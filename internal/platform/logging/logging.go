// Package logging configura el logger estructurado (slog) con el nivel que
// indica LOG_LEVEL, equivalente a los niveles de logging de la API en Java.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Setup crea el logger, lo fija como predeterminado y lo devuelve.
func Setup(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		lvl = slog.LevelDebug
	case "WARN", "WARNING":
		lvl = slog.LevelWarn
	case "ERROR":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
	slog.SetDefault(logger)
	return logger
}
