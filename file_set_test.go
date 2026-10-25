package surrealdb_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestFileAndSet_server round-trips files (tag 55) and sets (tag 56) through
// the server, which has both since SurrealDB 3.
func TestFileAndSet_server(t *testing.T) {
	ctx := context.Background()
	db := testenv.MustNew("file_set", "file_set", "t")
	defer db.Close(ctx)

	v, err := testenv.GetVersion(ctx, db)
	require.NoError(t, err)
	if !v.IsV3OrLater() {
		t.Skip("files and sets need SurrealDB v3")
	}

	t.Run("set from the server", func(t *testing.T) {
		res, err := surrealdb.Query[models.Set](ctx, db, "RETURN <set>[3, 1, 1, 2]", nil)
		require.NoError(t, err)
		got := (*res)[0].Result
		require.Len(t, got, 3, "the server deduplicates a set")
		for i, want := range []int{1, 2, 3} {
			assert.EqualValues(t, want, got[i], "the server sorts a set")
		}

		var anything []surrealdb.QueryResult[any]
		r, err := surrealdb.Query[any](ctx, db, "RETURN <set>['b', 'a']", nil)
		require.NoError(t, err)
		anything = *r
		assert.Equal(t, models.NewSet("a", "b"), anything[0].Result, "a set decoded into any stays a set")
	})

	t.Run("set to the server", func(t *testing.T) {
		res, err := surrealdb.Query[map[string]any](ctx, db,
			"RETURN { kind: type::of($s), len: array::len(<array>$s), is_set: $s.is_set() }",
			map[string]any{"s": models.NewSet(int64(2), int64(1), int64(2))})
		require.NoError(t, err)
		got := (*res)[0].Result
		assert.EqualValues(t, 2, got["len"], "the server deduplicates a set")
		assert.Equal(t, true, got["is_set"])

		res, err = surrealdb.Query[map[string]any](ctx, db,
			"RETURN { len: array::len(<array>$s), is_set: $s.is_set() }",
			map[string]any{"s": models.NewSet()})
		require.NoError(t, err)
		got = (*res)[0].Result
		assert.EqualValues(t, 0, got["len"])
		assert.Equal(t, true, got["is_set"], "NewSet() is an empty set, not null")
	})

	t.Run("set in a record", func(t *testing.T) {
		_, err := surrealdb.Query[any](ctx, db, "UPSERT t:1 SET tags = $tags", map[string]any{"tags": models.NewSet("x", "y", "x")})
		require.NoError(t, err)
		type rec struct {
			Tags models.Set `json:"tags"`
		}
		res, err := surrealdb.Query[[]rec](ctx, db, "SELECT tags FROM t:1", nil)
		require.NoError(t, err)
		assert.Equal(t, models.NewSet("x", "y"), (*res)[0].Result[0].Tags)

		type recSlice struct {
			Tags []string `json:"tags"`
		}
		res2, err := surrealdb.Query[[]recSlice](ctx, db, "SELECT tags FROM t:1", nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"x", "y"}, (*res2)[0].Result[0].Tags, "a set can be read into a slice")
	})

	t.Run("file", func(t *testing.T) {
		f := models.NewFile("bucket", "/a/b.txt")
		res, err := surrealdb.Query[models.File](ctx, db, "RETURN $f", map[string]any{"f": f})
		require.NoError(t, err)
		assert.Equal(t, f, (*res)[0].Result)

		kind, err := surrealdb.Query[string](ctx, db, "RETURN type::of($f)", map[string]any{"f": f})
		require.NoError(t, err)
		assert.Equal(t, "file", (*kind)[0].Result)
	})

	t.Run("file string", func(t *testing.T) {
		for _, f := range []models.File{
			models.NewFile("bucket", "/a/b.txt"),
			models.NewFile("buck et", `/a"b\c:d`),
		} {
			res, err := surrealdb.Query[string](ctx, db, "RETURN <string>$f", map[string]any{"f": f})
			require.NoError(t, err)
			assert.Equal(t, (*res)[0].Result, f.String(), "String matches the server's own formatting")
		}
	})

	t.Run("file literal", func(t *testing.T) {
		f := models.NewFile("my_bucket-1.x", "/dir/sub-dir/file_name.v2.tar.gz")
		res, err := surrealdb.Query[models.File](ctx, db, "RETURN "+f.String(), nil)
		if err != nil && strings.Contains(err.Error(), "experimental files feature") {
			t.Skip("parsing a file literal needs --allow-experimental files")
		}
		require.NoError(t, err)
		assert.Equal(t, f, (*res)[0].Result, "the server parses String back into the same file")
	})
}
