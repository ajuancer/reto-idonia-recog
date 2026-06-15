package media

import (
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"reto-idonia-recog-refactored/internal/pkg"

	"reto-idonia-recog-refactored/internal/domain"
)

// StageFile streams a multipart file directly to the local disk's temp folder.
func StageFile(header *multipart.FileHeader) (domain.StagedUpload, error) {
	file, err := header.Open()
	if err != nil {
		slog.Warn("Error opening file", "filename", header.Filename, "error", err)
		return domain.StagedUpload{}, err
	}
	defer file.Close()

	// 1. Detect the true file type via Magic Bytes
	fileType, err := pkg.DetectFileType(file)
	if err != nil || fileType == pkg.TypeUnknown {
		return domain.StagedUpload{}, fmt.Errorf("invalid file format for %s: must be a real PDF or DICOM", header.Filename)
	}

	// 2. REWIND THE FILE POINTER!
	// We read the first 132 bytes during detection, so we must reset back to the start.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return domain.StagedUpload{}, fmt.Errorf("failed to reset file pointer: %w", err)
	}

	// 3. Force the extension based on the true type, ignoring whatever the user named it
	ext := ".dcm"
	if fileType == pkg.TypePDF {
		ext = ".pdf"
	}

	// Create temp file and copy the safely verified content
	tempFile, err := os.CreateTemp("", fmt.Sprintf("upload-*%s", ext))
	if err != nil {
		return domain.StagedUpload{}, err
	}

	if _, err := io.Copy(tempFile, file); err != nil {
		slog.Warn("Error copying file", "filename", header.Filename, "error", err)
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name()) // Clean up on failure
		return domain.StagedUpload{}, err
	}

	tempFilePath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempFilePath)
		return domain.StagedUpload{}, err
	}

	return domain.StagedUpload{
		Path:     tempFilePath,
		Ext:      ext,
		Filename: header.Filename,
	}, nil
}
