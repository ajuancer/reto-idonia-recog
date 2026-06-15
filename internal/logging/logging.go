package logging

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"reto-idonia-recog-refactored/internal/config"
)

const (
	formatJSON = "json"
	formatText = "text"
)

var defaultRedactKeys = []string{
	"access_token",
	"api_key",
	"apikey",
	"authorization",
	"csrf",
	"dicompatientid",
	"email",
	"filename",
	"jwt",
	"password",
	"patient_id",
	"patientid",
	"pin",
	"refresh_token",
	"secret",
	"ssn",
	"token",
}

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(payload []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	written, err := w.ResponseWriter.Write(payload)
	w.bytes += written
	return written, err
}

// HTTPMiddleware logs incoming HTTP requests.
// Improved: Uses Context propagation to ensure Trace IDs or other context values are logged.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		writer := &responseWriter{ResponseWriter: w}

		next.ServeHTTP(writer, r)

		status := writer.status
		if status == 0 {
			status = http.StatusOK
		}

		duration := time.Since(start)
		logger := slog.Default()

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration_ms", duration.Milliseconds(),
			"bytes", writer.bytes,
			"remote_addr", r.RemoteAddr,
		}

		switch {
		case status >= http.StatusInternalServerError:
			logger.ErrorContext(r.Context(), "HTTP request completed", attrs...)
		case status >= http.StatusBadRequest:
			logger.WarnContext(r.Context(), "HTTP request completed", attrs...)
		default:
			logger.InfoContext(r.Context(), "HTTP request completed", attrs...)
		}
	})
}

// Setup initializes the slog logger.
func Setup(cfg *config.Config, service string, out io.Writer) (*slog.Logger, *slog.LevelVar) {
	level, levelWarning := parseLevel(cfg.LogLevel)
	format, formatWarning := parseFormat(cfg.LogFormat, cfg.Env)
	redactKeys := parseRedactKeys(cfg.LogRedactKeys)

	// Use LevelVar for atomic, concurrent-safe log level changes at runtime
	logLevel := new(slog.LevelVar)
	logLevel.Set(level)

	handlerOptions := &slog.HandlerOptions{
		Level:     logLevel,
		AddSource: !strings.EqualFold(cfg.Env, "production"),
	}

	var handler slog.Handler
	if format == formatJSON {
		handler = slog.NewJSONHandler(out, handlerOptions)
	} else {
		handler = slog.NewTextHandler(out, handlerOptions)
	}

	if strings.EqualFold(cfg.Env, "production") {
		// Assuming NewRedactingHandler is implemented elsewhere in the package
		handler = NewRedactingHandler(handler, redactKeys)
	}

	// Assuming MyTesseraSetup, NewAsyncAppender, and NewTesseraHandler are implemented elsewhere
	tesseraAppender, _ := MyTesseraSetup(cfg)
	if tesseraAppender != nil {
		// Wrap the synchronous appender in the Async wrapper with a buffer of 1000 logs
		asyncAppender := NewAsyncAppender(tesseraAppender, 1000)
		handler = NewTesseraHandler(handler, asyncAppender)
	}

	logger := slog.New(handler)
	if service != "" {
		logger = logger.With("service", service)
	}
	slog.SetDefault(logger)

	if levelWarning != "" {
		logger.Warn(levelWarning, "value", cfg.LogLevel)
	}
	if formatWarning != "" {
		logger.Warn(formatWarning, "value", cfg.LogFormat)
	}

	return logger, logLevel
}

func parseLevel(raw string) (slog.Level, string) {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case "":
		return slog.LevelInfo, ""
	case "debug":
		return slog.LevelDebug, ""
	case "info":
		return slog.LevelInfo, ""
	case "warn", "warning":
		return slog.LevelWarn, ""
	case "error":
		return slog.LevelError, ""
	default:
		return slog.LevelInfo, fmt.Sprintf("Invalid LOG_LEVEL %q, defaulting to info", raw)
	}
}

func parseFormat(raw string, env string) (string, string) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		if strings.EqualFold(env, "production") {
			return formatJSON, ""
		}
		return formatText, ""
	}

	switch value {
	case formatJSON, formatText:
		return value, ""
	default:
		fallback := formatText
		if strings.EqualFold(env, "production") {
			fallback = formatJSON
		}
		return fallback, fmt.Sprintf("Invalid LOG_FORMAT %q, defaulting to %s", raw, fallback)
	}
}

func parseRedactKeys(raw string) []string {
	keys := map[string]struct{}{}
	for _, key := range defaultRedactKeys {
		if normalized := normalizeKey(key); normalized != "" {
			keys[normalized] = struct{}{}
		}
	}

	for _, key := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '	'
	}) {
		if normalized := normalizeKey(key); normalized != "" {
			keys[normalized] = struct{}{}
		}
	}

	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	return result
}

func normalizeKey(key string) string {
	return strings.TrimSpace(strings.ToLower(key))
}
