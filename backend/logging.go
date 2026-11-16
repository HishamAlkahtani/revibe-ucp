package main

import (
	"context"
	"log/slog"
)

func requestLog(_ context.Context) *slog.Logger {
	return slog.Default()
}
