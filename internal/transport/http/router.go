package http

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"reto-idonia-recog-refactored/internal/config"
	"reto-idonia-recog-refactored/internal/idempotency"
	"reto-idonia-recog-refactored/internal/logging"
	"reto-idonia-recog-refactored/internal/usecase"

	"github.com/gorilla/csrf"
)

// Router acts as the container for our HTTP dependencies.
type Router struct {
	orchestrator     *usecase.Orchestrator
	logger           *slog.Logger
	tmpl             *template.Template
	idempotencyStore idempotency.IdempotencyStore
	jobQueue         usecase.JobQueue
}

// NewRouter wires up the HTTP routes, middleware, and templates.
func NewRouter(cfg *config.Config, logger *slog.Logger, orchestrator *usecase.Orchestrator, jobQueue usecase.JobQueue, metricsHandler http.Handler, idempotencyStore idempotency.IdempotencyStore) http.Handler {
	r := &Router{
		orchestrator:     orchestrator,
		logger:           logger,
		tmpl:             template.Must(template.ParseFiles("web/templates/index.html")),
		idempotencyStore: idempotencyStore,
		jobQueue:         jobQueue,
	}

	csrfKey := []byte(cfg.CSRFKey)
	if len(csrfKey) != 32 {
		logger.Error("CSRF key must be exactly 32 bytes", "current_length", len(csrfKey))
		panic("invalid CSRF key length")
	}

	csrfMiddleware := csrf.Protect(
		csrfKey,
		csrf.Secure(cfg.Env == "production"),
		csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			err := csrf.FailureReason(req)
			logger.Warn("CSRF validation failed", "error", err)
			http.Error(w, fmt.Sprintf("CSRF Validation Failed: %v", err), http.StatusForbidden)
		})),
		csrf.TrustedOrigins(cfg.Hosts),
	)

	// rootMux handles unprotected routes
	rootMux := http.NewServeMux()
	// appMux handles routes that require CSRF protection
	appMux := http.NewServeMux()

	appMux.HandleFunc("GET /", r.handleIndex)
	appMux.HandleFunc("POST /api/v1/upload", r.withIdempotency(r.handleUpload))
	appMux.HandleFunc("GET /api/v1/upload/status/{job_id}", r.handleUploadStatus)

	// Static server
	fs := http.FileServer(http.Dir("web/static"))
	rootMux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	// Health check endpoint
	rootMux.HandleFunc("GET /health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Prometheus metrics
	if metricsHandler != nil {
		rootMux.Handle("GET /metrics", metricsHandler)
	}

	// Mount the CSRF-protected appMux onto the rootMux at the root
	rootMux.Handle("/", csrfMiddleware(appMux))

	return logging.HTTPMiddleware(rootMux)
}

// handleIndex serves the main HTML page and injects the CSRF token
func (r *Router) handleIndex(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := map[string]interface{}{
		"CSRFToken": csrf.Token(req),
	}

	if err := r.tmpl.ExecuteTemplate(w, "main", data); err != nil {
		r.logger.Error("Template render failed", "error", err)
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}
