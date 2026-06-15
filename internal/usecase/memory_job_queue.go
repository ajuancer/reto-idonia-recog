package usecase

import (
	"context"
	"mime/multipart"
	"sync"

	"github.com/google/uuid"

	"reto-idonia-recog-refactored/internal/domain"
)

type JobTask struct {
	JobID   string
	Files   []*multipart.FileHeader
	Cleanup func()
}

type JobQueue interface {
	Enqueue(ctx context.Context, files []*multipart.FileHeader, cleanup func()) (domain.Job, error)
	Tasks() <-chan JobTask
	Get(jobID string) (domain.Job, bool)
	Save(job domain.Job)
}

type InMemoryJobQueue struct {
	tasks chan JobTask
	jobs  sync.Map
}

func NewInMemoryJobQueue(buffer int) *InMemoryJobQueue {
	if buffer < 1 {
		buffer = 1
	}
	return &InMemoryJobQueue{
		tasks: make(chan JobTask, buffer),
	}
}

func (q *InMemoryJobQueue) Enqueue(ctx context.Context, files []*multipart.FileHeader, cleanup func()) (domain.Job, error) {
	job := domain.Job{
		JobID:  uuid.NewString(),
		Status: domain.JobStatusPending,
	}
	q.jobs.Store(job.JobID, job)

	select {
	case q.tasks <- JobTask{JobID: job.JobID, Files: files, Cleanup: cleanup}:
		return job, nil
	case <-ctx.Done():
		q.jobs.Delete(job.JobID)
		return domain.Job{}, ctx.Err()
	}
}

func (q *InMemoryJobQueue) Tasks() <-chan JobTask {
	return q.tasks
}

func (q *InMemoryJobQueue) Get(jobID string) (domain.Job, bool) {
	value, ok := q.jobs.Load(jobID)
	if !ok {
		return domain.Job{}, false
	}
	job, ok := value.(domain.Job)
	return job, ok
}

func (q *InMemoryJobQueue) Save(job domain.Job) {
	q.jobs.Store(job.JobID, job)
}
