package rews

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExponentialBackoffRetryer(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		retryer := NewExponentialBackoffRetryer()

		// First retry (attempt 0)
		delay, shouldRetry := retryer.NextDelay(0, nil)
		assert.True(t, shouldRetry)
		assert.GreaterOrEqual(t, delay, 700*time.Millisecond) // 1s - 30% jitter
		assert.LessOrEqual(t, delay, 1300*time.Millisecond)   // 1s + 30% jitter

		// Second retry (attempt 1)
		delay, shouldRetry = retryer.NextDelay(1, nil)
		assert.True(t, shouldRetry)
		assert.GreaterOrEqual(t, delay, 1400*time.Millisecond) // 2s - 30% jitter
		assert.LessOrEqual(t, delay, 2600*time.Millisecond)    // 2s + 30% jitter

		// Third retry (attempt 2)
		delay, shouldRetry = retryer.NextDelay(2, nil)
		assert.True(t, shouldRetry)
		assert.GreaterOrEqual(t, delay, 2800*time.Millisecond) // 4s - 30% jitter
		assert.LessOrEqual(t, delay, 5200*time.Millisecond)    // 4s + 30% jitter
	})

	t.Run("without jitter", func(t *testing.T) {
		retryer := &ExponentialBackoffRetryer{
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     1 * time.Second,
			Multiplier:   2.0,
			Jitter:       false,
		}

		// First retry
		delay, shouldRetry := retryer.NextDelay(0, nil)
		assert.True(t, shouldRetry)
		assert.Equal(t, 100*time.Millisecond, delay)

		// Second retry
		delay, shouldRetry = retryer.NextDelay(1, nil)
		assert.True(t, shouldRetry)
		assert.Equal(t, 200*time.Millisecond, delay)

		// Third retry
		delay, shouldRetry = retryer.NextDelay(2, nil)
		assert.True(t, shouldRetry)
		assert.Equal(t, 400*time.Millisecond, delay)

		// Fourth retry
		delay, shouldRetry = retryer.NextDelay(3, nil)
		assert.True(t, shouldRetry)
		assert.Equal(t, 800*time.Millisecond, delay)

		// Fifth retry - should hit max delay
		delay, shouldRetry = retryer.NextDelay(4, nil)
		assert.True(t, shouldRetry)
		assert.Equal(t, 1*time.Second, delay)

		// Sixth retry - should still be at max delay
		delay, shouldRetry = retryer.NextDelay(5, nil)
		assert.True(t, shouldRetry)
		assert.Equal(t, 1*time.Second, delay)
	})

	t.Run("with max retries", func(t *testing.T) {
		retryer := &ExponentialBackoffRetryer{
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     10 * time.Second,
			Multiplier:   2.0,
			MaxRetries:   3,
			Jitter:       false,
		}

		// First three retries should succeed
		for i := 0; i < 3; i++ {
			delay, shouldRetry := retryer.NextDelay(i, nil)
			assert.True(t, shouldRetry, "attempt %d should retry", i)
			assert.Greater(t, delay, time.Duration(0))
		}

		// Fourth retry should fail
		delay, shouldRetry := retryer.NextDelay(3, nil)
		assert.False(t, shouldRetry)
		assert.Equal(t, time.Duration(0), delay)
	})

	t.Run("reset does not affect stateless retryer", func(t *testing.T) {
		retryer := NewExponentialBackoffRetryer()
		retryer.Jitter = false // Disable jitter for consistent results

		delay1, _ := retryer.NextDelay(2, nil)
		retryer.Reset()
		delay2, _ := retryer.NextDelay(2, nil)

		assert.Equal(t, delay1, delay2)
	})
}

func TestFixedDelayRetryer(t *testing.T) {
	t.Run("basic operation", func(t *testing.T) {
		retryer := NewFixedDelayRetryer(500*time.Millisecond, 0)

		// All retries should have the same delay
		for i := 0; i < 10; i++ {
			delay, shouldRetry := retryer.NextDelay(i, nil)
			assert.True(t, shouldRetry)
			assert.Equal(t, 500*time.Millisecond, delay)
		}
	})

	t.Run("with max retries", func(t *testing.T) {
		retryer := NewFixedDelayRetryer(100*time.Millisecond, 2)

		// First two retries should succeed
		delay, shouldRetry := retryer.NextDelay(0, nil)
		assert.True(t, shouldRetry)
		assert.Equal(t, 100*time.Millisecond, delay)

		delay, shouldRetry = retryer.NextDelay(1, nil)
		assert.True(t, shouldRetry)
		assert.Equal(t, 100*time.Millisecond, delay)

		// Third retry should fail
		delay, shouldRetry = retryer.NextDelay(2, nil)
		assert.False(t, shouldRetry)
		assert.Equal(t, time.Duration(0), delay)
	})
}
