package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisIdempotencyKeyPrefix = "idempotency:"

type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisStore(client *redis.Client, ttl time.Duration) *RedisStore {
	return &RedisStore{
		client: client,
		ttl:    ttl,
	}
}

func (r *RedisStore) Lock(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, errors.New("idempotency key required")
	}

	data, err := json.Marshal(StoredResponse{Status: StatusInProgress})
	if err != nil {
		return false, err
	}

	locked, err := r.client.SetNX(ctx, r.entryKey(key), data, r.ttl).Result()
	if err != nil {
		return false, err
	}

	return locked, nil
}

func (r *RedisStore) Unlock(ctx context.Context, key string) error {
	if key == "" {
		return errors.New("idempotency key required")
	}

	stored, found, err := r.GetResponse(ctx, key)
	if err != nil || !found {
		return err
	}

	if stored.Status == StatusInProgress {
		return r.client.Del(ctx, r.entryKey(key)).Err()
	}

	return nil
}

func (r *RedisStore) SaveResponse(ctx context.Context, key string, response CachedResponse) error {
	if key == "" {
		return errors.New("idempotency key required")
	}

	stored := StoredResponse{
		Status: StatusCompleted,
		Response: CachedResponse{
			Status: response.Status,
			Header: cloneHeader(response.Header),
			Body:   cloneBody(response.Body),
		},
	}

	data, err := json.Marshal(stored)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, r.entryKey(key), data, r.ttl).Err()
}

func (r *RedisStore) GetResponse(ctx context.Context, key string) (StoredResponse, bool, error) {
	if key == "" {
		return StoredResponse{}, false, errors.New("idempotency key required")
	}

	data, err := r.client.Get(ctx, r.entryKey(key)).Bytes()
	if err == redis.Nil {
		return StoredResponse{}, false, nil
	}
	if err != nil {
		return StoredResponse{}, false, err
	}

	var stored StoredResponse
	if err := json.Unmarshal(data, &stored); err != nil {
		return StoredResponse{}, false, err
	}

	return StoredResponse{
		Status: stored.Status,
		Response: CachedResponse{
			Status: stored.Response.Status,
			Header: cloneHeader(stored.Response.Header),
			Body:   cloneBody(stored.Response.Body),
		},
	}, true, nil
}

func (r *RedisStore) entryKey(key string) string {
	return fmt.Sprintf("%s%s", redisIdempotencyKeyPrefix, key)
}
