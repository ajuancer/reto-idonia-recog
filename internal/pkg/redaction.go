package pkg

import (
	"context"
	"strings"

	"github.com/taoq-ai/wuming"
)

// Redact text removes Personally Identifiable Information from the input string
func Redact(ctx context.Context, text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return text, nil
	}
	return wuming.Redact(ctx, text)
}
