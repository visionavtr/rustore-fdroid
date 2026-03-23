package internal

import (
	"fmt"
	"math"
	"time"
)

var (
	retryMaxAttempts = 3
	retryBaseBackoff = 500 * time.Millisecond
)

// WithRetry retries fn up to retryMaxAttempts times with exponential backoff.
func WithRetry[T any](label string, fn func() (T, error)) (T, error) {
	var lastErr error
	for attempt := range retryMaxAttempts {
		result, err := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err

		if attempt == retryMaxAttempts-1 {
			break
		}

		backoff := time.Duration(float64(retryBaseBackoff) * math.Pow(2, float64(attempt)))
		time.Sleep(backoff)
	}
	var zero T
	return zero, fmt.Errorf("%s (after %d attempts): %w", label, retryMaxAttempts, lastErr)
}
