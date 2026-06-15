//go:build !windows
package logging

import (
	"context"
	"fmt"
	"os"
	"reto-idonia-recog-refactored/internal/config"
	"time"

	"github.com/transparency-dev/tessera"
	"github.com/transparency-dev/tessera/storage/posix"
	"golang.org/x/mod/sumdb/note"
)

func MyTesseraSetup(cnf *config.Config) (TesseraAppender, error) {
	if err := os.MkdirAll(cnf.TesseraLogDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create tessera log dir: %w", err)
	}

	ctx := context.Background()

	storage, err := posix.New(ctx, posix.Config{Path: cnf.TesseraLogDir})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tessera posix storage: %w", err)
	}

	signer, err := note.NewSigner(cnf.TesseraPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate signer: %w", err)
	}

	opts := tessera.NewAppendOptions().
		WithCheckpointSigner(signer).
		WithCheckpointInterval(time.Second).
		WithBatching(256, time.Second)

	appender, _, _, err := tessera.NewAppender(ctx, storage, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create new appender: %w", err)
	}

	return &posixAppenderWrapper{
		appender: appender,
	}, nil
}

type posixAppenderWrapper struct {
	appender *tessera.Appender
}

func (w *posixAppenderWrapper) Add(ctx context.Context, entry []byte) error {
	_, err := w.appender.Add(ctx, tessera.NewEntry(entry))()
	return err
}
