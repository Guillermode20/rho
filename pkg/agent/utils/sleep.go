package agentutils

import (
	"context"
	"math/rand"
	"time"
)

// Sleep pauses for the given duration.
func Sleep(d time.Duration) {
	time.Sleep(d)
}

// SleepWithContext pauses for the given duration or until the context is cancelled.
func SleepWithContext(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// BackoffConfig configures exponential backoff.
type BackoffConfig struct {
	Initial time.Duration
	Max     time.Duration
	Factor  float64
	Jitter  bool
}

// DefaultBackoffConfig returns sensible defaults.
func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		Initial: 1 * time.Second,
		Max:     60 * time.Second,
		Factor:  2.0,
		Jitter:  true,
	}
}

// BackoffSleep sleeps for the appropriate backoff duration for the given attempt.
func BackoffSleep(attempt int, cfg BackoffConfig) {
	d := backoffDuration(attempt, cfg)
	Sleep(d)
}

// BackoffSleepWithContext sleeps for backoff duration or until context cancelled.
func BackoffSleepWithContext(ctx context.Context, attempt int, cfg BackoffConfig) error {
	d := backoffDuration(attempt, cfg)
	return SleepWithContext(ctx, d)
}

func backoffDuration(attempt int, cfg BackoffConfig) time.Duration {
	if cfg.Initial == 0 {
		cfg.Initial = 1 * time.Second
	}
	if cfg.Max == 0 {
		cfg.Max = 60 * time.Second
	}
	if cfg.Factor == 0 {
		cfg.Factor = 2.0
	}

	d := float64(cfg.Initial)
	for i := 0; i < attempt; i++ {
		d *= cfg.Factor
		if d > float64(cfg.Max) {
			d = float64(cfg.Max)
			break
		}
	}

	if cfg.Jitter {
		// Add ±25% jitter
		jitter := d * 0.25
		d = d - jitter + rand.Float64()*jitter*2
	}

	return time.Duration(d)
}

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxRetries int
	Backoff    BackoffConfig
}

// DefaultRetryConfig returns sensible defaults for retries.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		Backoff:    DefaultBackoffConfig(),
	}
}

// RetryWithBackoff retries a function with exponential backoff.
func RetryWithBackoff(ctx context.Context, cfg RetryConfig, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			if err := BackoffSleepWithContext(ctx, attempt-1, cfg.Backoff); err != nil {
				return err
			}
		}
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}
