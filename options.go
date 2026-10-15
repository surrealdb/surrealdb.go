package surrealdb

import "time"

// Option configures a client created with [FromEndpointURLString] or [FromConnection].
type Option func(*options)

type options struct {
	versionCheck bool
	tokenRenewal bool
	expiryMargin time.Duration

	reconnect         bool
	reconnectMinDelay time.Duration
	reconnectMaxDelay time.Duration
	reconnectAttempts int
}

func defaultOptions() options {
	return options{
		versionCheck: true,
		tokenRenewal: true,
		expiryMargin: DefaultTokenExpiryMargin,

		reconnect:         true,
		reconnectMinDelay: DefaultReconnectMinDelay,
		reconnectMaxDelay: DefaultReconnectMaxDelay,
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

// WithReconnect turns reconnecting on or off. It is on by default, for
// WebSocket connections: when the connection is lost, the client connects
// again, with backoff between attempts, and restores what it held on the
// connection before any other request is sent: the namespace, database,
// variables and authentication of the default session and of each attached
// [Session], and the live queries started with [Live], under the IDs [Live]
// returned. Requests made meanwhile wait for it, until their context ends.
// Requests in flight when the connection was lost fail with
// [connection.ErrConnectionClosed]: they may or may not have run.
//
// What the client cannot restore is lost: open transactions, and live queries
// started with a LIVE SELECT statement rather than [Live]. [DB.OnStatus]
// reports the connection's status as it changes.
//
// Connections passed to [FromConnection] are reconnected when they implement
// [connection.Redialer] and [connection.CloseNotifier], as the gorillaws and
// gws connections do.
func WithReconnect(enabled bool) Option {
	return func(o *options) {
		o.reconnect = enabled
	}
}

// WithReconnectDelay sets the wait between attempts to reconnect, which starts
// at minDelay after the first failed attempt and doubles after each until
// maxDelay. They default to [DefaultReconnectMinDelay] and
// [DefaultReconnectMaxDelay].
func WithReconnectDelay(minDelay, maxDelay time.Duration) Option {
	return func(o *options) {
		if minDelay > 0 {
			o.reconnectMinDelay = minDelay
		}
		if maxDelay > 0 {
			o.reconnectMaxDelay = maxDelay
		}
		o.reconnectMaxDelay = max(o.reconnectMaxDelay, o.reconnectMinDelay)
	}
}

// WithReconnectAttempts sets how many attempts in a row the client makes to
// reconnect before it gives up and closes. It defaults to 0: no limit.
func WithReconnectAttempts(attempts int) Option {
	return func(o *options) {
		o.reconnectAttempts = max(attempts, 0)
	}
}
