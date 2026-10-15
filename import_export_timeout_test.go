package surrealdb

import (
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/http"
)

// A bulk transfer over an HTTP connection is bounded by its context, not by
// the timeout the connection gives RPC requests.
func TestHTTPRequest_outlastsTheRPCTimeout(t *testing.T) {
	srv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, _ *nethttp.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte("done"))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	con := http.New(connection.NewConfig(u)).SetTimeout(100 * time.Millisecond)
	db := &DB{con: con}

	res, err := db.httpRequest(context.Background(), "/export", nethttp.NoBody, "application/json", "application/octet-stream")
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, nethttp.StatusOK, res.StatusCode)

	_, client := con.HTTPEndpoint()
	assert.Equal(t, 100*time.Millisecond, client.Timeout, "the connection's own client is left as it was")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	res, err = db.httpRequest(ctx, "/export", nethttp.NoBody, "application/json", "application/octet-stream")
	if err == nil {
		_ = res.Body.Close()
	}
	require.ErrorIs(t, err, context.DeadlineExceeded, "the context still bounds it")
}

// Export asks for the text/plain document the server sends.
func TestExport_acceptsTextPlain(t *testing.T) {
	var accept string
	srv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		accept = r.Header.Get("Accept")
		_, _ = w.Write([]byte("OPTION IMPORT;"))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	db := &DB{con: http.New(connection.NewConfig(u))}

	var out strings.Builder
	require.NoError(t, db.Export(context.Background(), &out, nil))
	assert.Equal(t, "text/plain", accept)
	assert.Equal(t, "OPTION IMPORT;", out.String())
}
