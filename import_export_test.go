package surrealdb_test

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/contrib/rews"
	"github.com/surrealdb/surrealdb.go/contrib/testenv"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// rootClient connects over the given endpoint as root, in a fresh database.
func rootClient(t *testing.T, endpoint, database string) *surrealdb.DB {
	t.Helper()
	ctx := context.Background()
	db, err := surrealdb.FromEndpointURLString(ctx, endpoint)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close(ctx) })

	_, err = db.SignIn(ctx, surrealdb.Auth{Username: "root", Password: "root"})
	require.NoError(t, err)
	require.NoError(t, db.Use(ctx, "import_export", database))
	_, err = surrealdb.Query[any](ctx, db, "REMOVE DATABASE IF EXISTS "+database+"; DEFINE DATABASE "+database, nil)
	require.NoError(t, err)
	require.NoError(t, db.Use(ctx, "import_export", database))
	return db
}

func TestExportThenImport(t *testing.T) {
	ctx := context.Background()

	for family, endpoint := range endpoints() {
		t.Run(family, func(t *testing.T) {
			source := rootClient(t, endpoint, "source_"+family)
			_, err := surrealdb.Query[any](ctx, source, `
				DEFINE TABLE person SCHEMALESS;
				CREATE person:tobie SET name = 'Tobie';
				CREATE person:jaime SET name = 'Jaime';
				DEFINE TABLE other;
				CREATE other:one;
			`, nil)
			require.NoError(t, err)

			var all bytes.Buffer
			require.NoError(t, source.Export(ctx, &all, nil))
			assert.Contains(t, all.String(), "OPTION IMPORT")
			assert.Contains(t, all.String(), "Tobie")

			var some bytes.Buffer
			require.NoError(t, source.Export(ctx, &some, &surrealdb.ExportOptions{Tables: []string{"other"}}))
			assert.NotContains(t, some.String(), "Tobie", "only the tables asked for are exported")
			assert.Contains(t, some.String(), "other")

			target := rootClient(t, endpoint, "target_"+family)
			require.NoError(t, target.Import(ctx, &all))

			tobie, err := surrealdb.Select[map[string]any](ctx, target, models.NewRecordID("person", "tobie"))
			require.NoError(t, err)
			require.NotNil(t, tobie)
			assert.Equal(t, "Tobie", (*tobie)["name"])
		})
	}
}

func TestImport_reportsFailedStatements(t *testing.T) {
	ctx := context.Background()

	for family, endpoint := range endpoints() {
		t.Run(family, func(t *testing.T) {
			db := rootClient(t, endpoint, "failing_"+family)

			err := db.Import(ctx, strings.NewReader(`OPTION IMPORT;
				CREATE thing:one;
				CREATE thing:one;
				CREATE thing:two;
			`))
			require.Error(t, err)

			var importErr *surrealdb.ImportError
			require.True(t, errors.As(err, &importErr), "%T: %v", err, err)
			require.Len(t, importErr.Failures, 1)
			assert.Contains(t, importErr.Failures[0].Error(), "already exists")

			var se *surrealdb.ServerError
			assert.True(t, errors.As(importErr.Failures[0], &se))

			two, err := surrealdb.Select[map[string]any](ctx, db, models.NewRecordID("thing", "two"))
			require.NoError(t, err)
			assert.NotNil(t, two, "the statements which succeeded are kept")
		})
	}
}

// Export and Health work through the reconnecting WebSocket connection, which
// forwards them to the connection it holds.
func TestExport_overRews(t *testing.T) {
	ctx := context.Background()

	u, err := url.ParseRequestURI(testenv.GetSurrealDBWSURL())
	require.NoError(t, err)
	conf := connection.NewConfig(u)
	con := rews.New(func(context.Context) (*gorillaws.Connection, error) {
		return gorillaws.New(conf), nil
	}, time.Second, conf.Unmarshaler, conf.Logger)
	db, err := surrealdb.FromConnection(ctx, con)
	require.NoError(t, err)
	defer db.Close(ctx)

	require.NoError(t, db.Health(ctx))

	_, err = db.SignIn(ctx, surrealdb.Auth{Username: "root", Password: "root"})
	require.NoError(t, err)
	require.NoError(t, db.Use(ctx, "import_export", "rews"))
	_, err = surrealdb.Query[any](ctx, db, `
		REMOVE DATABASE IF EXISTS rews; DEFINE DATABASE rews;
		USE DB rews;
		CREATE person:tobie SET name = 'Tobie';
	`, nil)
	require.NoError(t, err)

	var out bytes.Buffer
	require.NoError(t, db.Export(ctx, &out, nil))
	assert.Contains(t, out.String(), "Tobie")
}
