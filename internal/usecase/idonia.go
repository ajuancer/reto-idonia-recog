package usecase

import (
	"context"
	"sync"

	"reto-idonia-recog-refactored/internal/domain"
)

func (o *Orchestrator) uploadStagedFiles(ctx context.Context, uploads []domain.StagedUpload, meta domain.UploadMetadata) error {
	maxWorkers := workerLimit(len(uploads))
	sem := make(chan struct{}, maxWorkers)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	var errOnce sync.Once
	var firstErr error

	for _, upload := range uploads {
		wg.Add(1)
		go func(u domain.StagedUpload) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			err := o.uploadSingleFile(ctx, u, meta)
			if err != nil {
				errOnce.Do(func() {
					firstErr = err
					cancel()
				})
			}
		}(upload)
	}

	wg.Wait()
	return firstErr
}

func (o *Orchestrator) uploadSingleFile(ctx context.Context, upload domain.StagedUpload, meta domain.UploadMetadata) error {
	if upload.Ext == ".pdf" {
		return retryWithBackoff(ctx, func() error {
			_, err := o.idonia.UploadReport(ctx, upload.Path, meta)
			return err
		})
	}

	fileMeta, err := extractDICOMMetadata(upload.Path)
	if err != nil {
		fileMeta = meta
	}

	return retryWithBackoff(ctx, func() error {
		_, callErr := o.idonia.UploadDICOM(ctx, upload.Path, fileMeta)
		return callErr
	})
}
