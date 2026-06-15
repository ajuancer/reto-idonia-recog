package idonia

import (
	"net/http"
	"reto-idonia-recog-refactored/internal/config"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type Client struct {
	config                *config.Config
	httpClient            *http.Client
	magicLinkCounter      metric.Int64Counter
	dicomsUploadedCounter metric.Int64Counter
}

func NewClient(cfg *config.Config) *Client {
	meter := otel.Meter("reto-idonia-recog/clients/idonia")

	mlCounter, _ := meter.Int64Counter(
		"magic_links_created_total",
		metric.WithDescription("Total number of magic links created via the API"),
	)

	dicomsCounter, _ := meter.Int64Counter(
		"dicoms_uploaded_total",
		metric.WithDescription("Total number of DICOMs uploaded via the API"),
	)

	return &Client{
		config:                cfg,
		httpClient:            &http.Client{Timeout: 30 * time.Second},
		magicLinkCounter:      mlCounter,
		dicomsUploadedCounter: dicomsCounter,
	}
}
