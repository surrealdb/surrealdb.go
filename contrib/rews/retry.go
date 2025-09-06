package rews

import (
	"math"
	"math/rand"
	"time"
)

// Retryer defines the interface for implementing retry strategies
type Retryer interface {
	// NextDelay returns the delay before the next retry attempt
	// attempt is 0-based (0 for first retry, 1 for second, etc.)
	// Returns the delay duration and whether to continue retrying
	NextDelay(attempt int, lastErr error) (time.Duration, bool)

	// Reset resets the retry strategy state (called on successful connection)
	Reset()
}

// ExponentialBackoffRetryer implements exponential backoff with jitter
type ExponentialBackoffRetryer struct {
	// InitialDelay is the initial retry delay
	InitialDelay time.Duration

	// MaxDelay is the maximum retry delay
	MaxDelay time.Duration

	// Multiplier is the exponential backoff multiplier
	Multiplier float64

	// MaxRetries is the maximum number of retry attempts (0 for infinite)
	MaxRetries int

	// Jitter adds randomness to the delay to avoid thundering herd
	Jitter bool

	// JitterFactor is the maximum jitter as a fraction of the delay (0.0 to 1.0)
	JitterFactor float64
}

// NewExponentialBackoffRetryer creates a new exponential backoff retryer with defaults
func NewExponentialBackoffRetryer() *ExponentialBackoffRetryer {
	return &ExponentialBackoffRetryer{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		MaxRetries:   0, // infinite retries by default
		Jitter:       true,
		JitterFactor: 0.3,
	}
}

// NextDelay implements Retryer
func (r *ExponentialBackoffRetryer) NextDelay(attempt int, lastErr error) (time.Duration, bool) {
	// Check if we've exceeded max retries
	if r.MaxRetries > 0 && attempt >= r.MaxRetries {
		return 0, false
	}

	// Calculate exponential delay
	delay := float64(r.InitialDelay) * math.Pow(r.Multiplier, float64(attempt))

	// Cap at max delay
	if delay > float64(r.MaxDelay) {
		delay = float64(r.MaxDelay)
	}

	// Add jitter if enabled
	if r.Jitter && r.JitterFactor > 0 {
		// Using math/rand is acceptable for jitter in retry delays (non-cryptographic use)
		//nolint:gosec // math/rand is fine for jitter, not security-critical
		jitter := delay * r.JitterFactor * (2*rand.Float64() - 1) // -jitterFactor to +jitterFactor
		delay += jitter
		if delay < 0 {
			delay = float64(r.InitialDelay)
		}
	}

	return time.Duration(delay), true
}

// Reset implements Retryer
func (r *ExponentialBackoffRetryer) Reset() {
	// No state to reset for exponential backoff
}

// FixedDelayRetryer implements a simple fixed delay retry retryer
type FixedDelayRetryer struct {
	// Delay is the fixed delay between retries
	Delay time.Duration

	// MaxRetries is the maximum number of retry attempts (0 for infinite)
	MaxRetries int
}

// NewFixedDelayRetryer creates a new fixed delay retryer
func NewFixedDelayRetryer(delay time.Duration, maxRetries int) *FixedDelayRetryer {
	return &FixedDelayRetryer{
		Delay:      delay,
		MaxRetries: maxRetries,
	}
}

// NextDelay implements Retryer
func (r *FixedDelayRetryer) NextDelay(attempt int, lastErr error) (time.Duration, bool) {
	if r.MaxRetries > 0 && attempt >= r.MaxRetries {
		return 0, false
	}
	return r.Delay, true
}

// Reset implements Retryer
func (r *FixedDelayRetryer) Reset() {
	// No state to reset for fixed delay
}
