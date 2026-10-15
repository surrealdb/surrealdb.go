package surrealdb

import (
	"context"
	nethttp "net/http"
	"net/http/httptest"
	"net/url"
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
