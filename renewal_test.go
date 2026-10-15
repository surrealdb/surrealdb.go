package surrealdb

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jwtExpiringIn returns an unsigned JWT whose exp claim is d from now.
func jwtExpiringIn(d time.Duration) string {
	enc := base64.RawURLEncoding.EncodeToString
	payload := fmt.Sprintf(`{"exp":%d}`, time.Now().Add(d).Unix())
	return enc([]byte(`{"alg":"none"}`)) + "." + enc([]byte(payload)) + ".sig"
}

func TestTokenExpiry(t *testing.T) {
	exp, ok := tokenExpiry(jwtExpiringIn(time.Hour))
	require.True(t, ok)
	assert.WithinDuration(t, time.Now().Add(time.Hour), exp, 2*time.Second)

	for _, token := range []string{"", "not-a-jwt", "a.b.c", "a." + base64.RawURLEncoding.EncodeToString([]byte(`{}`)) + ".c"} {
		_, ok := tokenExpiry(token)
		assert.False(t, ok, token)
	}
}

func TestRenewalDelay(t *testing.T) {
	assert.Equal(t, 59*time.Minute, renewalDelay(time.Hour, time.Minute))
	assert.Equal(t, 45*time.Second, renewalDelay(90*time.Second, time.Minute), "never earlier than half way")
	assert.Equal(t, 5*time.Second, renewalDelay(10*time.Second, time.Minute))
	assert.Equal(t, time.Duration(0), renewalDelay(-time.Second, time.Minute))
}

func TestRenewer_renewsAndRetries(t *testing.T) {
	var calls atomic.Int32
	renewed := make(chan Tokens, 10)

	r := newRenewer(time.Minute, func(_ context.Context, tokens Tokens) (*Tokens, error) {
		// The first attempt fails; the retry succeeds with a long-lived pair.
		if calls.Add(1) == 1 {
			return nil, errors.New("server unavailable")
		}
		next := Tokens{Access: jwtExpiringIn(time.Hour), Refresh: tokens.Refresh + "+"}
		renewed <- next
		return &next, nil
	})
	defer r.clear()

	r.set(&Tokens{Access: jwtExpiringIn(4 * time.Second), Refresh: "r"})

	select {
	case next := <-renewed:
		assert.Equal(t, "r+", next.Refresh)
		assert.Equal(t, &next, r.current())
	case <-time.After(6 * time.Second):
		t.Fatal("the pair was not renewed")
	}
	assert.Equal(t, int32(2), calls.Load())
}

func TestRenewer_clearStopsRenewal(t *testing.T) {
	var calls atomic.Int32
	r := newRenewer(time.Minute, func(context.Context, Tokens) (*Tokens, error) {
		calls.Add(1)
		return nil, errors.New("unexpected")
	})

	r.set(&Tokens{Access: jwtExpiringIn(time.Second), Refresh: "r"})
	r.clear()
	time.Sleep(1500 * time.Millisecond)

	assert.Zero(t, calls.Load())
	assert.Nil(t, r.current())
}

func TestRenewer_nil(t *testing.T) {
	var r *renewer
	r.set(&Tokens{Access: "a", Refresh: "r"})
	r.clear()
	assert.Nil(t, r.current())
}
