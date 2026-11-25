package main

import (
	"context"
	"log/slog"
)

type ctxHandler struct {
	slog.Handler
}

func (h ctxHandler) Handle(ctx context.Context, rec slog.Record) error {
	if id := RequestIDFromContext(ctx); id != "" {
		rec.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, rec)
}
