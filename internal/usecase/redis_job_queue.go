package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"reto-idonia-recog-refactored/internal/domain"
)

const redisJobKeyPrefix = "job:"

// RedisJobQueue implements JobQueue using Redis for persistent job state
// storage. The tasks channel remains in-memory because multipart file
// headers cannot be serialised across process boundaries.
type RedisJobQueue struct {
	client *redis.Client
	tasks  chan JobTask
	ttl    time.Duration
}

// NewRedisJobQueue creates a RedisJobQueue with the given Redis client,
// channel buffer size, and job TTL.
func NewRedisJobQueue(client *redis.Client, buffer int, ttl time.Duration) *RedisJobQueue {
	if buffer < 1 {
		buffer = 1
	}
	return &RedisJobQueue{
		client: client,
		tasks:  make(chan JobTask, buffer),
		ttl:    ttl,
	}
}

// Enqueue creates a new pending job, persists it to Redis, and sends the
// associated task to the worker channel.
func (q *RedisJobQueue) Enqueue(ctx context.Context, files []*multipart.FileHeader, cleanup func()) (domain.Job, error) {
	job := domain.Job{
		JobID:  uuid.NewString(),
		Status: domain.JobStatusPending,
	}

	if err := q.saveToRedis(ctx, job); err != nil {
		return domain.Job{}, fmt.Errorf("redis job queue enqueue: %w", err)
	}

	select {
	case q.tasks <- JobTask{JobID: job.JobID, Files: files, Cleanup: cleanup}:
		return job, nil
	case <-ctx.Done():
		// Best-effort cleanup: remove the job we just stored.
		_ = q.client.Del(context.Background(), q.key(job.JobID)).Err()
		return domain.Job{}, ctx.Err()
	}
}

// Tasks returns the channel from which workers consume job tasks.
func (q *RedisJobQueue) Tasks() <-chan JobTask {
	return q.tasks
}

// Get retrieves a job by ID from Redis.
func (q *RedisJobQueue) Get(jobID string) (domain.Job, bool) {
	data, err := q.client.Get(context.Background(), q.key(jobID)).Bytes()
	if err == redis.Nil {
		return domain.Job{}, false
	}
	if err != nil {
		return domain.Job{}, false
	}

	var job domain.Job
	if err := json.Unmarshal(data, &job); err != nil {
		return domain.Job{}, false
	}
	return job, true
}

// Save persists an updated job back to Redis, refreshing its TTL.
func (q *RedisJobQueue) Save(job domain.Job) {
	_ = q.saveToRedis(context.Background(), job)
}

func (q *RedisJobQueue) saveToRedis(ctx context.Context, job domain.Job) error {
	data, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return q.client.Set(ctx, q.key(job.JobID), data, q.ttl).Err()
}

func (q *RedisJobQueue) key(jobID string) string {
	return redisJobKeyPrefix + jobID
}
