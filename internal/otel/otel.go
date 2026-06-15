package otel

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
)

func Setup(ctx context.Context) (func(context.Context) error, http.Handler, error) {
	var shutdownFuncs []func(context.Context) error
	var err error
	var metricsHandler http.Handler

	shutdown := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// handleErr calls shutdown for cleanup and makes sure that all errors are returned.
	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	res, err := newResource(ctx)
	if err != nil {
		handleErr(err)
		return shutdown, nil, err
	}

	// Set up propagator.
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	// Set up trace provider.
	tracerProvider, err := newTracerProvider(res)
	if err != nil {
		handleErr(err)
		return shutdown, nil, err
	}
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	// Set up meter provider.
	meterProvider, handler, err := newMeterProvider(res)
	if err != nil {
		handleErr(err)
		return shutdown, nil, err
	}
	metricsHandler = handler
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	// Set up logger provider.
	loggerProvider, err := newLoggerProvider(res)
	if err != nil {
		handleErr(err)
		return shutdown, metricsHandler, err
	}
	shutdownFuncs = append(shutdownFuncs, loggerProvider.Shutdown)
	global.SetLoggerProvider(loggerProvider)

	return shutdown, metricsHandler, err
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func newTracerProvider(resource *sdkresource.Resource) (*sdktrace.TracerProvider, error) {
	traceExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource),
		sdktrace.WithBatcher(traceExporter,
			// Default is 5s. Set to 1s for demonstrative purposes.
			sdktrace.WithBatchTimeout(time.Second)),
	)
	return tracerProvider, nil
}

func newMeterProvider(resource *sdkresource.Resource) (*sdkmetric.MeterProvider, http.Handler, error) {
	prometheusExporter, err := prometheus.New()
	if err != nil {
		return nil, nil, err
	}

	// Only attach the Prometheus exporter. The stdoutmetric exporter has been removed.
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource),
		sdkmetric.WithReader(prometheusExporter),
	)

	return meterProvider, promhttp.Handler(), nil
}

func newLoggerProvider(resource *sdkresource.Resource) (*sdklog.LoggerProvider, error) {
	logExporter, err := stdoutlog.New(stdoutlog.WithPrettyPrint())
	if err != nil {
		return nil, err
	}

	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(resource),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
	)
	return loggerProvider, nil
}

func newResource(ctx context.Context) (*sdkresource.Resource, error) {
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "reto-idonia-recog"
	}

	resource, err := sdkresource.Merge(
		sdkresource.DefaultWithContext(ctx),
		sdkresource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	return resource, nil
}
