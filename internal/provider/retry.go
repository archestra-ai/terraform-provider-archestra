package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// RetryConfig configures retry behavior for operations that may need to wait
// for asynchronous resource availability.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts (default: 10)
	MaxRetries int
	// InitialBackoff is the initial wait time between retries (default: 500ms)
	InitialBackoff time.Duration
	// MaxBackoff is the maximum wait time between retries (default: 5s)
	MaxBackoff time.Duration
	// Description is used for logging purposes
	Description string
}

// DefaultRetryConfig returns the default retry configuration.
// This is tuned for waiting for asynchronously assigned resources like built-in tools.
func DefaultRetryConfig(description string) RetryConfig {
	return RetryConfig{
		MaxRetries:     20,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     5 * time.Second,
		Description:    description,
	}
}

// RetryResult represents the result of a retry operation.
type RetryResult[T any] struct {
	Value T
	Found bool
}

// RetryUntilFound retries an operation until it returns a found result or max retries is reached.
// The operation function should return (value, found, error).
// - If found is true, the value is returned immediately.
// - If found is false and error is nil, the operation is retried.
// - If error is non-nil, it's returned immediately without retrying.
func RetryUntilFound[T any](ctx context.Context, config RetryConfig, operation func() (T, bool, error)) (T, bool, error) {
	var zero T
	backoff := config.InitialBackoff

	for attempt := 0; attempt < config.MaxRetries; attempt++ {
		value, found, err := operation()
		if err != nil {
			return zero, false, err
		}
		if found {
			return value, true, nil
		}

		// Not found, wait and retry
		if attempt < config.MaxRetries-1 {
			tflog.Debug(ctx, fmt.Sprintf("%s not yet available, retrying in %v (attempt %d/%d)",
				config.Description, backoff, attempt+1, config.MaxRetries))

			select {
			case <-ctx.Done():
				return zero, false, ctx.Err()
			case <-time.After(backoff):
				// Continue with next attempt
			}

			// Exponential backoff with cap
			backoff = backoff * 2
			if backoff > config.MaxBackoff {
				backoff = config.MaxBackoff
			}
		}
	}

	return zero, false, nil
}
