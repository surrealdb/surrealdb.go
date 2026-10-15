package surrealdb_test

import (
	"context"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

type streamedPerson struct {
	ID *models.RecordID `json:"id,omitempty"`
	N  int              `json:"n"`
}

// streamedStatement is what a statement's frames add up to.
type streamedStatement struct {
	rows   []streamedPerson
	done   int
	single bool
	value  streamedPerson
	err    error
}

// collectStream reads a whole stream, holding each statement to the frame
// contract: no frame follows a statement's Done frame.
func collectStream(t *testing.T, seq func(func(surrealdb.StreamFrame[streamedPerson], error) bool)) map[int]*streamedStatement {
	t.Helper()
	statements := map[int]*streamedStatement{}
	for frame, err := range seq {
		require.NoError(t, err)
		st := statements[frame.Statement]
		if st == nil {
			st = &streamedStatement{}
			statements[frame.Statement] = st
		}
		require.Zero(t, st.done, "statement %d has a frame after its Done frame", frame.Statement)
		st.rows = append(st.rows, frame.Rows...)
		if frame.Done {
			st.done++
			st.single = frame.Single
			st.value = frame.Value
			st.err = frame.Err
		}
	}
	return statements
}

func TestQueryStream(t *testing.T) {
	ctx := context.Background()
	db := testenv.MustNew("query_stream", "statements", "person")
	defer db.Close(ctx)

	statements := collectStream(t, surrealdb.QueryStream[streamedPerson](ctx, db, `
		CREATE person:a SET n = 1;
		CREATE person:b SET n = 2;
		SELECT * FROM person ORDER BY id;
		SELECT * FROM person WHERE false;
		SELECT * FROM ONLY person:a;
		THROW 'boom';
		SELECT * FROM person WHERE n = $n;
	`, map[string]any{"n": 2}))

	require.Len(t, statements, 7)
	for i, st := range statements {
		assert.Equal(t, 1, st.done, "statement %d is finished once", i)
	}

	a := streamedPerson{ID: &models.RecordID{Table: "person", ID: "a"}, N: 1}
	b := streamedPerson{ID: &models.RecordID{Table: "person", ID: "b"}, N: 2}

	assert.Equal(t, []streamedPerson{a}, statements[0].rows)
	assert.Equal(t, []streamedPerson{b}, statements[1].rows)
	assert.Equal(t, []streamedPerson{a, b}, statements[2].rows)

	assert.Empty(t, statements[3].rows, "an empty result has no rows")
	assert.False(t, statements[3].single)
	assert.NoError(t, statements[3].err)

	assert.True(t, statements[4].single, "SELECT FROM ONLY is a single value")
	assert.Empty(t, statements[4].rows)
	assert.Equal(t, a, statements[4].value)

	require.Error(t, statements[5].err)
	assert.Contains(t, statements[5].err.Error(), "boom")
	var qe *surrealdb.QueryError
	assert.ErrorAs(t, statements[5].err, &qe)

	assert.Equal(t, []streamedPerson{b}, statements[6].rows, "variables are bound")
}

func TestQueryStream_session(t *testing.T) {
	v := getVersion(t)
	if !v.IsV3OrLater() {
		t.Skipf("SurrealDB %s has no sessions", v)
	}
	ctx := context.Background()
	db := mustNewWS("query_stream", "session", "person")
	defer db.Close(ctx)

	session, err := db.Attach(ctx)
	require.NoError(t, err)
	defer func() { _ = session.Detach(ctx) }()
	_, err = session.SignIn(ctx, surrealdb.Auth{Username: "root", Password: "root"})
	require.NoError(t, err)
	require.NoError(t, session.Use(ctx, "query_stream", "session"))
	require.NoError(t, session.Let(ctx, "who", "session"))

	var got []any
	for frame, err := range surrealdb.QueryStream[any](ctx, session, "RETURN $who", nil) {
		require.NoError(t, err)
		if frame.Done {
			got = append(got, frame.Value)
		}
	}
	assert.Equal(t, []any{"session"}, got, "the query runs in the session")
}

func TestQueryStream_leavingEarly(t *testing.T) {
	ctx := context.Background()
	db := testenv.MustNew("query_stream", "leaving", "person")
	defer db.Close(ctx)

	for range 3 {
		frames := 0
		for _, err := range surrealdb.QueryStream[any](ctx, db, "RETURN 1; RETURN 2; RETURN 3; SLEEP 100ms; RETURN 4", nil) {
			require.NoError(t, err)
			frames++
			break
		}
		assert.Equal(t, 1, frames)

		// Frames still arriving for the abandoned stream go nowhere else.
		res, err := surrealdb.Query[int](ctx, db, "RETURN 42", nil)
		require.NoError(t, err)
		assert.Equal(t, 42, (*res)[0].Result)
	}
}

// streamRecorder records whether a connection was asked to stream, and whether
// it then ran the query with the query method.
type streamRecorder struct {
	*gorillaws.Connection
	streamed atomic.Bool
	queried  atomic.Bool
}

func (r *streamRecorder) CallStream(ctx context.Context, req *connection.RPCRequest) (*connection.ResponseStream, error) {
	r.streamed.Store(true)
	return r.Connection.CallStream(ctx, req)
}

func (r *streamRecorder) Send(ctx context.Context, method string, params ...any) (*connection.RPCResponse[cbor.RawMessage], error) {
	if method == "query" {
		r.queried.Store(true)
	}
	return r.Connection.Send(ctx, method, params...)
}

func TestQueryStream_fallsBackWhenServerCannotStream(t *testing.T) {
	v := getVersion(t)
	ctx := context.Background()

	u, err := url.ParseRequestURI(testenv.GetSurrealDBWSURL())
	require.NoError(t, err)
	rec := &streamRecorder{Connection: gorillaws.New(connection.NewConfig(u))}
	db, err := surrealdb.FromConnection(ctx, rec)
	require.NoError(t, err)
	defer db.Close(ctx)
	_, err = db.SignIn(ctx, surrealdb.Auth{Username: "root", Password: "root"})
	require.NoError(t, err)
	require.NoError(t, db.Use(ctx, "query_stream", "fallback"))
	rec.queried.Store(false)

	var values []any
	for frame, err := range surrealdb.QueryStream[any](ctx, db, "RETURN 1; RETURN [2, 3]", nil) {
		require.NoError(t, err)
		values = append(values, frame.Rows...)
		if frame.Single {
			values = append(values, frame.Value)
		}
	}
	assert.Equal(t, []any{uint64(1), uint64(2), uint64(3)}, values)

	assert.True(t, rec.streamed.Load(), "streaming is tried first")
	streams := v.Major > 3 || (v.Major == 3 && v.Minor >= 3)
	assert.Equal(t, !streams, rec.queried.Load(),
		"SurrealDB %s streams: %v; the query is re-sent only when it does not", v, streams)
}

func TestQueryStream_http(t *testing.T) {
	ctx := context.Background()
	db := testenv.MustNewHTTP("query_stream_http", "person")
	defer db.Close(ctx)

	var errs []string
	for frame, err := range surrealdb.QueryStream[any](ctx, db, "RETURN 'over http'; THROW 'nope'", nil) {
		require.NoError(t, err)
		if frame.Err != nil {
			errs = append(errs, frame.Err.Error())
			continue
		}
		assert.Equal(t, "over http", frame.Value)
	}
	require.Len(t, errs, 1)
	assert.True(t, strings.Contains(errs[0], "nope"), errs[0])
}

func TestQueryStream_afterClose(t *testing.T) {
	ctx := context.Background()
	// Closing an HTTP client leaves nothing to fail: each request opens its own.
	db := mustNewWS("query_stream", "closed", "person")
	require.NoError(t, db.Close(ctx))

	var got error
	for _, err := range surrealdb.QueryStream[any](ctx, db, "RETURN 1", nil) {
		got = err
	}
	require.Error(t, got, "a closed client yields an error rather than panicking")
}
