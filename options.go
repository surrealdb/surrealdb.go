package surrealdb

import "time"

// Option configures a client created with [FromEndpointURLStringWithOptions] or
// [FromConnectionWithOptions].
type Option func(*options)

type options struct {
	versionCheck bool
	tokenRenewal bool
	expiryMargin time.Duration
}

func defaultOptions() options {
	return options{
		versionCheck: false,
		tokenRenewal: true,
		expiryMargin: DefaultTokenExpiryMargin,
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

// WithTokenRenewal turns automatic token renewal on or off. It is on by
// default: once a session holds a token pair, from [DB.SignInWithRefresh],
// [DB.SignUpWithRefresh] or [DB.Refresh], the client exchanges it for a new
// pair shortly before the access token expires, and keeps doing so. The
// current pair is returned by [DB.Tokens].
func WithTokenRenewal(enabled bool) Option {
	return func(o *options) {
		o.tokenRenewal = enabled
	}
}

// WithTokenExpiryMargin sets how long before an access token expires that it
// is renewed. It defaults to [DefaultTokenExpiryMargin]. Access tokens which
// live shorter than twice the margin are renewed half way through their life.
func WithTokenExpiryMargin(margin time.Duration) Option {
	return func(o *options) {
		if margin > 0 {
			o.expiryMargin = margin
		}
	}
}
