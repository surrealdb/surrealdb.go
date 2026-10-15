package surrealdb_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func newLiveTestDB(t *testing.T, table string) *surrealdb.DB {
	t.Helper()
	if strings.HasPrefix(testenv.GetSurrealDBURL(), "http") {
		t.Skip("live queries need a WebSocket connection")
	}
	db, err := testenv.New("live_reliability", "live_reliability", table)
	require.NoError(t, err)
	require.NoError(t, testenv.DefineSchemalessTables(db, table))
	t.Cleanup(func() { _ = db.Close(context.Background()) })
	return db
}

func startLive(t *testing.T, db *surrealdb.DB, table string) string {
	t.Helper()
	// The table is one of this file's constants. type::table() can't be used:
	// SurrealDB 2.x does not accept it in LIVE SELECT.
	res, err := surrealdb.Query[models.UUID](context.Background(), db, "LIVE SELECT * FROM "+table, nil)
	require.NoError(t, err)
	return (*res)[0].Result.String()
}

func nextNotification(t *testing.T, ch chan connection.Notification) (connection.Notification, bool) {
	t.Helper()
	select {
	case n, ok := <-ch:
		return n, ok
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for a live notification")
		return connection.Notification{}, false
	}
}

// TestLive_burstWithoutReading creates many records before the consumer reads
// a single notification. Notifications beyond the channel's buffer used to be dropped.
func TestLive_burstWithoutReading(t *testing.T) {
	ctx := context.Background()
	db := newLiveTestDB(t, "burst")

	id := startLive(t, db, "burst")
	ch, err := db.LiveNotifications(id)
	require.NoError(t, err)
	defer func() { _ = surrealdb.Kill(ctx, db, id) }()

	// More than the channel's buffer of 16, which used to be all that was kept,
	// but fewer than the 100 notifications SurrealDB 3.3 queues per socket: past
	// that it closes the connection, however fast the client reads.
	const count = 60
	rows := make([]map[string]any, count)
	for i := range rows {
		rows[i] = map[string]any{"i": i}
	}
	_, err = surrealdb.Query[any](ctx, db, "INSERT INTO burst $rows", map[string]any{"rows": rows})
	require.NoError(t, err)
	// Let every notification arrive before the first is read.
	time.Sleep(300 * time.Millisecond)

	for i := 0; i < count; i++ {
		n, ok := nextNotification(t, ch)
		require.True(t, ok)
		require.Equal(t, connection.CreateAction, n.Action)
	}
}

// TestLive_notificationsBeforeSubscribing changes records between starting the
// live query and asking for its notifications. Those notifications used to be dropped.
func TestLive_notificationsBeforeSubscribing(t *testing.T) {
	ctx := context.Background()
	db := newLiveTestDB(t, "early")

	id := startLive(t, db, "early")
	defer func() { _ = surrealdb.Kill(ctx, db, id) }()

	_, err := surrealdb.Query[any](ctx, db, "CREATE early:1 SET n = 1; CREATE early:2 SET n = 2", nil)
	require.NoError(t, err)
	// Let the notifications arrive before anyone has subscribed.
	time.Sleep(200 * time.Millisecond)

	ch, err := db.LiveNotifications(id)
	require.NoError(t, err)

	for i := 0; i < 2; i++ {
		n, ok := nextNotification(t, ch)
		require.True(t, ok)
		assert.Equal(t, connection.CreateAction, n.Action)
	}
}

// TestLive_killEndsSubscription kills a live query with a KILL statement, which
// sends KILLED. The channel must be closed afterwards, and Kill must still succeed
// on a live query whose KILLED notification already ended it.
func TestLive_killEndsSubscription(t *testing.T) {
	ctx := context.Background()
	db := newLiveTestDB(t, "killed")

	id := startLive(t, db, "killed")
	ch, err := db.LiveNotifications(id)
	require.NoError(t, err)

	_, err = surrealdb.Query[any](ctx, db, "CREATE killed:1", nil)
	require.NoError(t, err)
	n, ok := nextNotification(t, ch)
	require.True(t, ok)
	require.Equal(t, connection.CreateAction, n.Action)

	uid, err := uuid.FromString(id)
	require.NoError(t, err)
	_, err = surrealdb.Query[any](ctx, db, "KILL $id", map[string]any{"id": models.UUID{UUID: uid}})
	require.NoError(t, err)

	// Either the server sends KILLED, which is delivered and closes the channel,
	// or it does not, and the channel stays open until it is closed.
	select {
	case n, ok := <-ch:
		if ok {
			require.Equal(t, connection.KilledAction, n.Action)
			_, ok = nextNotification(t, ch)
			require.False(t, ok, "the channel is closed after KILLED")
		}
	case <-time.After(2 * time.Second):
		t.Log("the server sent no KILLED notification")
	}

	require.NoError(t, db.CloseLiveNotifications(id))
}

// TestLive_closeEndsSubscription closes the connection while a live query's
// channel is open. The channel must be closed, rather than left open for ever
// with a goroutine waiting on it.
func TestLive_closeEndsSubscription(t *testing.T) {
	db := newLiveTestDB(t, "closing")

	id := startLive(t, db, "closing")
	ch, err := db.LiveNotifications(id)
	require.NoError(t, err)

	require.NoError(t, db.Close(context.Background()))

	_, ok := nextNotification(t, ch)
	require.False(t, ok, "the channel is closed with the connection")
	// Kill's own CloseLiveNotifications must not fail afterwards.
	require.NoError(t, db.CloseLiveNotifications(id))
}
