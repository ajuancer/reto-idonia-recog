package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"reto-idonia-recog-refactored/internal/clients/idonia"
	"reto-idonia-recog-refactored/internal/clients/recog"
	"reto-idonia-recog-refactored/internal/config"
	"reto-idonia-recog-refactored/internal/logging"
	"reto-idonia-recog-refactored/internal/otel"
	"reto-idonia-recog-refactored/internal/usecase"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	_ = godotenv.Load(".env/.local")

	rootCmd := &cobra.Command{
		Use:   "uploader",
		Short: "Medical file uploader CLI and Daemon",
		Long:  `A suite of tools to batch upload medical files or watch local directories for automated processing.`,
	}

	var deleteAfterUpload bool

	uploadCmd := &cobra.Command{
		Use:   "upload [directory]",
		Short: "Perform a one-off upload of a specific directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dirPath := args[0]
			if err := validateDirectory(dirPath); err != nil {
				return err
			}

			orchestrator, logger, cleanup, err := setupDependencies(ctx, "cli-upload")
			if err != nil {
				return err
			}
			defer cleanup()

			logger.Info("Starting manual file processing...", "directory", dirPath)
			result, err := orchestrator.ProcessDirectory(ctx, dirPath)
			if err != nil {
				logger.Error("Upload failed", "error", err)
				return err
			}

			printSuccess(result.ViewerURL, result.PIN)

			if deleteAfterUpload {
				logger.Info("Cleanup flag provided. Deleting processed files...")
				cleanupProcessedFiles(dirPath, logger)
			}

			return nil
		},
	}

	uploadCmd.Flags().BoolVarP(&deleteAfterUpload, "delete", "d", false, "Delete files from the directory after a successful upload")

	watchCmd := &cobra.Command{
		Use:   "watch [directory]",
		Short: "Start a background daemon to watch a directory for new files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dirPath := args[0]
			if err := validateDirectory(dirPath); err != nil {
				return err
			}

			orchestrator, logger, cleanup, err := setupDependencies(ctx, "cli-watcher")
			if err != nil {
				return err
			}
			defer cleanup()

			logger.Info("Starting background watcher...", "directory", dirPath)
			return RunWatcher(ctx, dirPath, orchestrator, logger)
		},
	}

	rootCmd.AddCommand(uploadCmd, watchCmd)

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

// setupDependencies handles the boilerplate initialization for Config, Logging, OTel, and DI.
func setupDependencies(ctx context.Context, serviceName string) (*usecase.Orchestrator, *slog.Logger, func(), error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("configuration error: %w", err)
	}

	logger, _ := logging.Setup(cfg, serviceName, os.Stdout)

	otelShutdown, _, err := otel.Setup(ctx)
	if err != nil {
		logger.Error("OpenTelemetry setup failed", "error", err)
		return nil, nil, nil, err
	}

	idoniaClient := idonia.NewClient(cfg)
	recogClient := recog.NewClient(cfg)
	orchestrator := usecase.NewOrchestrator(idoniaClient, recogClient)

	cleanup := func() {
		logger.Info("Flushing telemetry data...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := otelShutdown(shutdownCtx); err != nil {
			logger.Error("OpenTelemetry shutdown failed", "error", err)
		}
	}

	return orchestrator, logger, cleanup, nil
}

func validateDirectory(dirPath string) error {
	info, err := os.Stat(dirPath)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("invalid directory path '%s': %w", dirPath, err)
	}
	return nil
}

func printSuccess(viewerURL, pin string) {
	fmt.Printf("\033[32mFiles uploaded and humanized report created successfully. Data is at %s with pin %s\033[0m\n", viewerURL, pin)
}

func RunWatcher(ctx context.Context, dirPath string, orchestrator *usecase.Orchestrator, logger *slog.Logger) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	if err := watcher.Add(dirPath); err != nil {
		return fmt.Errorf("failed to watch directory %s: %w", dirPath, err)
	}

	logger.Info("Daemon active. Listening for new files...")

	var processTimer *time.Timer
	debounceDuration := 5 * time.Second

	for {
		select {
		case <-ctx.Done():
			logger.Info("Shutting down file watcher...")
			if processTimer != nil {
				processTimer.Stop()
			}
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
				logger.Debug("File activity detected", "file", event.Name, "operation", event.Op.String())

				if processTimer != nil {
					processTimer.Stop()
				}

				processTimer = time.AfterFunc(debounceDuration, func() {
					logger.Info("File transfers settled. Processing batch...")

					result, err := orchestrator.ProcessDirectory(context.Background(), dirPath)
					if err != nil {
						logger.Error("Upload batch failed", "error", err)
						return
					}

					printSuccess(result.ViewerURL, result.PIN)
					cleanupProcessedFiles(dirPath, logger)
				})
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			logger.Error("File watcher encountered an error", "error", err)
		}
	}
}

func cleanupProcessedFiles(dirPath string, logger *slog.Logger) {
	logger.Info("Cleaning up processed files from hot folder...")

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		logger.Error("Failed to read directory for cleanup", "error", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			fullPath := filepath.Join(dirPath, entry.Name())
			if err := os.Remove(fullPath); err != nil {
				logger.Error("Failed to delete processed file", "file", entry.Name(), "error", err)
			}
		}
	}
}
