package logger

import (
	"log/slog"
	"os"
)

type Config interface {
	Format() string
	Level() string
}

var logLevelMap = map[string]slog.Level{
	"debug":   slog.LevelDebug,
	"info":    slog.LevelInfo,
	"warn":    slog.LevelWarn,
	"warning": slog.LevelWarn,
	"error":   slog.LevelError,
}

func Configure(cfg Config) {
	var handler slog.Handler

	level := logLevelMap[cfg.Level()]

	handlerOptions := &slog.HandlerOptions{AddSource: level == slog.LevelDebug, Level: level}

	switch cfg.Format() {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, handlerOptions)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, handlerOptions)
	default:
		handler = slog.NewJSONHandler(os.Stdout, handlerOptions)
	}

	logger := slog.New(handler)

	slog.SetDefault(logger)
}
