package surrealdb_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

func TestRun(t *testing.T) {
	ctx := context.Background()
	db := testenv.MustNew("run", "run", "t")
	defer db.Close(ctx)

	n, err := surrealdb.Run[int](ctx, db, "string::len", "surreal")
	require.NoError(t, err)
	assert.Equal(t, 7, *n)

	_, err = surrealdb.Query[any](ctx, db,
		"DEFINE FUNCTION OVERWRITE fn::greet($name: string, $greeting: string) { RETURN $greeting + ', ' + $name };", nil)
	require.NoError(t, err)

	greeting, err := surrealdb.Run[string](ctx, db, "fn::greet", "Tobie", "Hello")
	require.NoError(t, err)
	assert.Equal(t, "Hello, Tobie", *greeting)

	none, err := surrealdb.Run[any](ctx, db, "rand::uuid")
	require.NoError(t, err)
	assert.NotNil(t, none, "a function without arguments")

	count, err := surrealdb.Run[int](ctx, db, "count", []int{1, 2, 3})
	require.NoError(t, err, "a built-in without a namespace")
	assert.Equal(t, 3, *count)

	random, err := surrealdb.Run[float64](ctx, db, "rand")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, *random, 0.0)

	upper, err := surrealdb.Run[string](ctx, db, "string::uppercase", "surreal")
	require.NoError(t, err)
	assert.Equal(t, "SURREAL", *upper)

	_, err = surrealdb.Query[any](ctx, db,
		"DEFINE FUNCTION OVERWRITE fn::nested::twice($n: int) { RETURN $n * 2 };", nil)
	require.NoError(t, err)
	twice, err := surrealdb.Run[int](ctx, db, "fn::nested::twice", 21)
	require.NoError(t, err)
	assert.Equal(t, 42, *twice)

	_, err = surrealdb.Run[any](ctx, db, "fn::greet", "only one")
	require.Error(t, err, "the server's error is returned")

	_, err = surrealdb.Run[any](ctx, db, "fn::greet(); DELETE user; fn::x")
	require.ErrorContains(t, err, "invalid function name")
	_, err = surrealdb.RunVersion[any](ctx, db, "ml::model", "1.0.0>(); DELETE user; //")
	require.ErrorContains(t, err, "invalid function version")

	// A statement keyword is not a function name, so nothing is deleted.
	_, err = surrealdb.Create[any](ctx, db, models.NewRecordID("t", 1), nil)
	require.NoError(t, err)
	_, err = surrealdb.Run[any](ctx, db, "DELETE", models.Table("t"))
	require.ErrorContains(t, err, "invalid function name")
	left, err := surrealdb.Select[[]any](ctx, db, models.Table("t"))
	require.NoError(t, err)
	assert.Len(t, *left, 1)
}

// TestRun_refusesInvalidNames checks that a name or version that is not one
// of a function is refused before anything is sent. A bare word would be
// parsed as whatever it names: "DELETE" would send RETURN DELETE($arg0).
func TestRun_refusesInvalidNames(t *testing.T) {
	ctx := context.Background()
	db, sent := recordingDB(t)
	table := models.Table("user")

	for _, name := range []string{
		"DELETE", "delete", "Delete", "UPDATE", "update", "UPSERT", "CREATE", "INSERT", "RELATE",
		"SELECT", "LET", "DEFINE", "REMOVE", "KILL", "LIVE", "INFO", "RETURN", "IF", "fn", "len",
		"", "fn::", "::", "::fn", "fn:::a", "fn::::a", "a:b", "fn::a::", "fn::greet(); DELETE user; fn::x",
		"fn::a b", "fn::a-b", "fn::a.b",
	} {
		_, err := surrealdb.Run[any](ctx, db, name, table)
		assert.ErrorContains(t, err, "invalid function name", "%q", name)
		_, err = surrealdb.RunVersion[any](ctx, db, name, "1.0.0", table)
		assert.ErrorContains(t, err, "invalid function name", "%q", name)
	}

	for _, version := range []string{".", "1", "1.0", "1..0", "1.0.0.0", "a.b.c", "1.0.0>(); DELETE user; //"} {
		_, err := surrealdb.RunVersion[any](ctx, db, "ml::model", version, table)
		assert.ErrorContains(t, err, "invalid function version", "%q", version)
	}

	assert.Empty(t, *sent, "nothing is sent for an invalid name or version")

	for _, name := range []string{
		"count", "not", "rand", "sleep", "string::len", "string::is::email", "fn::greet", "fn::a::b",
		"mod::x::y", "ml::model", "rand::uuid", "fn::_private", "fn::v2",
	} {
		_, err := surrealdb.Run[any](ctx, db, name)
		assert.NoError(t, err, "%q", name)
	}
	_, err := surrealdb.RunVersion[any](ctx, db, "ml::model", "1.0.0", 1)
	require.NoError(t, err)
	require.NotEmpty(t, *sent)
	assert.Equal(t, "RETURN ml::model<1.0.0>($arg0)", (*sent)[len(*sent)-1].Params[0])
}

func TestAPI(t *testing.T) {
	if v := getVersion(t); !v.IsV3OrLater() {
		t.Skipf("DEFINE API needs SurrealDB v3+, got %s", v)
	}
	ctx := context.Background()
	db := testenv.MustNew("api", "api", "t")
	defer db.Close(ctx)

	_, err := surrealdb.Query[any](ctx, db, `
		DEFINE API OVERWRITE "/greet/:name"
			FOR get THEN {
				RETURN { status: 200, body: { hello: $request.params.name, q: $request.query.q } };
			}
			FOR post THEN {
				RETURN { status: 201, headers: { "x-made": "yes" }, body: $request.body };
			};
	`, nil)
	require.NoError(t, err)

	got, err := surrealdb.API[map[string]any](ctx, db, "/greet/tobie", &surrealdb.APIRequest{Query: map[string]string{"q": "1"}})
	require.NoError(t, err)
	assert.Equal(t, 200, got.Status)
	assert.Equal(t, map[string]any{"hello": "tobie", "q": "1"}, got.Body)

	posted, err := surrealdb.API[map[string]any](ctx, db, "/greet/tobie", &surrealdb.APIRequest{
		Method: "POST",
		Body:   map[string]any{"a": 1},
	})
	require.NoError(t, err)
	assert.Equal(t, 201, posted.Status)
	assert.Equal(t, "yes", posted.Headers["x-made"])
	assert.EqualValues(t, 1, posted.Body["a"])

	missing, err := surrealdb.API[any](ctx, db, "/nowhere", nil)
	require.NoError(t, err, "an unsuccessful status is not an error")
	assert.Equal(t, 404, missing.Status)

	// The body of an error does not have the type of a successful one.
	typed, err := surrealdb.API[struct{ Hello string }](ctx, db, "/nowhere", nil)
	require.NoError(t, err, "an error body that does not fit TBody is not an error")
	assert.Equal(t, 404, typed.Status)
	assert.Empty(t, typed.Body.Hello)
}

// TestAPI_errorBody checks that API returns the status of an unsuccessful
// response whose body does not fit TBody, on both codecs, and that such a
// body is still an error for a successful status.
func TestAPI_errorBody(t *testing.T) {
	ctx := context.Background()
	type greeting struct {
		Hello string `json:"hello"`
	}

	//nolint:staticcheck // the fxamacker codec is still supported
	fxamacker := &models.CborUnmarshaler{}
	for name, unmarshaler := range map[string]codec.Unmarshaler{"surrealcbor": surrealcbor.New(), "fxamacker": fxamacker} {
		t.Run(name, func(t *testing.T) {
			db, _ := stubDB(t, unmarshaler, map[string]any{"status": "OK", "result": map[string]any{
				"status": 404, "body": "Not found",
			}})
			res, err := surrealdb.API[greeting](ctx, db, "/nowhere", nil)
			require.NoError(t, err)
			assert.Equal(t, 404, res.Status)
			assert.Equal(t, greeting{}, res.Body)

			db, _ = stubDB(t, unmarshaler, map[string]any{"status": "OK", "result": map[string]any{
				"status": 200, "headers": map[string]any{"x-a": "b"}, "body": map[string]any{"hello": "tobie"},
			}})
			res, err = surrealdb.API[greeting](ctx, db, "/greet", nil)
			require.NoError(t, err)
			assert.Equal(t, 200, res.Status)
			assert.Equal(t, "b", res.Headers["x-a"])
			assert.Equal(t, greeting{Hello: "tobie"}, res.Body)

			db, _ = stubDB(t, unmarshaler, map[string]any{"status": "OK", "result": map[string]any{
				"status": 200, "body": "not a greeting",
			}})
			_, err = surrealdb.API[greeting](ctx, db, "/greet", nil)
			require.ErrorContains(t, err, "unmarshaling body")

			db, _ = stubDB(t, unmarshaler, map[string]any{"status": "OK", "result": map[string]any{"status": 204}})
			res, err = surrealdb.API[greeting](ctx, db, "/empty", nil)
			require.NoError(t, err)
			assert.Equal(t, 204, res.Status)
			assert.Equal(t, greeting{}, res.Body)
		})
	}
}

func TestGQL(t *testing.T) {
	ctx := context.Background()
	db := testenv.MustNew("gql", "gql", "person")
	defer db.Close(ctx)

	_, err := surrealdb.Query[any](ctx, db, "CREATE person:tobie SET name = 'Tobie'", nil)
	require.NoError(t, err)

	res, err := surrealdb.GQL[[]map[string]any](ctx, db, "MATCH (p:person) RETURN p.name AS name", nil)
	if err != nil && strings.Contains(err.Error(), "Method not found") {
		t.Skipf("SurrealDB %s has no gql method", getVersion(t))
	}
	require.NoError(t, err)
	require.Len(t, *res, 1)
	assert.Equal(t, []map[string]any{{"name": "Tobie"}}, (*res)[0].Result)
}
