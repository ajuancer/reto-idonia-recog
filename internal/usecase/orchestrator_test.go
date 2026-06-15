package usecase

import (
	"context"
	"errors"
	"mime/multipart"
	"path/filepath"
	"testing"

	"reto-idonia-recog-refactored/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockIdoniaClient struct {
	mock.Mock
}

func (m *mockIdoniaClient) UploadReport(ctx context.Context, filePath string, meta domain.UploadMetadata) ([]string, error) {
	args := m.Called(ctx, filePath, meta)
	return mockStringSlice(args, 0), args.Error(1)
}

func (m *mockIdoniaClient) UploadReportBytes(ctx context.Context, filename string, data []byte, meta domain.UploadMetadata) ([]string, error) {
	args := m.Called(ctx, filename, data, meta)
	return mockStringSlice(args, 0), args.Error(1)
}

func (m *mockIdoniaClient) UploadDICOM(ctx context.Context, filePath string, meta domain.UploadMetadata) ([]string, error) {
	args := m.Called(ctx, filePath, meta)
	return mockStringSlice(args, 0), args.Error(1)
}

func (m *mockIdoniaClient) CreateMagicLink(ctx context.Context, patientID string) (domain.MagicLinkResult, error) {
	args := m.Called(ctx, patientID)
	result, _ := args.Get(0).(domain.MagicLinkResult)
	return result, args.Error(1)
}

func (m *mockIdoniaClient) GetViewerURL(baseURL string) string {
	args := m.Called(baseURL)
	return args.String(0)
}

type mockRecogClient struct {
	mock.Mock
}

func (m *mockRecogClient) GeneratePDF(ctx context.Context, dictationText string, meta domain.UploadMetadata) ([]byte, error) {
	args := m.Called(ctx, dictationText, meta)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

type depOverrides struct {
	stageFile            func(header *multipart.FileHeader) (domain.StagedUpload, error)
	extractDICOMMetadata func(filePath string) (domain.UploadMetadata, error)
	extractText          func(filePath string) (string, error)
	redactText           func(ctx context.Context, text string) (string, error)
	removeFile           func(path string) error
}

func stubUsecaseDeps(t *testing.T, overrides depOverrides) {
	t.Helper()
	originalStageFile := stageFile
	originalExtractDICOMMetadata := extractDICOMMetadata
	originalExtractText := extractText
	originalRedactText := redactText
	originalRemoveFile := removeFile

	stageFile = overrides.stageFile
	extractDICOMMetadata = overrides.extractDICOMMetadata
	extractText = overrides.extractText
	redactText = overrides.redactText
	removeFile = overrides.removeFile

	t.Cleanup(func() {
		stageFile = originalStageFile
		extractDICOMMetadata = originalExtractDICOMMetadata
		extractText = originalExtractText
		redactText = originalRedactText
		removeFile = originalRemoveFile
	})
}

func TestOrchestrator_ProcessFiles(t *testing.T) {
	baseMeta := domain.UploadMetadata{
		PatientID:         "DICOM-123",
		StudyDescription:  "Study",
		SeriesDescription: "Series",
		StudyID:           "Study-001",
	}
	generatedPDF := []byte("pdf-bytes")

	stubUsecaseDeps(t, depOverrides{
		stageFile: func(header *multipart.FileHeader) (domain.StagedUpload, error) {
			ext := filepath.Ext(header.Filename)
			if ext == "" {
				ext = ".dcm"
			}
			return domain.StagedUpload{
				Path:     filepath.Join("/fake", header.Filename),
				Ext:      ext,
				Filename: header.Filename,
			}, nil
		},
		extractDICOMMetadata: func(filePath string) (domain.UploadMetadata, error) {
			return baseMeta, nil
		},
		extractText: func(filePath string) (string, error) {
			return "raw dictation", nil
		},
		redactText: func(ctx context.Context, text string) (string, error) {
			return "redacted dictation", nil
		},
		removeFile: func(path string) error {
			return nil
		},
	})

	ctxCanceled, cancel := context.WithCancel(context.Background())

	type testInputs struct {
		ctx     context.Context
		headers []*multipart.FileHeader
	}

	tests := []struct {
		name           string
		inputs         testInputs
		mockSetup      func(t *testing.T, idonia *mockIdoniaClient, recog *mockRecogClient)
		expectedResult domain.UploadResult
		expectedError  string
		assertMocks    func(t *testing.T, idonia *mockIdoniaClient, recog *mockRecogClient)
	}{
		{
			name: "happy path with dicom and pdf",
			inputs: testInputs{
				ctx: context.Background(),
				headers: []*multipart.FileHeader{
					newFileHeader("scan1.dcm"),
					newFileHeader("report.pdf"),
				},
			},
			mockSetup: func(t *testing.T, idonia *mockIdoniaClient, recog *mockRecogClient) {
				t.Helper()
				recog.On("GeneratePDF", mock.Anything, "redacted dictation", baseMeta).Return(generatedPDF, nil).Once()
				idonia.On("UploadReportBytes", mock.Anything, "humanized_report.pdf", generatedPDF, baseMeta).Return([]string{"report-id"}, nil).Once()
				idonia.On("UploadDICOM", mock.Anything, filepath.Join("/fake", "scan1.dcm"), baseMeta).Return([]string{"dicom-id"}, nil).Once()
				idonia.On("UploadReport", mock.Anything, filepath.Join("/fake", "report.pdf"), baseMeta).Return([]string{"pdf-id"}, nil).Once()
				idonia.On("CreateMagicLink", mock.Anything, baseMeta.PatientID).Return(domain.MagicLinkResult{URL: "magic-url", PIN: "1234"}, nil).Once()
				idonia.On("GetViewerURL", "magic-url").Return("viewer-url").Once()
			},
			expectedResult: domain.UploadResult{ViewerURL: "viewer-url", PIN: "1234"},
		},
		{
			name: "idonia upload returns 500 error",
			inputs: testInputs{
				ctx: context.Background(),
				headers: []*multipart.FileHeader{
					newFileHeader("scan1.dcm"),
				},
			},
			mockSetup: func(t *testing.T, idonia *mockIdoniaClient, recog *mockRecogClient) {
				t.Helper()
				idonia.On("UploadDICOM", mock.Anything, filepath.Join("/fake", "scan1.dcm"), baseMeta).Return([]string(nil), errors.New("idonia 500")).Times(3)
			},
			expectedError: "file upload failed",
			assertMocks: func(t *testing.T, idonia *mockIdoniaClient, recog *mockRecogClient) {
				idonia.AssertNotCalled(t, "CreateMagicLink", mock.Anything, mock.Anything)
			},
		},
		{
			name: "no pdf found in batch",
			inputs: testInputs{
				ctx: context.Background(),
				headers: []*multipart.FileHeader{
					newFileHeader("scan1.dcm"),
				},
			},
			mockSetup: func(t *testing.T, idonia *mockIdoniaClient, recog *mockRecogClient) {
				t.Helper()
				idonia.On("UploadDICOM", mock.Anything, filepath.Join("/fake", "scan1.dcm"), baseMeta).Return([]string{"dicom-id"}, nil).Once()
				idonia.On("CreateMagicLink", mock.Anything, baseMeta.PatientID).Return(domain.MagicLinkResult{URL: "magic-url", PIN: "9876"}, nil).Once()
				idonia.On("GetViewerURL", "magic-url").Return("viewer-url").Once()
			},
			expectedResult: domain.UploadResult{ViewerURL: "viewer-url", PIN: "9876"},
			assertMocks: func(t *testing.T, idonia *mockIdoniaClient, recog *mockRecogClient) {
				recog.AssertNotCalled(t, "GeneratePDF", mock.Anything, mock.Anything, mock.Anything)
				idonia.AssertNotCalled(t, "UploadReportBytes", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			},
		},
		{
			name: "context cancelled during upload",
			inputs: testInputs{
				ctx: ctxCanceled,
				headers: []*multipart.FileHeader{
					newFileHeader("scan1.dcm"),
				},
			},
			mockSetup: func(t *testing.T, idonia *mockIdoniaClient, recog *mockRecogClient) {
				t.Helper()
				idonia.On("UploadDICOM", mock.Anything, filepath.Join("/fake", "scan1.dcm"), baseMeta).
					Run(func(args mock.Arguments) { cancel() }).
					Return([]string(nil), context.Canceled).
					Once()
			},
			expectedError: "context canceled",
			assertMocks: func(t *testing.T, idonia *mockIdoniaClient, recog *mockRecogClient) {
				idonia.AssertNotCalled(t, "CreateMagicLink", mock.Anything, mock.Anything)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			idonia := &mockIdoniaClient{}
			recog := &mockRecogClient{}

			tc.mockSetup(t, idonia, recog)

			orchestrator := NewOrchestrator(idonia, recog)
			result, err := orchestrator.ProcessFiles(tc.inputs.ctx, tc.inputs.headers)

			if tc.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedResult, result)
			}

			if tc.assertMocks != nil {
				tc.assertMocks(t, idonia, recog)
			}
			idonia.AssertExpectations(t)
			recog.AssertExpectations(t)
		})
	}
}

func newFileHeader(name string) *multipart.FileHeader {
	return &multipart.FileHeader{Filename: name}
}

func mockStringSlice(args mock.Arguments, index int) []string {
	if args.Get(index) == nil {
		return nil
	}
	return args.Get(index).([]string)
}
