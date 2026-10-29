package surrealdb

import (
	"context"
	"errors"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/http"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

// fakeServer serves handler, and returns a client over an HTTP connection to
// it.
func fakeServer(t *testing.T, handler nethttp.HandlerFunc) *DB {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return &DB{con: http.New(connection.NewConfig(u))}
}

// answer returns a handler which reads the request and answers with status
// and body.
func answer(status int, body string) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, r *nethttp.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

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

// The answers /import gives, from each server version, and what Import makes
// of them.
func TestImport_answers(t *testing.T) {
	const failed = `{"details":{"details":{"id":"a:1"},"kind":"Record"},"kind":"AlreadyExists",` +
		`"result":"Database record ` + "`a:1`" + ` already exists","status":"ERR","time":"50.917µs","type":null}`

	tests := []struct {
		name     string
		status   int
		body     string
		failures int
		err      string
	}{
		{
			name:   "2.x answers with every statement's result",
			status: 200,
			body: `[{"result":[{"id":"a:1","name":"one"}],"status":"OK","time":"1ms"},` +
				`{"result":"Database record ` + "`a:1`" + ` already exists","status":"ERR","time":"1ms"}]`,
			failures: 1,
		},
		{
			name:   "3.0 answers with OK for OPTION IMPORT",
			status: 200,
			body:   `[{"result":null,"status":"OK","time":"1ms","type":null}]`,
		},
		{
			name:     "3.2 lists only the failures",
			status:   200,
			body:     "[" + failed + "]",
			failures: 1,
		},
		{
			name:     "3.3.1 and later list the failures with 422",
			status:   422,
			body:     "[" + failed + "," + failed + "]",
			failures: 2,
		},
		{
			name:   "no failure",
			status: 200,
			body:   "[]",
		},
		{
			name:   "an answer which is not an array",
			status: 200,
			body:   "{}",
		},
		{
			name:   "a document without OPTION IMPORT",
			status: 400,
			body:   `{"code":400,"details":"Request problems detected","information":"Invalid statement: Import requires OPTION IMPORT"}`,
			err:    `import failed: 400 Bad Request: {"code":400`,
		},
		{
			name:   "a bad token",
			status: 401,
			body:   "There was a problem with authentication\n",
			err:    "import failed: 401 Unauthorized: There was a problem with authentication",
		},
		{
			name:   "an error status without failures",
			status: 500,
			body:   " []",
			err:    "import failed: 500 Internal Server Error",
		},
		{
			name:   "a truncated answer",
			status: 200,
			body:   `[{"result":null,"status":"OK"},`,
			err:    "import: reading the answer",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := fakeServer(t, answer(tt.status, tt.body))
			err := db.Import(context.Background(), strings.NewReader("OPTION IMPORT;"))

			var importErr *ImportError
			switch {
			case tt.failures > 0:
				require.True(t, errors.As(err, &importErr), "%T: %v", err, err)
				require.Len(t, importErr.Failures, tt.failures)
				for _, f := range importErr.Failures {
					var se *ServerError
					require.True(t, errors.As(f, &se), "%T: %v", f, f)
					assert.Equal(t, "Database record `a:1` already exists", se.Message)
				}
			case tt.err != "":
				require.Error(t, err)
				assert.False(t, errors.As(err, &importErr))
				assert.Contains(t, err.Error(), tt.err)
			default:
				require.NoError(t, err)
			}
		})
	}
}

// Import reads the answer one statement result at a time and keeps only the
// failures, so that the records SurrealDB 2.x echoes back are not held in
// memory.
func TestImport_streamsTheAnswer(t *testing.T) {
	element := `{"result":[` + strings.Repeat(`{"id":"t:1","name":"`+strings.Repeat("x", 1000)+`"},`, 63) +
		`{"id":"t:1"}],"status":"OK","time":"1ms"},`
	const elements = 1024
	size := uint64(len(element) * elements)

	db := fakeServer(t, func(w nethttp.ResponseWriter, r *nethttp.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = io.WriteString(w, "[")
		for range elements {
			_, _ = io.WriteString(w, element)
		}
		_, _ = io.WriteString(w, `{"result":"Database record `+"`t:1`"+` already exists","status":"ERR",`+
			`"kind":"AlreadyExists","details":{"kind":"Record","details":{"id":"t:1"}}}]`)
	})

	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	done := make(chan struct{})
	peak := make(chan uint64)
	go func() {
		var high uint64
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			var s runtime.MemStats
			runtime.ReadMemStats(&s)
			high = max(high, s.HeapAlloc)
			select {
			case <-done:
				peak <- high
				return
			case <-ticker.C:
			}
		}
	}()
	err := db.Import(context.Background(), strings.NewReader("OPTION IMPORT;"))
	close(done)
	grown := <-peak - stats.HeapAlloc

	var importErr *ImportError
	require.True(t, errors.As(err, &importErr), "%T: %v", err, err)
	require.Len(t, importErr.Failures, 1)
	assert.Less(t, grown, size/4, "the heap grew by %d MiB for an answer of %d MiB", grown>>20, size>>20)
}

// Import leaves the document open, so that the caller can rewind it and try
// again.
func TestImport_doesNotCloseTheDocument(t *testing.T) {
	db := fakeServer(t, answer(nethttp.StatusUnauthorized, "There was a problem with authentication"))

	f, err := os.CreateTemp(t.TempDir(), "import-*.surql")
	require.NoError(t, err)
	defer f.Close()
	_, err = f.WriteString("OPTION IMPORT;")
	require.NoError(t, err)
	_, err = f.Seek(0, io.SeekStart)
	require.NoError(t, err)

	require.Error(t, db.Import(context.Background(), f))

	_, err = f.Seek(0, io.SeekStart)
	require.NoError(t, err, "the document is still open")
	content, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "OPTION IMPORT;", string(content))
}

// Export goes to the path a reverse proxy mounts SurrealDB under, as RPC does.
func TestExport_keepsTheBaseURLPath(t *testing.T) {
	var path string
	srv := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte("OPTION IMPORT;"))
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	for _, base := range []string{srv.URL + "/surreal", srv.URL + "/surreal/", srv.URL + "/surreal/rpc"} {
		conf := connection.NewConfig(u)
		conf.BaseURL = base
		db := &DB{con: http.New(conf)}

		var out strings.Builder
		require.NoError(t, db.Export(context.Background(), &out, nil), base)
		assert.Equal(t, "/surreal/export", path, base)
	}
}

// noHTTPConnection is a connection which cannot reach the HTTP endpoints, such
// as an embedded one.
type noHTTPConnection struct {
	connection.Connection
}

// Import and Export report a connection which cannot reach the HTTP endpoints
// as Health does.
func TestImportExport_methodNotAvailable(t *testing.T) {
	ctx := context.Background()
	db := &DB{con: noHTTPConnection{}}

	require.ErrorIs(t, db.Import(ctx, strings.NewReader("OPTION IMPORT;")), constants.ErrMethodNotAvailable)
	require.ErrorIs(t, db.Export(ctx, io.Discard, nil), constants.ErrMethodNotAvailable)
	require.ErrorIs(t, db.Health(ctx), constants.ErrMethodNotAvailable)
}

// An ImportError without failures, as a caller's test double may build, still
// has a message.
func TestImportError_noFailures(t *testing.T) {
	assert.Equal(t, "import: statements failed", (&ImportError{}).Error())
}
