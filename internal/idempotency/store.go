package idempotency

import (
	"context"
	"errors"
	"net/http"
	"sync"
)

type Status int

const (
	StatusInProgress Status = iota + 1
	StatusCompleted
)

type CachedResponse struct {
	Status int
	Header http.Header
	Body   []byte
}

type StoredResponse struct {
	Status   Status
	Response CachedResponse
}

type IdempotencyStore interface {
	Lock(ctx context.Context, key string) (bool, error)
	Unlock(ctx context.Context, key string) error
	SaveResponse(ctx context.Context, key string, response CachedResponse) error
	GetResponse(ctx context.Context, key string) (StoredResponse, bool, error)
}

type MemoryStore struct {
	entries sync.Map
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (m *MemoryStore) Lock(_ context.Context, key string) (bool, error) {
	if key == "" {
		return false, errors.New("idempotency key required")
	}

	_, loaded := m.entries.LoadOrStore(key, StoredResponse{Status: StatusInProgress})
	return !loaded, nil
}

func (m *MemoryStore) Unlock(_ context.Context, key string) error {
	if key == "" {
		return errors.New("idempotency key required")
	}

	value, ok := m.entries.Load(key)
	if !ok {
		return nil
	}

	stored, ok := value.(StoredResponse)
	if !ok {
		m.entries.Delete(key)
		return nil
	}

	if stored.Status == StatusInProgress {
		m.entries.Delete(key)
	}
	return nil
}

func (m *MemoryStore) SaveResponse(_ context.Context, key string, response CachedResponse) error {
	if key == "" {
		return errors.New("idempotency key required")
	}

	m.entries.Store(key, StoredResponse{
		Status: StatusCompleted,
		Response: CachedResponse{
			Status: response.Status,
			Header: cloneHeader(response.Header),
			Body:   cloneBody(response.Body),
		},
	})
	return nil
}

func (m *MemoryStore) GetResponse(_ context.Context, key string) (StoredResponse, bool, error) {
	if key == "" {
		return StoredResponse{}, false, errors.New("idempotency key required")
	}

	value, ok := m.entries.Load(key)
	if !ok {
		return StoredResponse{}, false, nil
	}

	stored, ok := value.(StoredResponse)
	if !ok {
		return StoredResponse{}, false, errors.New("invalid idempotency entry")
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

func cloneHeader(header http.Header) http.Header {
	if header == nil {
		return make(http.Header)
	}
	return header.Clone()
}

func cloneBody(body []byte) []byte {
	if len(body) == 0 {
		return nil
	}
	return append([]byte(nil), body...)
}
