package usecase

import (
	"errors"
	"log/slog"
	"mime/multipart"
	"os"
	"sync"

	"reto-idonia-recog-refactored/internal/domain"
)

// stageUploads manages the concurrent worker pool to stage multiple files.
func (o *Orchestrator) stageUploads(fileHeaders []*multipart.FileHeader) ([]domain.StagedUpload, error) {
	var staged []domain.StagedUpload
	var mu sync.Mutex
	var wg sync.WaitGroup

	jobs := make(chan *multipart.FileHeader, len(fileHeaders))
	workerCount := workerLimit(len(fileHeaders))

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for header := range jobs {
				upload, err := stageFile(header)
				if err != nil {
					slog.Warn("Skipping invalid or unstageable file", "filename", header.Filename, "error", err)
					continue
				}
				mu.Lock()
				staged = append(staged, upload)
				mu.Unlock()
			}
		}()
	}

	for _, header := range fileHeaders {
		jobs <- header
	}
	close(jobs)
	wg.Wait()

	if len(staged) == 0 {
		return nil, errors.New("no valid files could be processed")
	}
	return staged, nil
}

// cleanupStagedUploads safely deletes the temporary files from the OS once the workflow finishes.
func (o *Orchestrator) cleanupStagedUploads(uploads []domain.StagedUpload) {
	for _, upload := range uploads {
		if err := removeFile(upload.Path); err != nil && !os.IsNotExist(err) {
			slog.Warn("Failed to cleanup staged upload", "path", upload.Path, "error", err)
		}
	}
}
