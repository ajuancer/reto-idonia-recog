package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
)

const maxUploadSize = 100 << 20 // 100 MB
const maxMemory = 20 << 20      // 20 MB

// UploadResponse represents the JSON payload sent back to the client.
type UploadResponse struct {
	Message string `json:"message"`
	JobID   string `json:"job_id"`
}

// handleUpload is the HTTP boundary.
func (r *Router) handleUpload(w http.ResponseWriter, req *http.Request) {
	// Parse request
	files, err := parseUploadFiles(w, req)
	if err != nil {
		r.logger.Warn("Failed to parse upload files", "error", err)
		http.Error(w, "Unable to parse form data", http.StatusBadRequest)
		return
	}

	cleanup := func() {
		if req.MultipartForm != nil {
			if err := req.MultipartForm.RemoveAll(); err != nil {
				r.logger.Warn("Failed to remove multipart form files", "error", err)
			}
		}
	}

	job, err := r.orchestrator.EnqueueJob(req.Context(), r.jobQueue, files, cleanup)
	if err != nil {
		r.logger.Error("Failed to enqueue upload job", "error", err)
		http.Error(w, "Failed to enqueue upload job", http.StatusInternalServerError)
		return
	}

	// Format and send HTTP response
	response := UploadResponse{
		Message: "Upload accepted",
		JobID:   job.JobID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		r.logger.Error("Failed to write upload response", "error", err)
	}
}

func (r *Router) handleUploadStatus(w http.ResponseWriter, req *http.Request) {
	if r.jobQueue == nil {
		http.Error(w, "Job queue unavailable", http.StatusInternalServerError)
		return
	}

	jobID := req.PathValue("job_id")
	if jobID == "" {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}

	job, ok := r.jobQueue.Get(jobID)
	if !ok {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(job); err != nil {
		r.logger.Error("Failed to write job status response", "error", err)
	}
}

// parseUploadFiles handles HTTP multipart parsing concerns
func parseUploadFiles(w http.ResponseWriter, req *http.Request) ([]*multipart.FileHeader, error) {
	req.Body = http.MaxBytesReader(w, req.Body, maxUploadSize)

	// #nosec G120 -- false positive: r.Body is safely bounded by http.MaxBytesReader above
	if err := req.ParseMultipartForm(maxMemory); err != nil {
		return nil, fmt.Errorf("unable to parse form data: %w", err)
	}

	files := req.MultipartForm.File["files"]
	if len(files) == 0 {
		return nil, errors.New("no files provided")
	}

	return files, nil
}
