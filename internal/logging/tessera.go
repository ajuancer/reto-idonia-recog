package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// TesseraAppender defines the interface for writing to the audit ledger.
type TesseraAppender interface {
	Add(ctx context.Context, entry []byte) error
}

// AsyncAppender wraps any TesseraAppender to make its Add method non-blocking.
type AsyncAppender struct {
	backend TesseraAppender
	ch      chan []byte
	done    chan struct{}
}

// NewAsyncAppender creates a buffered, non-blocking appender.
func NewAsyncAppender(backend TesseraAppender, bufferSize int) *AsyncAppender {
	a := &AsyncAppender{
		backend: backend,
		ch:      make(chan []byte, bufferSize),
		done:    make(chan struct{}),
	}
	go a.start()
	return a
}

func (a *AsyncAppender) start() {
	for bytes := range a.ch {
		// We use context.Background() here because the original request
		// context from the logger might be canceled by the time the worker processes this.
		_ = a.backend.Add(context.Background(), bytes)
	}
	close(a.done)
}

// Add pushes the log to the channel. If the buffer is full, it drops the log to prevent blocking.
func (a *AsyncAppender) Add(ctx context.Context, entry []byte) error {
	select {
	case a.ch <- entry:
		return nil
	default:
		return fmt.Errorf("tessera async buffer full, dropping audit log")
	}
}

// Close gracefully shuts down the background worker, flushing remaining logs.
func (a *AsyncAppender) Close() {
	close(a.ch)
	<-a.done
}

// TesseraHandler is a middleware handler for slog.
type TesseraHandler struct {
	next     slog.Handler
	appender TesseraAppender
	attrs    []slog.Attr // Tracks context added via With()
	groups   []string    // Tracks groups added via WithGroup()
}

func NewTesseraHandler(next slog.Handler, appender TesseraAppender) *TesseraHandler {
	return &TesseraHandler{
		next:     next,
		appender: appender,
	}
}

func (h *TesseraHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *TesseraHandler) Handle(ctx context.Context, r slog.Record) error {
	// Let the core handler do its job first
	err := h.next.Handle(ctx, r)

	if r.Level >= slog.LevelWarn || h.hasAuditFlag(r) {
		h.hashLogToTessera(ctx, r)
	}

	return err
}

func (h *TesseraHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := append(h.attrs[:len(h.attrs):len(h.attrs)], attrs...)
	return &TesseraHandler{
		next:     h.next.WithAttrs(attrs),
		appender: h.appender,
		attrs:    newAttrs,
		groups:   h.groups,
	}
}

func (h *TesseraHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	newGroups := append(h.groups[:len(h.groups):len(h.groups)], name)
	return &TesseraHandler{
		next:     h.next.WithGroup(name),
		appender: h.appender,
		attrs:    h.attrs,
		groups:   newGroups,
	}
}

func (h *TesseraHandler) hashLogToTessera(ctx context.Context, r slog.Record) {
	logData := map[string]any{
		"time":    r.Time.Format(time.RFC3339Nano),
		"level":   r.Level.String(),
		"message": r.Message,
	}

	prefix := ""
	if len(h.groups) > 0 {
		prefix = strings.Join(h.groups, ".") + "."
	}

	for _, a := range h.attrs {
		logData[prefix+a.Key] = a.Value.Any()
	}

	r.Attrs(func(a slog.Attr) bool {
		logData[prefix+a.Key] = a.Value.Any()
		return true
	})

	if bytes, err := json.Marshal(logData); err == nil {
		_ = h.appender.Add(ctx, bytes)
	}
}

func (h *TesseraHandler) hasAuditFlag(r slog.Record) bool {
	isAudit := false

	// Check stored attributes first
	for _, a := range h.attrs {
		if a.Key == "audit" {
			isAudit = a.Value.Bool()
		}
	}

	// Check record attributes (record overrides stored)
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "audit" {
			isAudit = a.Value.Bool()
		}
		return true
	})

	return isAudit
}
