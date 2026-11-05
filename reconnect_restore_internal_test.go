package surrealdb

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/v2/internal/codec"
	"github.com/surrealdb/surrealdb.go/v2/pkg/connection"
	"github.com/surrealdb/surrealdb.go/v2/pkg/models"
	"github.com/surrealdb/surrealdb.go/v2/surrealcbor"
)

// restoreConnection answers the requests a restore sends, and records them.
// A method in fail is answered with that error instead.
type restoreConnection struct {
	connection.Connection
	codec *surrealcbor.Codec

	mu     sync.Mutex
	calls  []string
	fail   map[string]error
	closed chan struct{}
}

func newRestoreConnection() *restoreConnection {
	return &restoreConnection{codec: surrealcbor.New(), fail: map[string]error{}, closed: make(chan struct{})}
}

func (c *restoreConnection) Call(_ context.Context, req *connection.RPCRequest) (*connection.RPCResponse[cbor.RawMessage], error) {
	c.mu.Lock()
	c.calls = append(c.calls, fmt.Sprintf("%s %v", req.Method, req.Params))
	err := c.fail[req.Method]
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if req.Method != string(connection.Query) {
		return &connection.RPCResponse[cbor.RawMessage]{}, nil
	}
	id, err := c.codec.Marshal(models.UUID{UUID: uuid.Must(uuid.NewV4())})
	if err != nil {
		return nil, err
	}
	raw, err := c.codec.Marshal([]statementResult{{Status: "OK", Result: id}})
	if err != nil {
		return nil, err
	}
	res := cbor.RawMessage(raw)
	return &connection.RPCResponse[cbor.RawMessage]{Result: &res}, nil
}

func (c *restoreConnection) GetUnmarshaler() codec.Unmarshaler { return c.codec }

func (c *restoreConnection) LiveNotifications(string) (chan connection.Notification, error) {
	return make(chan connection.Notification, 1), nil
}

func (c *restoreConnection) Close(context.Context) error {
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
	return nil
}

func (c *restoreConnection) Closed() <-chan struct{} { return c.closed }
func (c *restoreConnection) CloseError() error       { return nil }

func (c *restoreConnection) sent() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.calls...)
}

// rootToken and aliceToken sign in as two different users.
var (
	rootToken  = jwt(`{"exp":4102444800,"ID":"root"}`)
	aliceToken = jwt(`{"exp":4102444800,"NS":"app","DB":"main","AC":"user","ID":"user:alice"}`)
)

// liveIn returns a router keeping one live query, started in the default
// session with the given state, and that query.
func liveIn(t *testing.T, started sessionSnapshot) (*liveRouter, *liveQuery) {
	t.Helper()
	l := &liveRouter{queries: map[string]*liveQuery{}}
	id := models.UUID{UUID: uuid.Must(uuid.NewV4())}
	l.track(id, nil, "item", false, started)
	return l, l.queries[id.String()]
}

// A live query is started again in the database it was started in, and the
// session's own selection is restored after it.
func TestLiveRouter_restartsInItsOwnDatabase(t *testing.T) {
	l, q := liveIn(t, sessionSnapshot{namespace: "app", database: "first", token: rootToken})
	con := newRestoreConnection()
	before := q.serverID

	errs := l.restore(context.Background(), con, map[string]restoredSession{
		"": {namespace: "app", database: "second", who: identityOf(rootToken)},
	})

	require.Empty(t, errs)
	assert.Equal(t, []string{
		"use [app first]",
		"query [LIVE SELECT * FROM $what map[what:item]]",
		"use [app second]",
	}, con.sent())
	assert.NotEqual(t, before, q.serverID, "the query runs under the ID the server gave it again")
}

func TestLiveRouter_restartsInPlace(t *testing.T) {
	l, _ := liveIn(t, sessionSnapshot{namespace: "app", database: "first", token: rootToken})
	con := newRestoreConnection()

	errs := l.restore(context.Background(), con, map[string]restoredSession{
		"": {namespace: "app", database: "first", who: identityOf(rootToken)},
	})

	require.Empty(t, errs)
	assert.Equal(t, []string{"query [LIVE SELECT * FROM $what map[what:item]]"}, con.sent())
}

// A selection the session no longer has is cleared again with NULL.
func TestLiveRouter_clearsWhatTheSessionHasNotSelected(t *testing.T) {
	l, _ := liveIn(t, sessionSnapshot{namespace: "app", database: "first"})
	con := newRestoreConnection()

	errs := l.restore(context.Background(), con, map[string]restoredSession{"": {}})

	require.Empty(t, errs)
	assert.Equal(t, "use [<nil> <nil>]", con.sent()[2])
}

// A renewed token signs in as the same user, so the query is started again.
func TestLiveRouter_restartsAfterARenewal(t *testing.T) {
	l, _ := liveIn(t, sessionSnapshot{namespace: "app", database: "main", token: aliceToken})
	renewed := jwt(`{"exp":4102448400,"iat":4102444800,"NS":"app","DB":"main","AC":"user","ID":"user:alice"}`)
	con := newRestoreConnection()

	errs := l.restore(context.Background(), con, map[string]restoredSession{
		"": {namespace: "app", database: "main", who: identityOf(renewed)},
	})

	require.Empty(t, errs)
	assert.Len(t, con.sent(), 1)
}

// A query whose session is now signed in as someone else, or not at all, is
// ended rather than started again as them.
func TestLiveRouter_endsAQueryOfAnotherUser(t *testing.T) {
	for _, tc := range []struct {
		name     string
		sessions map[string]restoredSession
	}{
		{"another user", map[string]restoredSession{"": {namespace: "app", database: "main", who: identityOf(aliceToken)}}},
		{"no user", map[string]restoredSession{"": {namespace: "app", database: "main"}}},
		{"session not attached again", map[string]restoredSession{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l, q := liveIn(t, sessionSnapshot{namespace: "app", database: "main", token: rootToken})
			ch, ok, err := l.notifications(q.id.String(), newRestoreConnection())
			require.True(t, ok)
			require.NoError(t, err)
			con := newRestoreConnection()
			sessions := tc.sessions

			errs := l.restore(context.Background(), con, sessions)

			require.Len(t, errs, 1)
			assert.ErrorIs(t, errs[0], ErrLiveQueryEnded)
			assert.Empty(t, con.sent(), "nothing is started")
			_, open := <-ch
			assert.False(t, open, "the channel is closed")

			_, ok, err = l.notifications(q.id.String(), con)
			assert.True(t, ok)
			assert.ErrorIs(t, err, ErrLiveQueryEnded)
			_, ended := l.serverID(q.id.String())
			assert.True(t, ended, "Kill sends nothing for it")

			assert.Empty(t, l.restore(context.Background(), con, sessions), "it is not tried again")
			ok, err = l.close(q.id.String(), con)
			assert.True(t, ok)
			assert.NoError(t, err)
		})
	}
}

// A query the server refuses to start again is ended, rather than kept with a
// channel which never hears of it again.
func TestLiveRouter_endsAQueryTheServerRefuses(t *testing.T) {
	l, q := liveIn(t, sessionSnapshot{namespace: "app", database: "main", token: rootToken})
	ch, _, err := l.notifications(q.id.String(), newRestoreConnection())
	require.NoError(t, err)
	con := newRestoreConnection()
	con.fail[string(connection.Query)] = connection.NewStatementError("Anonymous access not allowed", "NotAllowed", nil)

	errs := l.restore(context.Background(), con, map[string]restoredSession{
		"": {namespace: "app", database: "main", who: identityOf(rootToken)},
	})

	require.Len(t, errs, 1)
	assert.ErrorIs(t, errs[0], ErrLiveQueryEnded)
	assert.ErrorContains(t, errs[0], "Anonymous access not allowed")
	_, open := <-ch
	assert.False(t, open)
}

// A query which fails because the connection was lost is kept, for the next
// attempt to reconnect to start again.
func TestLiveRouter_keepsAQueryWhenTheConnectionIsLost(t *testing.T) {
	l, q := liveIn(t, sessionSnapshot{namespace: "app", database: "main", token: rootToken})
	con := newRestoreConnection()
	con.fail[string(connection.Query)] = connection.ErrConnectionClosed
	_ = con.Close(context.Background())

	errs := l.restore(context.Background(), con, map[string]restoredSession{
		"": {namespace: "app", database: "main", who: identityOf(rootToken)},
	})

	require.Len(t, errs, 1)
	assert.NotErrorIs(t, errs[0], ErrLiveQueryEnded)
	assert.Nil(t, q.ended)
}

// When the session's own selection cannot be set again after the query, the
// connection is closed, so that the client restores again rather than run the
// caller's requests in the query's database.
func TestLiveRouter_closesTheConnectionWhenTheSelectionIsNotRestored(t *testing.T) {
	l, q := liveIn(t, sessionSnapshot{namespace: "app", database: "first", token: rootToken})
	con := &failingUseBack{restoreConnection: newRestoreConnection()}

	errs := l.restore(context.Background(), con, map[string]restoredSession{
		"": {namespace: "app", database: "second", who: identityOf(rootToken)},
	})

	require.Len(t, errs, 1)
	assert.NotErrorIs(t, errs[0], ErrLiveQueryEnded)
	assert.Nil(t, q.ended)
	select {
	case <-con.Closed():
	default:
		t.Fatal("the connection is still open")
	}
}

// failingUseBack refuses the second use request: the one which selects the
// session's own namespace and database again.
type failingUseBack struct {
	*restoreConnection
	uses int
}

func (c *failingUseBack) Call(ctx context.Context, req *connection.RPCRequest) (*connection.RPCResponse[cbor.RawMessage], error) {
	if req.Method == string(connection.Use) {
		c.uses++
		if c.uses == 2 {
			return nil, connection.NewStatementError("refused", "NotAllowed", nil)
		}
	}
	return c.restoreConnection.Call(ctx, req)
}

func TestIdentityOf(t *testing.T) {
	assert.Equal(t, identity{}, identityOf(""), "no token")
	assert.Equal(t, identityOf(aliceToken),
		identityOf(jwt(`{"exp":1,"iat":0,"ID":"user:alice","AC":"user","DB":"main","NS":"app"}`)),
		"other claims and the order of the claims do not count")
	assert.NotEqual(t, identityOf(aliceToken), identityOf(jwt(`{"NS":"app","DB":"main","AC":"user","ID":"user:bob"}`)))
	assert.NotEqual(t, identityOf(aliceToken), identityOf(jwt(`{"NS":"app","DB":"other","AC":"user","ID":"user:alice"}`)))
	assert.Equal(t, identity{signedIn: true}, identityOf("not a jwt"), "a token whose claims cannot be read")
}
