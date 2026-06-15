package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"reto-idonia-recog-refactored/internal/domain"
)

func (o *Orchestrator) detectBaseMetadata(uploads []domain.StagedUpload, fallback domain.UploadMetadata) (domain.UploadMetadata, bool) {
	for _, upload := range uploads {
		if upload.Ext != ".pdf" {
			extractedMeta, err := extractDICOMMetadata(upload.Path)
			if err == nil {
				return extractedMeta, true
			}
			slog.Warn("Failed to extract metadata from DICOM", "file", upload.Filename, "error", err)
		}
	}
	return fallback, false
}

func (o *Orchestrator) generateHumanizedReport(ctx context.Context, uploads []domain.StagedUpload, meta domain.UploadMetadata) error {
	pdfUpload := findFirstPDF(uploads)
	if pdfUpload == nil {
		return nil
	}

	dictationText, err := extractText(pdfUpload.Path)
	if err != nil {
		return fmt.Errorf("extract pdf content: %w", err)
	}

	redactedDictation, err := redactText(ctx, dictationText)
	if err != nil {
		return fmt.Errorf("redact dictation text: %w", err)
	}

	var pdfBytes []byte
	if err := retryWithBackoff(ctx, func() error {
		var callErr error
		pdfBytes, callErr = o.recog.GeneratePDF(ctx, redactedDictation, meta)
		return callErr
	}); err != nil {
		return fmt.Errorf("generation step failed: %w", err)
	}

	if err := retryWithBackoff(ctx, func() error {
		_, callErr := o.idonia.UploadReportBytes(ctx, "humanized_report.pdf", pdfBytes, meta)
		return callErr
	}); err != nil {
		return fmt.Errorf("upload step failed: %w", err)
	}

	return nil
}

func findFirstPDF(uploads []domain.StagedUpload) *domain.StagedUpload {
	for i := range uploads {
		if uploads[i].Ext == ".pdf" {
			return &uploads[i]
		}
	}
	return nil
}
