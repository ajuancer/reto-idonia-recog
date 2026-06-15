package recog

import (
	"net/http"
	"reto-idonia-recog-refactored/internal/config"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type Client struct {
	config                 *config.Config
	httpClient             *http.Client
	humanizedReportCounter metric.Int64Counter
}

func NewClient(cfg *config.Config) *Client {
	meter := otel.Meter("reto-idonia-recog/clients/recog")

	hrCounter, _ := meter.Int64Counter(
		"humanized_report_generated_total",
		metric.WithDescription("Total number of humanized reports generated via the API"),
	)

	return &Client{
		config:                 cfg,
		httpClient:             &http.Client{Timeout: 60 * time.Second},
		humanizedReportCounter: hrCounter,
	}
}
