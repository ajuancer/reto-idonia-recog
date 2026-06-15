//go:build windows

package logging

import (
	"context"
	"log/slog"

	"reto-idonia-recog-refactored/internal/config"
)

// MyTesseraSetup returns a dummy appender on Windows since POSIX storage is incompatible.
func MyTesseraSetup(cnf *config.Config) (TesseraAppender, error) {
	slog.Warn("Running on Windows: Tessera POSIX storage is disabled. Using a mock appender.")
	return &mockAppenderWrapper{}, nil
}

type mockAppenderWrapper struct{}

func (w *mockAppenderWrapper) Add(ctx context.Context, entry []byte) error {
	// Silently discard or just log the entry during local Windows development
	slog.Debug("Mock Tessera Add", "entry_len", len(entry))
	return nil
}
