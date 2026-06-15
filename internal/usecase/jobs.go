package usecase

import (
	"context"
	"errors"
	"mime/multipart"

	"reto-idonia-recog-refactored/internal/domain"
)

func (o *Orchestrator) EnqueueJob(ctx context.Context, queue JobQueue, files []*multipart.FileHeader, cleanup func()) (domain.Job, error) {
	if queue == nil {
		return domain.Job{}, errors.New("job queue not configured")
	}
	return queue.Enqueue(ctx, files, cleanup)
}
