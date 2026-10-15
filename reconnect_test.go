package surrealdb_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gws"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// dropProxy forwards TCP connections to the server, and can cut them, or turn
// new ones away, to stand in for a network which fails.
type dropProxy struct {
	ln     net.Listener
	target string
	refuse atomic.Bool

	mu    sync.Mutex
	conns []net.Conn
}

func newDropProxy(t *testing.T) *dropProxy {
	t.Helper()
	u, err := url.Parse(testenv.GetSurrealDBWSURL())
	require.NoError(t, err)

	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	p := &dropProxy{ln: ln, target: u.Host}
	t.Cleanup(func() {
		_ = ln.Close()
		p.drop()
	})
	go p.serve()
	return p
}

// url is the WebSocket URL of the server through the proxy.
func (p *dropProxy) url() string {
	return "ws://" + p.ln.Addr().String() + "/rpc"
}

func (p *dropProxy) serve() {
	for {
		in, err := p.ln.Accept()
		if err != nil {
			return
		}
		if p.refuse.Load() {
			_ = in.Close()
			continue
		}
		out, err := (&net.Dialer{}).DialContext(context.Background(), "tcp", p.target)
		if err != nil {
			_ = in.Close()
			continue
		}
		p.mu.Lock()
		p.conns = append(p.conns, in, out)
		p.mu.Unlock()
		go func() { _, _ = io.Copy(out, in); _ = out.Close() }()
		go func() { _, _ = io.Copy(in, out); _ = in.Close() }()
	}
}

// drop cuts every connection through the proxy.
func (p *dropProxy) drop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.conns {
		_ = c.Close()
	}
	p.conns = nil
}

// statusLog records the status events of a client.
type statusLog struct {
	mu     sync.Mutex
	events []surrealdb.StatusEvent
	ch     chan surrealdb.StatusEvent
}

func watchStatus(db *surrealdb.DB) *statusLog {
	l := &statusLog{ch: make(chan surrealdb.StatusEvent, 64)}
	db.OnStatus(func(ev surrealdb.StatusEvent) {
		l.mu.Lock()
		l.events = append(l.events, ev)
		l.mu.Unlock()
		l.ch <- ev
	})
	return l
}

// waitFor waits for an event with the given status.
func (l *statusLog) waitFor(t *testing.T, status surrealdb.ConnectionStatus) surrealdb.StatusEvent {
	t.Helper()
	timeout := time.After(20 * time.Second)
	for {
		select {
		case ev := <-l.ch:
			if ev.Status == status {
				return ev
			}
		case <-timeout:
			t.Fatalf("no %v event; got %v", status, l.statuses())
		}
	}
}

func (l *statusLog) statuses() []surrealdb.ConnectionStatus {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []surrealdb.ConnectionStatus
	for _, ev := range l.events {
		out = append(out, ev.Status)
	}
	return out
}

func connectThrough(t *testing.T, p *dropProxy, opts ...surrealdb.Option) *surrealdb.DB {
	t.Helper()
	ctx := context.Background()
	opts = append([]surrealdb.Option{surrealdb.WithReconnectDelay(20*time.Millisecond, 200*time.Millisecond)}, opts...)
	db, err := surrealdb.FromEndpointURLStringWithOptions(ctx, p.url(), opts...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close(context.Background()) })
	return db
}

// signInRoot signs in as root, and uses a database of the reconnect namespace.
func signInRoot(t *testing.T, db *surrealdb.DB, database string) {
	t.Helper()
	const ns = "reconnect"
	ctx := context.Background()
	_, err := db.SignIn(ctx, surrealdb.Auth{Username: "root", Password: "root"})
	require.NoError(t, err)
	_, err = surrealdb.Query[any](ctx, db, "DEFINE NAMESPACE IF NOT EXISTS "+ns+"; USE NS "+ns+"; DEFINE DATABASE IF NOT EXISTS "+database, nil)
	require.NoError(t, err)
	require.NoError(t, db.Use(ctx, ns, database))
}

func TestReconnect_restoresTheDefaultSession(t *testing.T) {
	ctx := context.Background()
	p := newDropProxy(t)
	db := connectThrough(t, p)
	status := watchStatus(db)

	signInRoot(t, db, "default_session")
	require.NoError(t, db.Let(ctx, "greeting", "hello"))
	require.NoError(t, db.Let(ctx, "dropped", "soon"))
	require.NoError(t, db.Unset(ctx, "dropped"))

	p.drop()
	status.waitFor(t, surrealdb.StatusDisconnected)
	ev := status.waitFor(t, surrealdb.StatusConnected)
	require.NoError(t, ev.Err, "everything is restored")
	assert.Equal(t, surrealdb.StatusConnected, db.Status())

	res, err := surrealdb.Query[[]any](ctx, db, "RETURN [session::ns(), session::db(), $greeting, $dropped]", nil)
	require.NoError(t, err)
	assert.Equal(t, []any{"reconnect", "default_session", "hello", nil}, (*res)[0].Result)

	// Still signed in as root: only root may define a namespace.
	_, err = surrealdb.Query[any](ctx, db, "DEFINE NAMESPACE IF NOT EXISTS reconnect_root_check", nil)
	require.NoError(t, err)

	assert.Equal(t, []surrealdb.ConnectionStatus{
		surrealdb.StatusDisconnected, surrealdb.StatusReconnecting, surrealdb.StatusConnected,
	}, status.statuses())
}

func TestReconnect_gws(t *testing.T) {
	ctx := context.Background()
	p := newDropProxy(t)
	u, err := url.ParseRequestURI(p.url())
	require.NoError(t, err)
	db, err := surrealdb.FromConnectionWithOptions(ctx, gws.New(connection.NewConfig(u)),
		surrealdb.WithReconnectDelay(20*time.Millisecond, 200*time.Millisecond))
	require.NoError(t, err)
	defer db.Close(ctx)
	status := watchStatus(db)

	signInRoot(t, db, "gws")
	require.NoError(t, db.Let(ctx, "engine", "gws"))

	p.drop()
	require.NoError(t, status.waitFor(t, surrealdb.StatusConnected).Err)

	res, err := surrealdb.Query[string](ctx, db, "RETURN $engine", nil)
	require.NoError(t, err)
	assert.Equal(t, "gws", (*res)[0].Result)
}

func TestReconnect_restoresAttachedSessions(t *testing.T) {
	if v := getVersion(t); !v.IsV3OrLater() {
		t.Skipf("SurrealDB %s has no sessions", v)
	}
	ctx := context.Background()
	p := newDropProxy(t)
	db := connectThrough(t, p)
	status := watchStatus(db)
	signInRoot(t, db, "sessions")

	session, err := db.Attach(ctx)
	require.NoError(t, err)
	_, err = session.SignIn(ctx, surrealdb.Auth{Username: "root", Password: "root"})
	require.NoError(t, err)
	require.NoError(t, session.Use(ctx, "reconnect", "attached"))
	require.NoError(t, session.Let(ctx, "who", "attached"))

	p.drop()
	require.NoError(t, status.waitFor(t, surrealdb.StatusConnected).Err)

	res, err := surrealdb.Query[[]any](ctx, session, "RETURN [session::db(), $who]", nil)
	require.NoError(t, err)
	assert.Equal(t, []any{"attached", "attached"}, (*res)[0].Result)

	listed, err := db.Sessions(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Same(t, session, listed[0], "the session is the same object, under the same ID")
}

func TestReconnect_restoresLiveQueries(t *testing.T) {
	ctx := context.Background()
	p := newDropProxy(t)
	db := connectThrough(t, p)
	status := watchStatus(db)
	signInRoot(t, db, "live")
	_, err := surrealdb.Query[any](ctx, db, "REMOVE TABLE IF EXISTS item; DEFINE TABLE item SCHEMALESS", nil)
	require.NoError(t, err)

	id, err := surrealdb.Live(ctx, db, "item", false)
	require.NoError(t, err)
	notifications, err := db.LiveNotifications(id.String())
	require.NoError(t, err)

	next := func() connection.Notification {
		t.Helper()
		select {
		case n, ok := <-notifications:
			require.True(t, ok, "the channel stays open")
			return n
		case <-time.After(10 * time.Second):
			t.Fatal("no notification")
		}
		return connection.Notification{}
	}

	_, err = surrealdb.Query[any](ctx, db, "CREATE item:before", nil)
	require.NoError(t, err)
	n := next()
	assert.Equal(t, *id, *n.ID)

	p.drop()
	require.NoError(t, status.waitFor(t, surrealdb.StatusConnected).Err)

	_, err = surrealdb.Query[any](ctx, db, "CREATE item:after", nil)
	require.NoError(t, err)
	n = next()
	assert.Equal(t, *id, *n.ID, "notifications keep the ID Live returned")
	assert.Equal(t, connection.CreateAction, n.Action)
	record, ok := n.Result.(map[string]any)
	require.True(t, ok, "%T", n.Result)
	assert.Equal(t, models.RecordID{Table: "item", ID: "after"}, record["id"])

	require.NoError(t, surrealdb.Kill(ctx, db, id.String()), "the query is killed by the ID Live returned")
	assertClosed(t, notifications, "Kill closes the channel")
}

// assertClosed waits for a notification channel to close, past the KILLED
// notification the server may send first.
func assertClosed(t *testing.T, ch <-chan connection.Notification, msg string) {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case n, open := <-ch:
			if !open {
				return
			}
			assert.Equal(t, connection.KilledAction, n.Action, msg)
		case <-timeout:
			t.Fatal(msg)
		}
	}
}

func TestReconnect_failsRequestsInFlight(t *testing.T) {
	ctx := context.Background()
	p := newDropProxy(t)
	db := connectThrough(t, p)
	status := watchStatus(db)
	signInRoot(t, db, "in_flight")

	errc := make(chan error, 1)
	go func() {
		_, err := surrealdb.Query[any](ctx, db, "SLEEP 3s", nil)
		errc <- err
	}()
	time.Sleep(300 * time.Millisecond)
	p.drop()

	select {
	case err := <-errc:
		require.Error(t, err)
		assert.ErrorIs(t, err, connection.ErrConnectionClosed)
	case <-time.After(2 * time.Second):
		t.Fatal("the request in flight did not fail when the connection was lost")
	}

	require.NoError(t, status.waitFor(t, surrealdb.StatusConnected).Err)
	_, err := surrealdb.Query[any](ctx, db, "RETURN 1", nil)
	require.NoError(t, err)
}

func TestReconnect_requestsWaitWhileDisconnected(t *testing.T) {
	ctx := context.Background()
	p := newDropProxy(t)
	db := connectThrough(t, p)
	status := watchStatus(db)
	signInRoot(t, db, "waiting")

	p.refuse.Store(true)
	p.drop()
	status.waitFor(t, surrealdb.StatusDisconnected)

	short, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	_, err := surrealdb.Query[any](short, db, "RETURN 1", nil)
	require.ErrorIs(t, err, context.DeadlineExceeded, "a request waits no longer than its context")

	resc := make(chan error, 1)
	go func() {
		res, err := surrealdb.Query[int](ctx, db, "RETURN 2", nil)
		if err == nil && (*res)[0].Result != 2 {
			err = errors.New("wrong result")
		}
		resc <- err
	}()

	time.Sleep(300 * time.Millisecond)
	p.refuse.Store(false)

	select {
	case err := <-resc:
		require.NoError(t, err)
	case <-time.After(20 * time.Second):
		t.Fatal("the waiting request did not run after reconnecting")
	}

	status.mu.Lock()
	events := append([]surrealdb.StatusEvent(nil), status.events...)
	status.mu.Unlock()
	failed := 0
	for _, ev := range events {
		if ev.Status == surrealdb.StatusDisconnected && ev.Attempt > 0 {
			failed++
			assert.Error(t, ev.Err)
		}
	}
	assert.Positive(t, failed, "attempts failed while the proxy refused connections")
}

func TestReconnect_givesUp(t *testing.T) {
	ctx := context.Background()
	p := newDropProxy(t)
	db := connectThrough(t, p, surrealdb.WithReconnectAttempts(2))
	status := watchStatus(db)
	signInRoot(t, db, "giving_up")

	p.refuse.Store(true)
	p.drop()
	closed := status.waitFor(t, surrealdb.StatusClosed)
	require.Error(t, closed.Err)
	assert.Equal(t, 2, closed.Attempt)

	assert.Equal(t, []surrealdb.ConnectionStatus{
		surrealdb.StatusDisconnected,
		surrealdb.StatusReconnecting, surrealdb.StatusDisconnected,
		surrealdb.StatusReconnecting, surrealdb.StatusDisconnected,
		surrealdb.StatusClosed,
	}, status.statuses())

	_, err := surrealdb.Query[any](ctx, db, "RETURN 1", nil)
	require.ErrorIs(t, err, surrealdb.ErrClientClosed)
	assert.Equal(t, surrealdb.StatusClosed, db.Status())
}

func TestReconnect_off(t *testing.T) {
	ctx := context.Background()
	p := newDropProxy(t)
	db := connectThrough(t, p, surrealdb.WithReconnect(false))
	signInRoot(t, db, "off")

	p.drop()
	require.Eventually(t, func() bool { return db.Status() == surrealdb.StatusClosed }, 5*time.Second, 10*time.Millisecond)
	_, err := surrealdb.Query[any](ctx, db, "RETURN 1", nil)
	require.ErrorIs(t, err, connection.ErrConnectionClosed)
}

func TestReconnect_closeStopsReconnecting(t *testing.T) {
	ctx := context.Background()
	p := newDropProxy(t)
	db := connectThrough(t, p)
	status := watchStatus(db)

	p.refuse.Store(true)
	p.drop()
	status.waitFor(t, surrealdb.StatusReconnecting)
	require.NoError(t, db.Close(ctx))
	status.waitFor(t, surrealdb.StatusClosed)

	_, err := surrealdb.Query[any](ctx, db, "RETURN 1", nil)
	require.ErrorIs(t, err, surrealdb.ErrClientClosed)
}

func TestReconnect_renewsAnExpiredToken(t *testing.T) {
	skipBeforeV3(t)
	ctx := context.Background()
	defineRefreshAccess(t, "2s")

	p := newDropProxy(t)
	db := connectThrough(t, p)
	status := watchStatus(db)
	require.NoError(t, db.Use(ctx, refreshNS, refreshDB))
	user := uniq()
	signUpWithRefresh(t, db, user)
	before := db.Tokens()

	// Stay disconnected until the access token has expired.
	p.refuse.Store(true)
	p.drop()
	status.waitFor(t, surrealdb.StatusDisconnected)
	time.Sleep(3 * time.Second)
	p.refuse.Store(false)

	require.NoError(t, status.waitFor(t, surrealdb.StatusConnected).Err)
	assert.NotEqual(t, before.Access, db.Tokens().Access, "the expired pair was renewed")
	assert.Equal(t, models.RecordID{Table: "user", ID: user}, whoAmI(t, db))
}

func TestReconnect_detachWhileDisconnected(t *testing.T) {
	if v := getVersion(t); !v.IsV3OrLater() {
		t.Skipf("SurrealDB %s has no sessions", v)
	}
	ctx := context.Background()
	p := newDropProxy(t)
	db := connectThrough(t, p)
	status := watchStatus(db)
	signInRoot(t, db, "detach")
	_, err := surrealdb.Query[any](ctx, db, "DEFINE TABLE IF NOT EXISTS thing SCHEMALESS", nil)
	require.NoError(t, err)

	session, err := db.Attach(ctx)
	require.NoError(t, err)
	_, err = session.SignIn(ctx, surrealdb.Auth{Username: "root", Password: "root"})
	require.NoError(t, err)
	require.NoError(t, session.Use(ctx, "reconnect", "detach"))
	id, err := surrealdb.Live(ctx, session, "thing", false)
	require.NoError(t, err)
	notifications, err := session.LiveNotifications(id.String())
	require.NoError(t, err)

	p.refuse.Store(true)
	p.drop()
	status.waitFor(t, surrealdb.StatusDisconnected)

	// Detach waits for the connection, holding the session; reconnecting must
	// not wait for Detach in turn.
	detached := make(chan error, 1)
	go func() { detached <- session.Detach(ctx) }()
	time.Sleep(100 * time.Millisecond)
	p.refuse.Store(false)

	select {
	case err := <-detached:
		require.NoError(t, err)
	case <-time.After(20 * time.Second):
		t.Fatal("Detach and reconnecting wait for each other")
	}
	assertClosed(t, notifications, "detaching closes the channels of the session's live queries")

	// The detached session and its live query are not restored again.
	p.drop()
	status.waitFor(t, surrealdb.StatusDisconnected)
	require.NoError(t, status.waitFor(t, surrealdb.StatusConnected).Err)
}

// Kill made while disconnected waits for the new connection, on which the
// live query was started again under a new server ID, and must kill it by that
// ID rather than the one of the lost connection.
func TestReconnect_killWhileDisconnected(t *testing.T) {
	ctx := context.Background()
	p := newDropProxy(t)
	db := connectThrough(t, p)
	status := watchStatus(db)
	signInRoot(t, db, "kill_disconnected")
	_, err := surrealdb.Query[any](ctx, db, "REMOVE TABLE IF EXISTS item; DEFINE TABLE item SCHEMALESS", nil)
	require.NoError(t, err)

	id, err := surrealdb.Live(ctx, db, "item", false)
	require.NoError(t, err)
	notifications, err := db.LiveNotifications(id.String())
	require.NoError(t, err)

	p.refuse.Store(true)
	p.drop()
	status.waitFor(t, surrealdb.StatusDisconnected)

	killed := make(chan error, 1)
	go func() { killed <- surrealdb.Kill(ctx, db, id.String()) }()
	time.Sleep(100 * time.Millisecond)
	p.refuse.Store(false)

	select {
	case err := <-killed:
		require.NoError(t, err, "the restored live query is killed by its new server ID")
	case <-time.After(20 * time.Second):
		t.Fatal("Kill did not return after reconnecting")
	}
	assertClosed(t, notifications, "Kill closes the channel")

	// The killed query is not restored again.
	p.drop()
	status.waitFor(t, surrealdb.StatusDisconnected)
	require.NoError(t, status.waitFor(t, surrealdb.StatusConnected).Err)
}

// Reset kills the live queries of the session on the server, so the client
// must stop restarting them. Were they kept, the restore would fail: the
// session has no namespace or database after Reset.
func TestReconnect_resetForgetsLiveQueries(t *testing.T) {
	ctx := context.Background()
	p := newDropProxy(t)
	db := connectThrough(t, p)
	status := watchStatus(db)
	signInRoot(t, db, "reset")
	_, err := surrealdb.Query[any](ctx, db, "REMOVE TABLE IF EXISTS item; DEFINE TABLE item SCHEMALESS", nil)
	require.NoError(t, err)

	id, err := surrealdb.Live(ctx, db, "item", false)
	require.NoError(t, err)
	notifications, err := db.LiveNotifications(id.String())
	require.NoError(t, err)

	require.NoError(t, db.Reset(ctx))
	assertClosed(t, notifications, "Reset closes the channels of the session's live queries")

	p.drop()
	status.waitFor(t, surrealdb.StatusDisconnected)
	require.NoError(t, status.waitFor(t, surrealdb.StatusConnected).Err, "nothing is left to restore")
}

func TestReconnect_sessionResetForgetsLiveQueries(t *testing.T) {
	if v := getVersion(t); !v.IsV3OrLater() {
		t.Skipf("SurrealDB %s has no sessions", v)
	}
	ctx := context.Background()
	p := newDropProxy(t)
	db := connectThrough(t, p)
	status := watchStatus(db)
	signInRoot(t, db, "session_reset")
	_, err := surrealdb.Query[any](ctx, db, "REMOVE TABLE IF EXISTS item; DEFINE TABLE item SCHEMALESS", nil)
	require.NoError(t, err)

	// A live query of the default session, which the session's Reset leaves.
	kept, err := surrealdb.Live(ctx, db, "item", false)
	require.NoError(t, err)
	keptNotifications, err := db.LiveNotifications(kept.String())
	require.NoError(t, err)

	session, err := db.Attach(ctx)
	require.NoError(t, err)
	_, err = session.SignIn(ctx, surrealdb.Auth{Username: "root", Password: "root"})
	require.NoError(t, err)
	require.NoError(t, session.Use(ctx, "reconnect", "session_reset"))
	id, err := surrealdb.Live(ctx, session, "item", false)
	require.NoError(t, err)
	notifications, err := session.LiveNotifications(id.String())
	require.NoError(t, err)

	require.NoError(t, session.Reset(ctx))
	assertClosed(t, notifications, "Reset closes the channels of the session's live queries")

	p.drop()
	status.waitFor(t, surrealdb.StatusDisconnected)
	require.NoError(t, status.waitFor(t, surrealdb.StatusConnected).Err, "only the default session's live query is restored")

	_, err = surrealdb.Query[any](ctx, db, "CREATE item:after_reset", nil)
	require.NoError(t, err)
	select {
	case n, ok := <-keptNotifications:
		require.True(t, ok, "the default session's live query is kept")
		assert.Equal(t, *kept, *n.ID)
	case <-time.After(10 * time.Second):
		t.Fatal("no notification for the default session's live query")
	}
}

// A status listener which closes the client used to wait for the goroutine
// which reconnects, which waited for the listener to return.
func TestReconnect_closeFromOnStatus(t *testing.T) {
	p := newDropProxy(t)
	db := connectThrough(t, p)
	status := watchStatus(db)

	closed := make(chan error, 1)
	var once sync.Once
	db.OnStatus(func(ev surrealdb.StatusEvent) {
		if ev.Status == surrealdb.StatusDisconnected {
			once.Do(func() { closed <- db.Close(context.Background()) })
		}
	})

	p.refuse.Store(true)
	p.drop()
	select {
	case err := <-closed:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("Close called from a status listener did not return")
	}
	status.waitFor(t, surrealdb.StatusClosed)
	assert.Equal(t, surrealdb.StatusClosed, db.Status())
	statuses := status.statuses()
	assert.Equal(t, surrealdb.StatusDisconnected, statuses[0], "events keep their order: %v", statuses)
	assert.Equal(t, surrealdb.StatusClosed, statuses[len(statuses)-1], "events keep their order: %v", statuses)
}
