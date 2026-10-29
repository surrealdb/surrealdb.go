package surrealdb

// Option configures a client created with [FromEndpointURLStringWithOptions] or
// [FromConnectionWithOptions].
type Option func(*options)

type options struct {
	versionCheck bool
}

func defaultOptions() options {
	return options{
		versionCheck: false,
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
// It is off by default. When it is on, connecting to a server outside
// [MinimumServerVersion] to [MaximumServerVersion] (exclusive) fails with an
// [UnsupportedVersionError]. Connecting also fails if the server does not
// answer the version request, as when it is started with --deny-rpc version.
// The check sends that request, so it adds a round trip to every connect.
func WithVersionCheck(enabled bool) Option {
	return func(o *options) {
		o.versionCheck = enabled
	}
}
