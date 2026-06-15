package pkg

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/ledongthuc/pdf"
)

func ExtractText(filePath string) (string, error) {
	file, reader, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	defer file.Close()

	textReader, err := reader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("extract pdf text: %w", err)
	}

	var buffer bytes.Buffer
	if _, err := io.Copy(&buffer, textReader); err != nil {
		return "", fmt.Errorf("read pdf text: %w", err)
	}

	rawText := buffer.String()

	// Normalize to be all lines
	cleanText := strings.Join(strings.Fields(rawText), " ")

	if cleanText == "" {
		return "", errors.New("pdf text is empty")
	}

	slog.Debug("Extracted PDF text", "length", len(cleanText))
	return cleanText, nil
}
