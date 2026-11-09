package internal

import (
	"errors"
	"fmt"
	"math"
	"time"
)

var (
	retryMaxAttempts = 3
	retryBaseBackoff = 500 * time.Millisecond
)

type permanentError struct {
	err error
}

func (e *permanentError) Error() string { return e.err.Error() }

func (e *permanentError) Unwrap() error { return e.err }

func permanent(err error) error {
	return &permanentError{err: err}
}

// WithRetry retries fn up to retryMaxAttempts times with exponential backoff.
// Errors wrapped with permanent are returned without further attempts.
func WithRetry[T any](label string, fn func() (T, error)) (T, error) {
	var zero T
	var lastErr error
	for attempt := range retryMaxAttempts {
		result, err := fn()
		if err == nil {
			return result, nil
		}
		var perm *permanentError
		if errors.As(err, &perm) {
			return zero, fmt.Errorf("%s: %w", label, perm.err)
		}
		lastErr = err

		if attempt == retryMaxAttempts-1 {
			break
		}

		backoff := time.Duration(float64(retryBaseBackoff) * math.Pow(2, float64(attempt)))
		time.Sleep(backoff)
	}
	return zero, fmt.Errorf("%s (after %d attempts): %w", label, retryMaxAttempts, lastErr)
}
