package observe

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/zambone/pfm-go/internal/platform/ctxutil"
	"go.opentelemetry.io/otel/trace"
)

// NewLogger builds the application's structured JSON logger.
func NewLogger(level string, w io.Writer) *slog.Logger {
	if w == nil {
		w = os.Stdout
	}

	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		AddSource: true,
		Level:     parseLevel(level),
	})

	return slog.New(&contextHandler{next: handler})
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type contextHandler struct {
	next slog.Handler
}

func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *contextHandler) Handle(ctx context.Context, record slog.Record) error {
	record = addContextAttrs(ctx, record)
	return h.next.Handle(ctx, record)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{next: h.next.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{next: h.next.WithGroup(name)}
}

func addContextAttrs(ctx context.Context, record slog.Record) slog.Record {
	if ctx == nil {
		return record
	}

	if spanCtx := trace.SpanFromContext(ctx).SpanContext(); spanCtx.IsValid() {
		record.AddAttrs(slog.String("trace_id", spanCtx.TraceID().String()))
		record.AddAttrs(slog.String("span_id", spanCtx.SpanID().String()))
	} else if traceId, ok := ctxutil.TraceID(ctx); ok && traceId != "" {
		record.AddAttrs(slog.String("trace_id", traceId))
		record.AddAttrs(slog.String("span_id", ""))
	}

	if userId, ok := ctxutil.UserID(ctx); ok {
		record.AddAttrs(slog.String("user_id", userId.String()))
	}

	if householdId, ok := ctxutil.HouseholdID(ctx); ok {
		record.AddAttrs(slog.String("household_id", householdId.String()))
	}

	return record
}
