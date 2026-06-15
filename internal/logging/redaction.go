package logging

import (
	"context"
	"log/slog"
	"strings"
)

const redactedValue = "[REDACTED]"

type RedactingHandler struct {
	handler    slog.Handler
	redactKeys []string
}

func NewRedactingHandler(handler slog.Handler, redactKeys []string) *RedactingHandler {
	normalized := make([]string, 0, len(redactKeys))
	for _, key := range redactKeys {
		if value := normalizeKey(key); value != "" {
			normalized = append(normalized, value)
		}
	}
	return &RedactingHandler{handler: handler, redactKeys: normalized}
}

func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *RedactingHandler) Handle(ctx context.Context, record slog.Record) error {
	sanitized := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		sanitized.AddAttrs(h.sanitizeAttr(attr))
		return true
	})
	return h.handler.Handle(ctx, sanitized)
}

func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &RedactingHandler{
		handler:    h.handler.WithAttrs(h.sanitizeAttrs(attrs)),
		redactKeys: h.redactKeys,
	}
}

func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{
		handler:    h.handler.WithGroup(name),
		redactKeys: h.redactKeys,
	}
}

func (h *RedactingHandler) sanitizeAttrs(attrs []slog.Attr) []slog.Attr {
	sanitized := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		sanitized = append(sanitized, h.sanitizeAttr(attr))
	}
	return sanitized
}

func (h *RedactingHandler) sanitizeAttr(attr slog.Attr) slog.Attr {
	if h.isSensitive(attr.Key) {
		return slog.String(attr.Key, redactedValue)
	}
	if attr.Value.Kind() == slog.KindGroup {
		group := attr.Value.Group()
		sanitized := h.sanitizeAttrs(group)
		args := make([]any, 0, len(sanitized))
		for _, item := range sanitized {
			args = append(args, item)
		}
		return slog.Group(attr.Key, args...)
	}
	return attr
}

func (h *RedactingHandler) isSensitive(key string) bool {
	normalized := normalizeKey(key)
	if normalized == "" {
		return false
	}
	for _, sensitive := range h.redactKeys {
		if strings.Contains(normalized, sensitive) {
			return true
		}
	}
	return false
}
