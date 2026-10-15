package surrealdb

// Option configures a client created with [FromEndpointURLString] or [FromConnection].
type Option func(*options)

type options struct {
	versionCheck bool
}

func defaultOptions() options {
	return options{
		versionCheck: true,
	}
}

func applyOptions(opts []Option) options {
	o := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

// WithVersionCheck turns the server version check made on connecting on or off.
// It is on by default: connecting to a server outside
// [MinimumServerVersion] to [MaximumServerVersion] (exclusive) fails with an
// [UnsupportedVersionError].
func WithVersionCheck(enabled bool) Option {
	return func(o *options) {
		o.versionCheck = enabled
	}
}
