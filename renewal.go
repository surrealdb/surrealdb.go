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

	mu     sync.Mutex
	tokens *Tokens
	timer  *time.Timer
	// cycle is bumped whenever the tokens change, so that a renewal started
	// for older tokens does not overwrite newer ones.
	cycle uint64
}

func newRenewer(margin time.Duration, refresh func(context.Context, Tokens) (*Tokens, error)) *renewer {
	return &renewer{margin: margin, refresh: refresh, log: logger.New(slog.Default().Handler())}
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
	if tokens == nil || tokens.Refresh == "" {
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

func (r *renewer) stopLocked() {
	r.cycle++
	r.tokens = nil
	if r.timer != nil {
		r.timer.Stop()
		r.timer = nil
	}
}

func (r *renewer) renew(cycle uint64, attempt int) {
	r.mu.Lock()
	if cycle != r.cycle || r.tokens == nil {
		r.mu.Unlock()
		return
	}
	tokens := *r.tokens
	r.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), renewalTimeout)
	next, err := r.refresh(ctx, tokens)
	cancel()

	r.mu.Lock()
	defer r.mu.Unlock()
	if cycle != r.cycle {
		return
	}

	if err == nil && next != nil {
		r.setLocked(next)
		return
	}

	exp, ok := tokenExpiry(tokens.Access)
	remaining := time.Until(exp)
	backoff := min(time.Second<<attempt, renewalMaxBackoff)
	if !ok || backoff >= remaining {
		r.log.Error("giving up renewing the access token", "error", err)
		return
	}
	r.log.Warn("renewing the access token failed; retrying", "error", err, "in", backoff)
	r.timer = time.AfterFunc(backoff, func() { r.renew(cycle, attempt+1) })
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
