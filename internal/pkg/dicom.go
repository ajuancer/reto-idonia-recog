package pkg

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"reto-idonia-recog-refactored/internal/domain"

	"github.com/suyashkumar/dicom"
	"github.com/suyashkumar/dicom/pkg/tag"
)

func ExtractDICOMMetadata(filePath string) (domain.UploadMetadata, error) {
	dataset, err := dicom.ParseFile(filePath, nil, dicom.SkipPixelData())
	if err != nil {
		return domain.UploadMetadata{}, fmt.Errorf("parse dicom: %w", err)
	}

	patientID, err := dicomStringValue(&dataset, tag.PatientID)
	if err != nil {
		return domain.UploadMetadata{}, fmt.Errorf("read patient id: %w", err)
	}

	studyDescription, err := dicomStringValue(&dataset, tag.StudyDescription)
	if err != nil {
		return domain.UploadMetadata{}, fmt.Errorf("read study description: %w", err)
	}

	seriesDescription, err := dicomStringValue(&dataset, tag.SeriesDescription)
	if err != nil {
		return domain.UploadMetadata{}, fmt.Errorf("read series description: %w", err)
	}

	studyId, err := dicomStringValue(&dataset, tag.StudyID)
	if err != nil {
		return domain.UploadMetadata{}, fmt.Errorf("read study ID: %w", err)
	}

	slog.Debug(
		"Extracted DICOM metadata",
		"patient_id_present", patientID != "",
		"study_description_present", studyDescription != "",
		"series_description_present", seriesDescription != "",
		"study_id_present", studyId != "",
	)
	return domain.UploadMetadata{
		PatientID:         patientID,
		StudyDescription:  studyDescription,
		SeriesDescription: seriesDescription,
		StudyID:           studyId,
	}, nil
}

func dicomStringValue(dataset *dicom.Dataset, tagID tag.Tag) (string, error) {
	element, err := dataset.FindElementByTag(tagID)
	if err != nil {
		return "", err
	}
	if element.Value == nil {
		return "", errors.New("dicom element has no value")
	}
	if element.Value.ValueType() != dicom.Strings {
		return "", fmt.Errorf("unexpected value type %v for %v", element.Value.ValueType(), tagID)
	}
	values := dicom.MustGetStrings(element.Value)
	if len(values) == 0 {
		return "", errors.New("dicom element has empty value")
	}
	value := strings.TrimSpace(values[0])
	if value == "" {
		return "", errors.New("dicom element has blank value")
	}
	return value, nil
}
