package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"reto-idonia-recog-refactored/internal/clients/idonia"
	"reto-idonia-recog-refactored/internal/clients/recog"
	"reto-idonia-recog-refactored/internal/config"
	"reto-idonia-recog-refactored/internal/idempotency"
	"reto-idonia-recog-refactored/internal/logging"
	"reto-idonia-recog-refactored/internal/otel"
	"reto-idonia-recog-refactored/internal/usecase"
	"runtime"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	httptransport "reto-idonia-recog-refactored/internal/transport/http"
)

func main() {
	_ = godotenv.Load(".env/.local")

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Configuration Error: %v\n", err)
		os.Exit(1)
	}

	logger, _ := logging.Setup(cfg, "api", os.Stdout)
	logger.Info("Starting service initialized")

	otelShutdown, metricsHandler, err := otel.Setup(context.Background())
	if err != nil {
		logger.Error("OpenTelemetry setup failed", "error", err)
		os.Exit(1)
	}

	redisOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		redisOptions = &redis.Options{Addr: cfg.RedisURL}
	}
	redisClient := redis.NewClient(redisOptions)
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Warn("Redis close failed", "error", err)
		}
	}()

	idoniaClient := idonia.NewClient(cfg)
	recogClient := recog.NewClient(cfg)
	orchestrator := usecase.NewOrchestrator(idoniaClient, recogClient)
	jobQueue := usecase.NewRedisJobQueue(redisClient, 100, cfg.RedisTTL)
	workerPool := usecase.NewWorkerPool(jobQueue, orchestrator, logger, runtime.GOMAXPROCS(0))
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	workerPool.Start(workerCtx)

	idempotencyStore := idempotency.NewRedisStore(redisClient, cfg.RedisTTL)
	router := httptransport.NewRouter(cfg, logger, orchestrator, jobQueue, metricsHandler, idempotencyStore)

	server := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("Starting HTTP server", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server crashed", "error", err)
			os.Exit(1)
		}
	}()

	<-stopChan
	logger.Info("Received termination signal. Shutting down...")
	workerCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("HTTP server forced to shutdown", "error", err)
	}

	if err := otelShutdown(ctx); err != nil {
		logger.Error("OpenTelemetry shutdown failed", "error", err)
	}

	logger.Info("Server stopped successfully")
}
