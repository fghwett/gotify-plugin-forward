package main

import (
	"log/slog"
	"os"
)

func NewLogger() *slog.Logger {
	handler := slog.NewTextHandler(os.Stdout, nil).WithGroup(PluginName)
	logger := slog.New(handler)

	return logger
}
