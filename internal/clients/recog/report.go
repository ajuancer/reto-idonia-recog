package recog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reto-idonia-recog-refactored/internal/domain"
	"strings"
)

const defaultInitialContext = "Un montañero asturiano vecino de Panes sufre una lesión de rodilla en una mala caída en la zona de los Picos de Europa. " +
	"Tras el accidente, lo bajan al consultorio de Panes y tras la inmovilización y analgesia, " +
	"se decide derivar al Hospital de Sierrallana para cirugía. " +
	"Le realizan una resonancia magnética antes de la operación, " +
	"se ejecuta el procedimiento quirúrgico y le efectúan una última " +
	"prueba de imagen después para confirmar el buen resultado de la misma."

// DTOs specifically for the Recog API
type RelistenMetadata struct {
	DictationReport string       `json:"dictation_report"`
	Consultation    Consultation `json:"consultation"`
}

type Consultation struct {
	Lang             string `json:"lang"`
	LangCult         string `json:"lang_cult"`
	UserRole         string `json:"user_role"`
	Country          string `json:"country"`
	HealthCenter     string `json:"health_center"`
	InitialContext   string `json:"initial_context"`
	IsMulti          bool   `json:"is_multi"`
	Speciality       string `json:"speciality,omitempty"`
	SubSpeciality    string `json:"sub_speciality,omitempty"`
	ConsultationType string `json:"consultation_type,omitempty"`
}

// GeneratePDF calls the Recog API and returns the generated PDF bytes.
func (c *Client) GeneratePDF(ctx context.Context, dictationText string, meta domain.UploadMetadata) ([]byte, error) {
	endpoint := fmt.Sprintf("%s/relisten/dictation/process/report-results", c.config.RecogAPIUrl)

	payload := RelistenMetadata{
		DictationReport: dictationText,
		Consultation:    buildConsultation(meta),
	}

	slog.Debug("full payload", "payload", payload)

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal recog payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create recog request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/pdf")
	if c.config.RecogAPIKey != "" {
		req.Header.Set("X-API-Key", c.config.RecogAPIKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("recog request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		slog.Warn("Recog request returned non-success status", "status", resp.StatusCode)
		return nil, fmt.Errorf("recog api returned unexpected status %d: %s", resp.StatusCode, string(body))
	}

	pdfBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	slog.Info("Humanized report generated", "event_type", "HUMANIZED_REPORT_GENERATED", "audit", true)
	c.humanizedReportCounter.Add(ctx, 1)

	return pdfBytes, nil
}

func buildConsultation(dicomMeta domain.UploadMetadata) Consultation {
	consultation := Consultation{
		Lang:           "es",
		LangCult:       "es-ES",
		UserRole:       "patient",
		Country:        "spain",
		HealthCenter:   "Hospital de Sierrallana",
		InitialContext: defaultInitialContext,
		IsMulti:        false,
	}

	// Assuming you add these fields to domain.UploadMetadata
	if value := strings.TrimSpace(dicomMeta.StudyDescription); value != "" {
		consultation.Speciality = value
	}
	if value := strings.TrimSpace(dicomMeta.SeriesDescription); value != "" {
		consultation.SubSpeciality = value
	}
	if value := strings.TrimSpace(dicomMeta.StudyID); value != "" {
		consultation.ConsultationType = value
	}

	return consultation
}
