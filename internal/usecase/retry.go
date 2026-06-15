package usecase

import (
	"context"

	"github.com/avast/retry-go/v4"
)

func retryWithBackoff(ctx context.Context, fn func() error) error {
	return retry.Do(
		fn,
		retry.Attempts(3),
		retry.DelayType(retry.BackOffDelay),
		retry.Context(ctx),
	)
}
