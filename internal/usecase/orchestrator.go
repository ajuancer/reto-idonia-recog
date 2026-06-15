package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"reto-idonia-recog-refactored/internal/domain"
)

type IdoniaClient interface {
	UploadReport(ctx context.Context, filePath string, meta domain.UploadMetadata) ([]string, error)
	UploadReportBytes(ctx context.Context, filename string, data []byte, meta domain.UploadMetadata) ([]string, error)
	UploadDICOM(ctx context.Context, filePath string, meta domain.UploadMetadata) ([]string, error)
	CreateMagicLink(ctx context.Context, patientID string) (domain.MagicLinkResult, error)
	GetViewerURL(baseURL string) string
}

type RecogClient interface {
	GeneratePDF(ctx context.Context, dictationText string, meta domain.UploadMetadata) ([]byte, error)
}

type Orchestrator struct {
	idonia IdoniaClient
	recog  RecogClient
}

func NewOrchestrator(idonia IdoniaClient, recog RecogClient) *Orchestrator {
	return &Orchestrator{
		idonia: idonia,
		recog:  recog,
	}
}

// ProcessFiles is the high-level workflow.
func (o *Orchestrator) ProcessFiles(ctx context.Context, fileHeaders []*multipart.FileHeader) (domain.UploadResult, error) {
	stagedFiles, err := o.stageUploads(fileHeaders)
	if err != nil {
		return domain.UploadResult{}, fmt.Errorf("staging failed: %w", err)
	}
	defer o.cleanupStagedUploads(stagedFiles)

	meta := domain.UploadMetadata{
		PatientID: fmt.Sprintf("PAT-%d", time.Now().Unix()),
	}

	meta, dicomFound := o.detectBaseMetadata(stagedFiles, meta)
	if !dicomFound {
		slog.Info("No DICOM metadata found in batch; using default metadata")
	}

	if err := o.generateHumanizedReport(ctx, stagedFiles, meta); err != nil {
		return domain.UploadResult{}, fmt.Errorf("report generation failed: %w", err)
	}

	if err := o.uploadStagedFiles(ctx, stagedFiles, meta); err != nil {
		return domain.UploadResult{}, fmt.Errorf("file upload failed: %w", err)
	}

	var res domain.MagicLinkResult
	if err := retryWithBackoff(ctx, func() error {
		var callErr error
		res, callErr = o.idonia.CreateMagicLink(ctx, meta.PatientID)
		return callErr
	}); err != nil {
		return domain.UploadResult{}, fmt.Errorf("magic link creation failed: %w", err)
	}

	return domain.UploadResult{
		ViewerURL: o.idonia.GetViewerURL(res.URL),
		PIN:       res.PIN,
	}, nil
}

// ProcessDirectory is the high-level workflow for local files (CLI use-case).
func (o *Orchestrator) ProcessDirectory(ctx context.Context, dirPath string) (domain.UploadResult, error) {
	stagedFiles, err := o.stageFromDirectory(dirPath)
	if err != nil {
		return domain.UploadResult{}, fmt.Errorf("directory processing failed: %w", err)
	}


	meta := domain.UploadMetadata{
		PatientID: fmt.Sprintf("PAT-%d", time.Now().Unix()),
	}

	meta, dicomFound := o.detectBaseMetadata(stagedFiles, meta)
	if !dicomFound {
		slog.Info("No DICOM metadata found in batch; using default metadata")
	}

	if err := o.generateHumanizedReport(ctx, stagedFiles, meta); err != nil {
		return domain.UploadResult{}, fmt.Errorf("report generation failed: %w", err)
	}

	if err := o.uploadStagedFiles(ctx, stagedFiles, meta); err != nil {
		return domain.UploadResult{}, fmt.Errorf("file upload failed: %w", err)
	}

	var res domain.MagicLinkResult
	if err := retryWithBackoff(ctx, func() error {
		var callErr error
		res, callErr = o.idonia.CreateMagicLink(ctx, meta.PatientID)
		return callErr
	}); err != nil {
		return domain.UploadResult{}, fmt.Errorf("magic link creation failed: %w", err)
	}

	return domain.UploadResult{
		ViewerURL: o.idonia.GetViewerURL(res.URL),
		PIN:       res.PIN,
	}, nil
}

// stageFromDirectory walks the local directory and maps files to the internal StagedUpload structure.
func (o *Orchestrator) stageFromDirectory(dirPath string) ([]domain.StagedUpload, error) {
	var files []domain.StagedUpload

	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err // Stop walking if we hit a permissions or path error
		}

		// Skip directories
		if !d.IsDir() {
			files = append(files, domain.StagedUpload{
				Path:     path,
				Ext:      filepath.Ext(d.Name()),
				Filename: d.Name(),
			})
			slog.Debug("Staged local file", "filename", d.Name(), "path", path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to read directory '%s': %w", dirPath, err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files found in directory: %s", dirPath)
	}

	return files, nil
}

// workerLimit caps goroutines dynamically based on CPU cores or array size.
func workerLimit(count int) int {
	limit := runtime.GOMAXPROCS(0)
	if limit > count {
		limit = count
	}
	if limit < 1 {
		return 1
	}
	return limit
}
