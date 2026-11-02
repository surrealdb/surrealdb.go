package surrealdb_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	nethttp "net/http"
	"reflect"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/http"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

type roundTripFunc func(*nethttp.Request) *nethttp.Response

func (f roundTripFunc) RoundTrip(req *nethttp.Request) (*nethttp.Response, error) { return f(req), nil }

// recordingDB returns a client whose requests are recorded rather than sent,
// and answered with the result of one successful statement.
func recordingDB(t *testing.T) (*surrealdb.DB, *[]connection.RPCRequest) {
	t.Helper()
	return stubDB(t, surrealcbor.New(), map[string]any{"status": "OK", "result": nil})
}

// stubDB returns a client that decodes responses with unmarshaler, whose
// requests are recorded rather than sent, and answered with the one statement
// result given.
func stubDB(t *testing.T, unmarshaler codec.Unmarshaler, statement map[string]any) (*surrealdb.DB, *[]connection.RPCRequest) {
	t.Helper()
	c := surrealcbor.New()
	var sent []connection.RPCRequest
	answer := false

	con := http.New(&connection.Config{BaseURL: "http://test.surreal", Marshaler: c, Unmarshaler: unmarshaler})
	con.SetHTTPClient(&nethttp.Client{Transport: roundTripFunc(func(req *nethttp.Request) *nethttp.Response {
		body, _ := io.ReadAll(req.Body)
		var r connection.RPCRequest
		_ = c.Unmarshal(body, &r)
		sent = append(sent, r)
		result := []any{map[string]any{"status": "OK", "result": nil}}
		if answer {
			result = []any{statement}
		}
		res, _ := c.Marshal(map[string]any{"id": r.ID, "result": result})
		return &nethttp.Response{StatusCode: nethttp.StatusOK, Body: io.NopCloser(bytes.NewReader(res)), Header: nethttp.Header{}}
	})})

	db, err := surrealdb.FromConnection(context.Background(), con)
	require.NoError(t, err)
	require.NoError(t, db.Use(context.Background(), "ns", "db"))
	sent = nil
	answer = true
	return db, &sent
}

// TestCRUD_compilesToQuery checks that the CRUD functions send SurrealQL with
// the query method, never the deprecated RPC methods, and that a string is
// always a table name.
func TestCRUD_compilesToQuery(t *testing.T) {
	ctx := context.Background()
	rec := models.NewRecordID("person", "tobie")
	table := models.Table("person")
	data := map[string]any{"name": "Tobie"}
	patches := []surrealdb.PatchData{{Op: "replace", Path: "/name", Value: "T"}}
	rel := &surrealdb.Relationship{In: rec, Out: models.NewRecordID("person", "jaime"), Relation: "knows", Data: data}

	cases := []struct {
		name string
		call func(db *surrealdb.DB) error
		sql  string
		what any
	}{
		{"select record", func(db *surrealdb.DB) error { _, err := surrealdb.Select[any](ctx, db, rec); return err },
			"SELECT * FROM ONLY $what", rec},
		{"select table", func(db *surrealdb.DB) error { _, err := surrealdb.Select[any](ctx, db, table); return err },
			"SELECT * FROM $what", table},
		{"select string is a table", func(db *surrealdb.DB) error {
			_, err := surrealdb.Select[any](ctx, db, "person:tobie")
			return err
		}, "SELECT * FROM type::table($what)", "person:tobie"},
		{"create record", func(db *surrealdb.DB) error { _, err := surrealdb.Create[any](ctx, db, rec, data); return err },
			"CREATE ONLY $what CONTENT $data RETURN AFTER", rec},
		{"create string", func(db *surrealdb.DB) error { _, err := surrealdb.Create[any](ctx, db, "person", data); return err },
			"CREATE ONLY type::table($what) CONTENT $data RETURN AFTER", "person"},
		{"create table", func(db *surrealdb.DB) error { _, err := surrealdb.Create[any](ctx, db, table, nil); return err },
			"CREATE $what RETURN AFTER", table},
		{"create with a nil pointer", func(db *surrealdb.DB) error {
			var p *struct{ Name string }
			_, err := surrealdb.Create[any](ctx, db, rec, p)
			return err
		}, "CREATE ONLY $what RETURN AFTER", rec},
		{"update with a nil map", func(db *surrealdb.DB) error {
			var m map[string]any
			_, err := surrealdb.Update[any](ctx, db, rec, m)
			return err
		}, "UPDATE ONLY $what RETURN AFTER", rec},
		{"upsert with a nil map", func(db *surrealdb.DB) error {
			var m map[string]any
			_, err := surrealdb.Upsert[any](ctx, db, rec, m)
			return err
		}, "UPSERT ONLY $what RETURN AFTER", rec},
		{"merge with a nil map", func(db *surrealdb.DB) error {
			var m map[string]any
			_, err := surrealdb.Merge[any](ctx, db, rec, m)
			return err
		}, "UPDATE ONLY $what RETURN AFTER", rec},
		{"update with an empty map", func(db *surrealdb.DB) error {
			_, err := surrealdb.Update[any](ctx, db, rec, map[string]any{})
			return err
		}, "UPDATE ONLY $what CONTENT $data RETURN AFTER", rec},
		{"update", func(db *surrealdb.DB) error { _, err := surrealdb.Update[any](ctx, db, rec, data); return err },
			"UPDATE ONLY $what CONTENT $data RETURN AFTER", rec},
		{"upsert", func(db *surrealdb.DB) error { _, err := surrealdb.Upsert[any](ctx, db, table, data); return err },
			"UPSERT $what CONTENT $data RETURN AFTER", table},
		{"merge", func(db *surrealdb.DB) error { _, err := surrealdb.Merge[any](ctx, db, rec, data); return err },
			"UPDATE ONLY $what MERGE $data RETURN AFTER", rec},
		{"patch", func(db *surrealdb.DB) error { _, err := surrealdb.Patch(ctx, db, rec, patches); return err },
			"UPDATE ONLY $what PATCH $data RETURN DIFF", rec},
		{"patch returning nothing", func(db *surrealdb.DB) error {
			_, err := surrealdb.PatchWithResult[any](ctx, db, rec, patches, surrealdb.ReturnNone)
			return err
		}, "UPDATE $what PATCH $data RETURN NONE", rec},
		{"patch returning after", func(db *surrealdb.DB) error {
			_, err := surrealdb.PatchWithResult[any](ctx, db, rec, patches, surrealdb.ReturnAfter)
			return err
		}, "UPDATE ONLY $what PATCH $data RETURN AFTER", rec},
		{"delete", func(db *surrealdb.DB) error { _, err := surrealdb.Delete[any](ctx, db, rec); return err },
			"DELETE ONLY $what RETURN BEFORE", rec},
		{"insert", func(db *surrealdb.DB) error { _, err := surrealdb.Insert[any](ctx, db, table, data); return err },
			"INSERT INTO $table $data RETURN AFTER", nil},
		{"relate", func(db *surrealdb.DB) error { _, err := surrealdb.Relate[any](ctx, db, rel); return err },
			"RELATE ONLY $in->$relation->$out CONTENT $data RETURN AFTER", nil},
		{"insert relation", func(db *surrealdb.DB) error { _, err := surrealdb.InsertRelation[any](ctx, db, rel); return err },
			"INSERT RELATION INTO $table $data RETURN AFTER", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, sent := recordingDB(t)
			require.NoError(t, tc.call(db))
			require.Len(t, *sent, 1)

			req := (*sent)[0]
			assert.Equal(t, "query", req.Method)
			require.Len(t, req.Params, 2)
			assert.Equal(t, tc.sql, req.Params[0])
			if tc.what != nil {
				vars, ok := req.Params[1].(map[string]any)
				require.True(t, ok)
				assert.EqualValues(t, tc.what, vars["what"])
			}
		})
	}

	t.Run("invalid return mode", func(t *testing.T) {
		db, sent := recordingDB(t)
		_, err := surrealdb.PatchWithResult[any](ctx, db, rec, patches, "SIDEWAYS")
		require.Error(t, err)
		assert.Empty(t, *sent)
	})
}

func TestCRUD_server(t *testing.T) {
	ctx := context.Background()
	db := testenv.MustNew("crud", "crud", "person", "thrower")
	defer db.Close(ctx)

	tobie := models.NewRecordID("person", "tobie")
	_, err := surrealdb.Create[map[string]any](ctx, db, tobie, map[string]any{"name": "Tobie", "age": 30})
	require.NoError(t, err)

	t.Run("a string is a table, not a record ID", func(t *testing.T) {
		// There is no table named person:tobie: v3 says so, v2 finds nothing in it.
		got, err := surrealdb.Select[[]map[string]any](ctx, db, "person:tobie")
		if err != nil {
			assert.Contains(t, err.Error(), "table 'person:tobie' does not exist")
		} else if got != nil {
			assert.Empty(t, *got)
		}

		one, err := surrealdb.Select[map[string]any](ctx, db, tobie)
		require.NoError(t, err)
		require.NotNil(t, one)
		assert.Equal(t, "Tobie", (*one)["name"])
	})

	t.Run("patch returns the diff, or what is asked for", func(t *testing.T) {
		diff, err := surrealdb.Patch(ctx, db, tobie, []surrealdb.PatchData{{Op: "replace", Path: "/age", Value: 31}})
		require.NoError(t, err)
		require.NotNil(t, diff)
		require.Len(t, *diff, 1)
		assert.Equal(t, "/age", (*diff)[0].Path)

		after, err := surrealdb.PatchWithResult[map[string]any](ctx, db, tobie,
			[]surrealdb.PatchData{{Op: "replace", Path: "/age", Value: 32}}, surrealdb.ReturnAfter)
		require.NoError(t, err)
		require.NotNil(t, after)
		assert.EqualValues(t, 32, (*after)["age"])
	})

	t.Run("a failed statement is an RPC error", func(t *testing.T) {
		_, err := surrealdb.Create[map[string]any](ctx, db, tobie, map[string]any{"name": "again"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")

		//nolint:staticcheck // RPCError is what the create RPC returned
		var rpcErr *surrealdb.RPCError
		require.True(t, errors.As(err, &rpcErr))
		assert.Equal(t, -32000, rpcErr.Code)
		var se *surrealdb.ServerError
		require.True(t, errors.As(err, &se))
		assert.Equal(t, rpcErr.Message, se.Message)
	})

	t.Run("a failed statement keeps the code of its RPC method", func(t *testing.T) {
		_, err := surrealdb.Query[any](ctx, db, `DEFINE TABLE thrower SCHEMALESS; DEFINE FIELD x ON thrower VALUE { THROW "boom" };`, nil)
		require.NoError(t, err)

		// The deprecated create RPC: SurrealDB v3 assigns a THROW -32006.
		//nolint:staticcheck // RPCError is what the create RPC returned
		var want *surrealdb.RPCError
		require.True(t, errors.As(surrealdb.Send[any](ctx, db, &connection.RPCResponse[any]{}, "create",
			models.NewRecordID("thrower", 1), map[string]any{"x": 1}), &want))

		_, err = surrealdb.Create[map[string]any](ctx, db, models.NewRecordID("thrower", 2), map[string]any{"x": 1})
		//nolint:staticcheck // RPCError is what the create RPC returned
		var rpcErr *surrealdb.RPCError
		require.True(t, errors.As(err, &rpcErr))
		assert.Equal(t, want.Code, rpcErr.Code)

		var se *surrealdb.ServerError
		require.True(t, errors.As(err, &se))
		assert.Equal(t, want.Code, se.Code)
		if se.Kind != "" { // SurrealDB v3
			assert.Equal(t, surrealdb.KindThrown, se.Kind)
			assert.Equal(t, -32006, rpcErr.Code)
		}
	})

	t.Run("delete returns the record as it was", func(t *testing.T) {
		before, err := surrealdb.Delete[map[string]any](ctx, db, tobie)
		require.NoError(t, err)
		require.NotNil(t, before)
		assert.Equal(t, "Tobie", (*before)["name"])

		gone, err := surrealdb.Select[map[string]any](ctx, db, tobie)
		require.NoError(t, err)
		// surrealcbor decodes NONE to nil; fxamacker/cbor to a pointer to a
		// zero value, as ExampleSelect_nonExistentRecord_fxamackercbor shows.
		if gone != nil {
			assert.Empty(t, *gone)
		}
	})
}

// TestCRUD_statementErrorDetails checks that the details of a failed
// statement are decoded as those of an RPC error, whatever the connection's
// codec decodes maps to, so that its code and the predicates see them.
func TestCRUD_statementErrorDetails(t *testing.T) {
	ctx := context.Background()
	//nolint:staticcheck // the legacy codec, decoding maps to map[any]any
	anyMaps := &models.CborUnmarshaler{DecOptions: cbor.DecOptions{DefaultMapType: reflect.TypeOf(map[any]any(nil))}}

	cases := []struct {
		name    string
		kind    string
		details map[string]any
		code    int
		is      func(error) bool
	}{
		{"transaction conflict", surrealdb.KindQuery, map[string]any{"kind": "TransactionConflict"},
			-32009, surrealdb.IsTransactionConflict},
		{"token expired", surrealdb.KindNotAllowed,
			map[string]any{"kind": "Auth", "details": map[string]any{"kind": "TokenExpired"}},
			-32002, surrealdb.IsTokenExpired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := stubDB(t, anyMaps, map[string]any{
				"status": "ERR", "result": "failed", "kind": tc.kind, "details": tc.details,
			})
			_, err := surrealdb.Create[map[string]any](ctx, db, models.NewRecordID("person", "tobie"), nil)
			require.Error(t, err)
			assert.True(t, tc.is(err))

			//nolint:staticcheck // RPCError is what the create RPC returned
			var rpcErr *surrealdb.RPCError
			require.True(t, errors.As(err, &rpcErr))
			assert.Equal(t, tc.code, rpcErr.Code)

			var se *surrealdb.ServerError
			require.True(t, errors.As(err, &se))
			assert.Equal(t, tc.kind, se.Kind)
			assert.Equal(t, tc.details, se.Details)
		})
	}
}

// TestCRUD_returnNone checks that PatchWithResult with ReturnNone returns
// nil, whatever the codec decodes the server's NONE to.
func TestCRUD_returnNone(t *testing.T) {
	ctx := context.Background()
	patches := []surrealdb.PatchData{{Op: "replace", Path: "/age", Value: 33}}

	//nolint:staticcheck // fxamacker decodes NONE to a pointer to a zero value
	fxamacker := &models.CborUnmarshaler{}
	for name, unmarshaler := range map[string]codec.Unmarshaler{"surrealcbor": surrealcbor.New(), "fxamacker": fxamacker} {
		t.Run(name, func(t *testing.T) {
			db, _ := stubDB(t, unmarshaler, map[string]any{"status": "OK", "result": models.None})
			got, err := surrealdb.PatchWithResult[map[string]any](ctx, db, models.NewRecordID("person", "tobie"),
				patches, surrealdb.ReturnNone)
			require.NoError(t, err)
			assert.Nil(t, got)
		})
	}

	t.Run("server", func(t *testing.T) {
		for _, impl := range []testenv.CBORImpl{testenv.CBORImplSurrealCBOR, testenv.CBORImplFxamackerCBOR} {
			t.Run(impl.String(), func(t *testing.T) {
				c := testenv.MustNewConfig("crud", "crud", "patched")
				c.CBORImpl = impl
				db := c.MustNew()
				defer db.Close(ctx)

				rec := models.NewRecordID("patched", "tobie")
				_, err := surrealdb.Create[map[string]any](ctx, db, rec, map[string]any{"age": 32})
				require.NoError(t, err)

				got, err := surrealdb.PatchWithResult[map[string]any](ctx, db, rec, patches, surrealdb.ReturnNone)
				require.NoError(t, err)
				assert.Nil(t, got)

				after, err := surrealdb.Select[map[string]any](ctx, db, rec)
				require.NoError(t, err)
				require.NotNil(t, after)
				assert.EqualValues(t, 33, (*after)["age"])
			})
		}
	})
}
