package surrealdb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

// heldConnection holds a token pair, as a connection from contrib/rews does.
type heldConnection struct {
	connection.Connection
	tokens connection.Tokens
	err    error
}

func (c heldConnection) Tokens(context.Context) (connection.Tokens, error) {
	return c.tokens, c.err
}

// DB.Fork signs the fork in with the access token the connection holds, which
// a reconnection may have refreshed without the client hearing of it.
func TestDB_snapshotPrefersHeldToken(t *testing.T) {
	ctx := context.Background()
	expired, refreshed := jwtExpiringIn(-time.Minute), jwtExpiringIn(time.Hour)
	signedIn := func(con connection.Connection) *DB {
		return &DB{con: con, state: &sessionState{token: expired}}
	}

	held := heldConnection{tokens: connection.Tokens{Access: refreshed, Refresh: "refresh"}}
	assert.Equal(t, refreshed, signedIn(held).snapshot(ctx).token)

	held.tokens.Refresh = ""
	assert.Equal(t, refreshed, signedIn(held).snapshot(ctx).token, "a pair whose refresh token was revoked")

	assert.Equal(t, expired, signedIn(heldConnection{}).snapshot(ctx).token, "a connection which holds no token")
	assert.Equal(t, expired, signedIn(heldConnection{err: context.Canceled}).snapshot(ctx).token, "a failure to read the pair")
	assert.Equal(t, expired, signedIn(closeOnlyConnection{}).snapshot(ctx).token, "a connection which holds no tokens")

	signedOut := &DB{con: heldConnection{tokens: connection.Tokens{Access: refreshed}}, state: &sessionState{}}
	assert.Empty(t, signedOut.snapshot(ctx).token, "a session the client signed out")
}
