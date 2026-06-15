package idonia

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"reto-idonia-recog-refactored/internal/domain"
)

// UploadReport handles file path based report uploads.
func (c *Client) UploadReport(ctx context.Context, filePath string, meta domain.UploadMetadata) ([]string, error) {
	file, err := openFileSafely(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	endpoint := fmt.Sprintf("%s/files/report_hak_%s", c.config.IdoniaAPIUrl, c.config.HackathonRef)
	slog.InfoContext(ctx, "Uploading report to Idonia", "endpoint", endpoint)

	return c.doUpload(ctx, endpoint, filepath.Base(filePath), file, meta)
}

// UploadReportBytes handles byte slice based report uploads.
func (c *Client) UploadReportBytes(ctx context.Context, filename string, data []byte, meta domain.UploadMetadata) ([]string, error) {
	endpoint := fmt.Sprintf("%s/files/report_hak_%s", c.config.IdoniaAPIUrl, c.config.HackathonRef)
	slog.InfoContext(ctx, "Uploading report bytes to Idonia", "endpoint", endpoint)

	return c.doUpload(ctx, endpoint, filename, bytes.NewReader(data), meta)
}

// UploadDICOM handles file path based DICOM uploads.
func (c *Client) UploadDICOM(ctx context.Context, filePath string, meta domain.UploadMetadata) ([]string, error) {
	file, err := openFileSafely(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	endpoint := fmt.Sprintf("%s/files/dicom_hak_%s", c.config.IdoniaAPIUrl, c.config.HackathonRef)
	slog.InfoContext(ctx, "Uploading DICOM to Idonia", "endpoint", endpoint)

	uuids, err := c.doUpload(ctx, endpoint, filepath.Base(filePath), file, meta)
	if err == nil {
		// Only increment metrics on success, safely tracking true DICOM uploads
		slog.InfoContext(ctx, "Successfully uploaded DICOM", "event_type", "UPLOAD_HUMANIZED_REPORT", "audit", true)
		c.dicomsUploadedCounter.Add(ctx, 1)
	}
	return uuids, err
}

// doUpload is the internal unified upload method. It accepts an io.Reader
// to seamlessly handle both *os.File streams and *bytes.Reader buffers.
func (c *Client) doUpload(ctx context.Context, endpoint, filename string, reader io.Reader, meta domain.UploadMetadata) ([]string, error) {
	token, err := c.generateJWT()
	if err != nil {
		return nil, fmt.Errorf("jwt generation failed: %w", err)
	}

	req, err := buildStreamingUploadRequest(ctx, endpoint, filename, reader, meta, token)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	return readUploadResponse(resp)
}

// buildStreamingUploadRequest uses io.Pipe to stream the multipart payload directly
// to the HTTP request without buffering the entire file into memory. This is critical
// for concurrent uploads of large DICOM files to prevent memory exhaustion (OOM).
func buildStreamingUploadRequest(ctx context.Context, endpoint, filename string, reader io.Reader, meta domain.UploadMetadata, token string) (*http.Request, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Run the multipart writing in a goroutine so the HTTP client can read from the pipe concurrently
	go func() {
		var err error
		defer func() {
			closeErr := writer.Close()
			if err == nil {
				err = closeErr
			}
			pw.CloseWithError(err)
		}()

		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			return
		}

		// Stream the data from the reader to the pipe
		if _, err = io.Copy(part, reader); err != nil {
			return
		}

		// Write metadata fields
		if meta.PatientID != "" {
			_ = writer.WriteField("DICOMPatientID", meta.PatientID)
		}
		if meta.StudyID != "" {
			_ = writer.WriteField("DICOMAccessionNumber", meta.StudyID)
		}
		if meta.StudyDescription != "" {
			_ = writer.WriteField("DICOMStudyDescription", meta.StudyDescription)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, pr)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	return req, nil
}

// openFileSafely is a helper to isolate file opening and path sanitization logic.
func openFileSafely(filePath string) (*os.File, error) {
	cleanPath := filepath.Clean(filePath)
	rootDir := filepath.Dir(cleanPath)
	baseName := filepath.Base(cleanPath)

	if baseName == "." || baseName == ".." || baseName == string(filepath.Separator) {
		return nil, fmt.Errorf("invalid file path: %s", filePath)
	}

	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return nil, fmt.Errorf("could not open file root: %w", err)
	}

	return root.Open(baseName)
}

// readUploadResponse parses the successful response UUIDs or extracts the error message.
func readUploadResponse(resp *http.Response) ([]string, error) {
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}

	var fileUUIDs []string
	if err := json.NewDecoder(resp.Body).Decode(&fileUUIDs); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	slog.Debug("Idonia upload completed", "file_count", len(fileUUIDs))
	return fileUUIDs, nil
}
