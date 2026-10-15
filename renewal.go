package surrealdb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/logger"
)

const (
	// DefaultTokenExpiryMargin is how long before an access token expires
	// that the client renews it.
	DefaultTokenExpiryMargin = time.Minute

	renewalTimeout    = 30 * time.Second
	renewalMaxBackoff = 30 * time.Second
)

// renewer keeps the token pair of one session fresh. It renews the pair with
// the refresh RPC shortly before the access token expires, and retries a failed
// renewal with backoff for as long as the access token is still valid.
//
// A nil renewer does nothing, so that renewal can be turned off by not
// creating one.
type renewer struct {
	margin  time.Duration
	refresh func(ctx context.Context, tokens Tokens) (*Tokens, error)
	log     logger.Logger

	// auth is held while the session's authentication changes: by a renewal
	// for the whole of its refresh request, and by each method which signs
	// the session in or out. A renewal's answer, which the connection may act
	// on, then never lands after a newer sign-in.
	auth chan struct{}

	mu     sync.Mutex
	tokens *Tokens
	timer  *time.Timer
	// cycle is bumped whenever the tokens change, so that a renewal started
	// for older tokens does not overwrite newer ones.
	cycle uint64
	// stopped is set when the client closes, after which nothing is renewed.
	stopped bool
	// inflight is the renewal whose refresh request is under way, if any.
	inflight *inflightRenewal
}

// inflightRenewal is a refresh request under way, which stop cancels and
// waits for.
type inflightRenewal struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func newRenewer(margin time.Duration, refresh func(context.Context, Tokens) (*Tokens, error)) *renewer {
	return &renewer{
		margin:  margin,
		refresh: refresh,
		log:     logger.New(slog.Default().Handler()),
		auth:    make(chan struct{}, 1),
	}
}

// lockAuth waits until no renewal or other authentication change is under way,
// or until ctx is done, and then holds off others until unlockAuth.
func (r *renewer) lockAuth(ctx context.Context) error {
	if r == nil {
		return nil
	}
	select {
	case r.auth <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *renewer) unlockAuth() {
	if r == nil {
		return
	}
	<-r.auth
}

// current returns the token pair the session holds, if it holds one.
func (r *renewer) current() *Tokens {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tokens == nil {
		return nil
	}
	t := *r.tokens
	return &t
}

// set records a new token pair and schedules its renewal.
func (r *renewer) set(tokens *Tokens) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.setLocked(tokens)
}

func (r *renewer) setLocked(tokens *Tokens) {
	r.stopLocked()
	if r.stopped || tokens == nil || tokens.Refresh == "" {
		return
	}
	t := *tokens
	r.tokens = &t

	exp, ok := tokenExpiry(t.Access)
	if !ok {
		return
	}
	remaining := time.Until(exp)
	cycle := r.cycle
	r.timer = time.AfterFunc(renewalDelay(remaining, r.margin), func() { r.renew(cycle, 0) })
}

// clear forgets the token pair, for when the session signs out or
// authenticates some other way.
func (r *renewer) clear() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
}

// stop forgets the token pair for good, for when the client closes. It
// cancels a refresh request under way and waits for it to return, so that
// nothing is renewed once stop has returned.
//
// The caller must not hold a lock the refresh function takes.
func (r *renewer) stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.stopped = true
	r.stopLocked()
	inflight := r.inflight
	r.mu.Unlock()

	if inflight != nil {
		inflight.cancel()
		<-inflight.done
	}
}

func (r *renewer) stopLocked() {
	r.cycle++
	r.tokens = nil
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}

func (r *renewer) renew(cycle uint64, attempt int) {
	r.auth <- struct{}{}
	defer func() { <-r.auth }()

	r.mu.Lock()
	if cycle != r.cycle || r.tokens == nil {
		r.mu.Unlock()
		return
	}
	tokens := *r.tokens
	ctx, cancel := context.WithTimeout(context.Background(), renewalTimeout)
	inflight := &inflightRenewal{cancel: cancel, done: make(chan struct{})}
	r.inflight = inflight
	r.mu.Unlock()

	next, err := r.refresh(ctx, tokens)
	cancel()
	close(inflight.done)

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.inflight == inflight {
		r.inflight = nil
	}
	// A renewal stopped, or overtaken by newer tokens, while its request was
	// under way changes nothing.
	if cycle != r.cycle {
		return
	}

	if err == nil && next != nil {
		r.setLocked(next)
		return
	}

	exp, ok := tokenExpiry(tokens.Access)
	remaining := time.Until(exp)
	backoff := retryBackoff(attempt)
	if !ok || backoff >= remaining {
		r.log.Error("giving up renewing the access token", "error", err)
		return
	}
	r.log.Warn("renewing the access token failed; retrying", "error", err, "in", backoff)
	r.timer = time.AfterFunc(backoff, func() { r.renew(cycle, attempt+1) })
}

// retryBackoff is how long to wait before retrying a renewal which failed
// attempt+1 times: a second, doubling up to renewalMaxBackoff.
func retryBackoff(attempt int) time.Duration {
	// 1<<5 seconds is past the maximum already; a larger shift would overflow.
	return min(time.Second<<min(max(attempt, 0), 5), renewalMaxBackoff)
}

// renewalDelay is how long to wait before renewing a token with the given time
// left: margin before it expires, or half way through when the token lives
// shorter than twice the margin.
func renewalDelay(remaining, margin time.Duration) time.Duration {
	if remaining <= 0 {
		return 0
	}
	return min(remaining, max(remaining-margin, remaining/2))
}

// tokenExpiry reads the exp claim of a JWT without verifying it.
func tokenExpiry(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp *float64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == nil {
		return time.Time{}, false
	}
	return time.Unix(int64(*claims.Exp), 0), true
}
