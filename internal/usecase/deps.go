package usecase

import (
	"os"

	"reto-idonia-recog-refactored/internal/pkg"
	"reto-idonia-recog-refactored/internal/pkg/media"
)

var (
	stageFile            = media.StageFile
	extractDICOMMetadata = pkg.ExtractDICOMMetadata
	extractText          = pkg.ExtractText
	redactText           = pkg.Redact
	removeFile           = os.Remove
)
