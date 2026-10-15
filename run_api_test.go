package surrealdb_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
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

	_, err = surrealdb.Run[any](ctx, db, "fn::greet", "only one")
	require.Error(t, err, "the server's error is returned")

	_, err = surrealdb.Run[any](ctx, db, "fn::greet(); DELETE user; fn::x")
	require.ErrorContains(t, err, "invalid function name")
	_, err = surrealdb.RunVersion[any](ctx, db, "ml::model", "1.0.0>(); DELETE user; //")
	require.ErrorContains(t, err, "invalid function version")
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
