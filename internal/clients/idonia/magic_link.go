package idonia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"reto-idonia-recog-refactored/internal/domain"
)

// We define this privately since the domain uses domain.MagicLinkResult
type magicLinkResponseDTO struct {
	URL string `json:"url"`
	PIN string `json:"pin"`
}

func (c *Client) CreateMagicLink(ctx context.Context, patientID string) (domain.MagicLinkResult, error) {
	token, err := c.generateJWT()
	if err != nil {
		return domain.MagicLinkResult{}, fmt.Errorf("jwt generation failed: %w", err)
	}

	requestURL, err := c.buildMagicLinkURL(patientID)
	if err != nil {
		return domain.MagicLinkResult{}, err
	}

	respBodyBytes, err := c.executeMagicLinkRequest(ctx, requestURL, token)
	if err != nil {
		return domain.MagicLinkResult{}, err
	}

	response, err := parseMagicLinkResponse(respBodyBytes)
	if err != nil {
		return domain.MagicLinkResult{}, err
	}

	slog.Info("Magic link successfully created",
		"event_type", "MAGIC_LINK_CREATED",
		"patient_id", patientID,
		"audit", true,
	)

	c.magicLinkCounter.Add(context.Background(), 1)

	return domain.MagicLinkResult{
		URL: response.URL,
		PIN: response.PIN,
	}, nil
}

func (c *Client) buildMagicLinkURL(patientID string) (string, error) {
	baseURL, err := url.Parse(c.config.IdoniaAPIUrl + "/ml")
	if err != nil {
		return "", fmt.Errorf("invalid API URL: %w", err)
	}

	q := baseURL.Query()
	q.Add("route", patientID)
	q.Add("expired_creation_mode", "update")
	baseURL.RawQuery = q.Encode()

	return baseURL.String(), nil
}

func (c *Client) executeMagicLinkRequest(ctx context.Context, requestURL string, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "PUT", requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("magic link request failed: %w", err)
	}
	defer resp.Body.Close()

	respBodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBodyBytes))
	}

	return respBodyBytes, nil
}

func parseMagicLinkResponse(respBodyBytes []byte) (*magicLinkResponseDTO, error) {
	var mlResponses []magicLinkResponseDTO
	if err := json.Unmarshal(respBodyBytes, &mlResponses); err != nil {
		return nil, fmt.Errorf("failed to decode magic link response: %w", err)
	}

	if len(mlResponses) == 0 {
		return nil, errors.New("empty magic link response array")
	}

	return &mlResponses[0], nil
}

func (c *Client) GetViewerURL(ref string) string {
	return fmt.Sprintf("https://demo.idonia.com/v/%s", ref)
}
