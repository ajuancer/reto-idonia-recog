package http

import (
	"bytes"
	"net/http"
	"reto-idonia-recog-refactored/internal/idempotency"
)

type responseRecorder struct {
	header http.Header
	body   *bytes.Buffer
	status int
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{
		header: make(http.Header),
		body:   &bytes.Buffer{},
		status: http.StatusOK,
	}
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	return r.body.Write(data)
}

func (r *responseRecorder) cachedResponse() idempotency.CachedResponse {
	return idempotency.CachedResponse{
		Status: r.status,
		Header: r.header.Clone(),
		Body:   cloneBody(r.body.Bytes()),
	}
}

func cloneBody(body []byte) []byte {
	if len(body) == 0 {
		return nil
	}
	return append([]byte(nil), body...)
}

func (r *Router) withIdempotency(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if r.idempotencyStore == nil {
			r.logger.Error("Idempotency store not configured")
			http.Error(w, "Idempotency unavailable", http.StatusInternalServerError)
			return
		}

		key := req.Header.Get("Idempotency-Key")
		if key == "" {
			http.Error(w, "Idempotency-Key header required", http.StatusBadRequest)
			return
		}

		stored, found, err := r.idempotencyStore.GetResponse(req.Context(), key)
		if err != nil {
			r.logger.Error("Idempotency lookup failed", "error", err)
			http.Error(w, "Idempotency lookup failed", http.StatusInternalServerError)
			return
		}

		if found {
			if stored.Status == idempotency.StatusInProgress {
				http.Error(w, "Request already in progress", http.StatusConflict)
				return
			}
			if stored.Status == idempotency.StatusCompleted {
				writeCachedResponse(w, stored.Response)
				return
			}
		}

		locked, err := r.idempotencyStore.Lock(req.Context(), key)
		if err != nil {
			r.logger.Error("Failed to lock idempotency key", "error", err)
			http.Error(w, "Idempotency lock failed", http.StatusInternalServerError)
			return
		}
		if !locked {
			stored, found, err = r.idempotencyStore.GetResponse(req.Context(), key)
			if err != nil {
				r.logger.Error("Idempotency lookup failed", "error", err)
				http.Error(w, "Idempotency lookup failed", http.StatusInternalServerError)
				return
			}
			if found && stored.Status == idempotency.StatusCompleted {
				writeCachedResponse(w, stored.Response)
				return
			}
			http.Error(w, "Request already in progress", http.StatusConflict)
			return
		}

		completed := false
		defer func() {
			if !completed {
				_ = r.idempotencyStore.Unlock(req.Context(), key)
			}
		}()

		recorder := newResponseRecorder()
		next(recorder, req)

		cached := recorder.cachedResponse()
		if err := r.idempotencyStore.SaveResponse(req.Context(), key, cached); err != nil {
			r.logger.Error("Failed to store idempotency response", "error", err)
		} else {
			completed = true
		}

		writeCachedResponse(w, cached)
	}
}

func writeCachedResponse(w http.ResponseWriter, cached idempotency.CachedResponse) {
	for key, values := range cached.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(cached.Status)
	if len(cached.Body) > 0 {
		_, _ = w.Write(cached.Body)
	}
}
