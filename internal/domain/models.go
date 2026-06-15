package domain

type UploadResponse struct {
	Message string `json:"message"`
	URL     string `json:"url"`
	PIN     string `json:"pin"`
}

type StagedUpload struct {
	Path     string
	Ext      string
	Filename string
}

type UploadResult struct {
	ViewerURL string
	PIN       string
}

type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
)

type JobResult struct {
	ViewerURL string `json:"viewer_url,omitempty"`
	PIN       string `json:"pin,omitempty"`
}

type Job struct {
	JobID  string     `json:"job_id"`
	Status JobStatus  `json:"status"`
	Result *JobResult `json:"result,omitempty"`
	Error  string     `json:"error,omitempty"`
}

type MagicLinkResult struct {
	URL string
	PIN string
}

type UploadMetadata struct {
	PatientID         string
	StudyDescription  string
	SeriesDescription string
	StudyID           string
}
