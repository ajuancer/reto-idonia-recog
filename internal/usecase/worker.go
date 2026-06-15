package usecase

import (
	"context"
	"log/slog"

	"reto-idonia-recog-refactored/internal/domain"
)

type WorkerPool struct {
	queue        JobQueue
	orchestrator *Orchestrator
	logger       *slog.Logger
	workers      int
}

func NewWorkerPool(queue JobQueue, orchestrator *Orchestrator, logger *slog.Logger, workers int) *WorkerPool {
	if workers < 1 {
		workers = 1
	}
	return &WorkerPool{
		queue:        queue,
		orchestrator: orchestrator,
		logger:       logger,
		workers:      workers,
	}
}

func (w *WorkerPool) Start(ctx context.Context) {
	for i := 0; i < w.workers; i++ {
		go w.run(ctx, i+1)
	}
}

func (w *WorkerPool) run(ctx context.Context, workerID int) {
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-w.queue.Tasks():
			if !ok {
				return
			}
			w.handleTask(ctx, task, workerID)
		}
	}
}

func (w *WorkerPool) handleTask(ctx context.Context, task JobTask, workerID int) {
	if task.Cleanup != nil {
		defer task.Cleanup()
	}

	job, ok := w.queue.Get(task.JobID)
	if !ok {
		w.logger.Warn("Job not found for processing", "job_id", task.JobID, "worker_id", workerID)
		return
	}

	job.Status = domain.JobStatusProcessing
	job.Error = ""
	job.Result = nil
	w.queue.Save(job)

	w.logger.Info("Job processing started", "job_id", task.JobID, "worker_id", workerID)

	result, err := w.orchestrator.ProcessFiles(ctx, task.Files)
	if err != nil {
		job.Status = domain.JobStatusFailed
		job.Error = err.Error()
		job.Result = nil
		w.queue.Save(job)
		w.logger.Error("Job processing failed", "job_id", task.JobID, "worker_id", workerID, "error", err)
		return
	}

	job.Status = domain.JobStatusCompleted
	job.Result = &domain.JobResult{
		ViewerURL: result.ViewerURL,
		PIN:       result.PIN,
	}
	job.Error = ""
	w.queue.Save(job)
	w.logger.Info("Job processing completed", "job_id", task.JobID, "worker_id", workerID)
}
