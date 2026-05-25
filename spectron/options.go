package spectron

import "time"

const (
	// DefaultTimeout is the per-request timeout applied when no
	// per-call deadline is set via context.
	DefaultTimeout = 30 * time.Second

	// DefaultMaxRetries is the default cap on retry attempts for
	// GETs and idempotent writes.
	DefaultMaxRetries = 3

	// DefaultUserAgent identifies this SDK in the User-Agent header.
	DefaultUserAgent = "surrealdb-go-spectron/1.0"
)

// config holds all knobs the Client respects. Constructed via the
// functional options passed to [New].
type config struct {
	timeout    time.Duration
	maxRetries int
	userAgent  string
}

func defaultConfig() config {
	return config{
		timeout:    DefaultTimeout,
		maxRetries: DefaultMaxRetries,
		userAgent:  DefaultUserAgent,
	}
}

// Option configures a [Client].
type Option func(*config)

// WithTimeout overrides the per-request timeout (default 30s).
func WithTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout = d }
}

// WithMaxRetries overrides the retry cap for GETs and idempotent writes
// (default 3). Values are clamped to the available backoff schedule.
func WithMaxRetries(n int) Option {
	return func(c *config) { c.maxRetries = n }
}

// WithUserAgent overrides the User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *config) {
		if ua != "" {
			c.userAgent = ua
		}
	}
}
