package surrealdb

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
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

	var next Tokens
	select {
	case next = <-renewed:
		assert.Equal(t, "r+", next.Refresh)
	case <-time.After(6 * time.Second):
		t.Fatal("the pair was not renewed")
	}
	// The refresh function returns before the renewer records its result.
	assert.Eventually(t, func() bool {
		cur := r.current()
		return cur != nil && *cur == next
	}, time.Second, 5*time.Millisecond)
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
	r.stop()
	require.NoError(t, r.lockAuth(context.Background()))
	r.unlockAuth()
	assert.Nil(t, r.current())
}

func TestRetryBackoff(t *testing.T) {
	assert.Equal(t, time.Second, retryBackoff(0))
	assert.Equal(t, 16*time.Second, retryBackoff(4))
	for attempt := 5; attempt < 200; attempt++ {
		require.Equal(t, renewalMaxBackoff, retryBackoff(attempt), "attempt %d", attempt)
	}
}

// An authentication change waits for a renewal under way, so the renewal's
// answer can't land after it.
func TestRenewer_authChangesWaitForRenewal(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	r := newRenewer(time.Minute, func(context.Context, Tokens) (*Tokens, error) {
		close(started)
		<-release
		next := Tokens{Access: jwtExpiringIn(time.Hour), Refresh: "r2"}
		return &next, nil
	})
	defer r.stop()

	r.set(&Tokens{Access: jwtExpiringIn(2 * time.Second), Refresh: "r"})
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the pair was not renewed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, r.lockAuth(ctx), context.DeadlineExceeded, "a sign-in waits for the renewal")

	close(release)
	require.NoError(t, r.lockAuth(context.Background()))
	r.clear()
	r.unlockAuth()
	assert.Nil(t, r.current(), "the sign-in that followed the renewal wins")
}

// A renewal waits for an authentication change under way, and then finds the
// pair it was for has gone.
func TestRenewer_renewalWaitsForAuthChanges(t *testing.T) {
	var calls atomic.Int32
	r := newRenewer(time.Minute, func(context.Context, Tokens) (*Tokens, error) {
		calls.Add(1)
		return nil, errors.New("unexpected")
	})
	defer r.stop()

	require.NoError(t, r.lockAuth(context.Background()))
	r.set(&Tokens{Access: jwtExpiringIn(time.Second), Refresh: "r"})
	time.Sleep(time.Second)
	r.clear()
	r.unlockAuth()

	time.Sleep(200 * time.Millisecond)
	assert.Zero(t, calls.Load())
}

type closeOnlyConnection struct{ connection.Connection }

func (closeOnlyConnection) Close(context.Context) error { return nil }

// Closing the client stops the renewal of every session, including attached
// sessions which were never detached.
func TestDB_closeStopsEveryRenewal(t *testing.T) {
	var calls atomic.Int32
	refresh := func(context.Context, Tokens) (*Tokens, error) {
		calls.Add(1)
		return nil, errors.New("unexpected")
	}
	db := &DB{con: closeOnlyConnection{}, renew: newRenewer(time.Minute, refresh)}
	session := newRenewer(time.Minute, refresh)
	db.sessions = map[models.UUID]*Session{{}: {renew: session}}

	pair := &Tokens{Access: jwtExpiringIn(time.Second), Refresh: "r"}
	db.renew.set(pair)
	session.set(pair)
	require.NoError(t, db.Close(context.Background()))
	time.Sleep(1500 * time.Millisecond)

	assert.Zero(t, calls.Load())
	assert.Nil(t, session.current())
	session.set(pair)
	assert.Nil(t, session.current(), "a closed client renews nothing")
}

// blockingRefresh is a refresh which blocks until its context is done, and
// then answers with a new pair anyway, as a server might have.
type blockingRefresh struct {
	started chan struct{}
	exited  atomic.Bool
}

func newBlockingRefresh() *blockingRefresh {
	return &blockingRefresh{started: make(chan struct{}, 1)}
}

func (b *blockingRefresh) refresh(ctx context.Context) (*Tokens, error) {
	defer b.exited.Store(true)
	b.started <- struct{}{}
	<-ctx.Done()
	return &Tokens{Access: jwtExpiringIn(time.Hour), Refresh: "late"}, nil
}

func (b *blockingRefresh) waitStarted(t *testing.T) {
	t.Helper()
	select {
	case <-b.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the renewal did not start")
	}
}

// Closing the client cancels a renewal under way, and waits for it, so that
// its answer is not applied once Close has returned.
func TestDB_closeCancelsAnInflightRenewal(t *testing.T) {
	b := newBlockingRefresh()
	r := newRenewer(time.Minute, func(ctx context.Context, _ Tokens) (*Tokens, error) { return b.refresh(ctx) })
	db := &DB{con: closeOnlyConnection{}, renew: r}

	r.set(&Tokens{Access: jwtExpiringIn(time.Second), Refresh: "r"})
	b.waitStarted(t)

	require.NoError(t, db.Close(context.Background()))
	assert.True(t, b.exited.Load(), "Close returned while the renewal was under way")
	assert.Nil(t, r.current())
}

// detachConnection answers detach, and blocks refresh until it is canceled.
type detachConnection struct {
	connection.Connection
	refresh *blockingRefresh
}

func (c detachConnection) Call(ctx context.Context, req *connection.RPCRequest) (*connection.RPCResponse[cbor.RawMessage], error) {
	if req.Method == string(connection.Refresh) {
		_, _ = c.refresh.refresh(ctx)
		return nil, ctx.Err()
	}
	return &connection.RPCResponse[cbor.RawMessage]{}, nil
}

// Detaching a session cancels its renewal under way, and waits for it.
func TestSession_detachCancelsAnInflightRenewal(t *testing.T) {
	b := newBlockingRefresh()
	db := &DB{con: detachConnection{refresh: b}}
	s := &Session{db: db, id: &models.UUID{}}
	s.renew = newRenewer(time.Minute, func(ctx context.Context, tokens Tokens) (*Tokens, error) {
		return sessionCall[Tokens](ctx, s, connection.Refresh, tokens)
	})
	db.sessions = map[models.UUID]*Session{*s.id: s}

	s.renew.set(&Tokens{Access: jwtExpiringIn(time.Second), Refresh: "r"})
	b.waitStarted(t)

	done := make(chan error, 1)
	go func() { done <- s.Detach(context.Background()) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Detach did not return")
	}
	assert.True(t, b.exited.Load(), "Detach returned while the renewal was under way")
	assert.Nil(t, s.renew.current())
}
