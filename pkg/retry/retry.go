package retry

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
)

// Options holds parameters for the retry logic
type Options struct {
	InitialInterval     time.Duration
	MaxInterval         time.Duration
	MaxElapsedTime      time.Duration
	RandomizationFactor float64
	Multiplier          float64
}

// DefaultOptions returns a sensible default for CLI network operations
func DefaultOptions() Options {
	return Options{
		InitialInterval:     1 * time.Second,
		MaxInterval:         15 * time.Second,
		MaxElapsedTime:      2 * time.Minute,
		RandomizationFactor: 0.5,
		Multiplier:          1.5,
	}
}

// Do executes the operation op with the provided context and options.
// Uses an exponential backoff strategy.
func Do(ctx context.Context, opts Options, op func() error) error {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = opts.InitialInterval
	b.MaxInterval = opts.MaxInterval
	b.MaxElapsedTime = opts.MaxElapsedTime
	b.RandomizationFactor = opts.RandomizationFactor
	b.Multiplier = opts.Multiplier
	b.Reset()

	// Apply context cancellation if necessary
	cb := backoff.WithContext(b, ctx)

	return backoff.Retry(op, cb)
}

// DoWithDefault is a helper for executing an operation with DefaultOptions.
func DoWithDefault(ctx context.Context, op func() error) error {
	return Do(ctx, DefaultOptions(), op)
}
