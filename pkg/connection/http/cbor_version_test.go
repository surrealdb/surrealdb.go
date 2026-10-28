package http

import (
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

type seenRequest struct {
	contentType, accept string
	body                []byte
}

// newCBOREchoServer is an HTTP /rpc endpoint that answers each request with
// its first parameter. With speaksV2 it behaves as SurrealDB 4.0 does: it
// answers in cbor.2 when asked to, and refuses a cbor.2 request that wants a
// plain CBOR answer. Without it, it behaves as 3.x does and always answers
// application/cbor.
func newCBOREchoServer(t *testing.T, speaksV2 bool) (srv *httptest.Server, seen func() []seenRequest) {
	t.Helper()
	var mu sync.Mutex
	var requests []seenRequest
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, seenRequest{r.Header.Get("Content-Type"), r.Header.Get("Accept"), body})
		mu.Unlock()

		reqV2 := r.Header.Get("Content-Type") == connection.MIMECBOR2
		wantV2 := r.Header.Get("Accept") == connection.MIMECBOR2
		if speaksV2 && reqV2 && !wantV2 {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}
		in := surrealcbor.New()
		if speaksV2 && reqV2 {
			in = surrealcbor.NewV2()
		}
		var req struct {
			ID     any   `cbor:"id"`
			Params []any `cbor:"params"`
		}
		if err := in.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		out, contentType := surrealcbor.New(), connection.MIMECBOR
		if speaksV2 && wantV2 {
			out, contentType = surrealcbor.NewV2(), connection.MIMECBOR2
		}
		var result any
		if len(req.Params) > 0 {
			result = req.Params[0]
		}
		res, err := out.Marshal(map[string]any{"id": req.ID, "result": result})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(res)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []seenRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]seenRequest(nil), requests...)
	}
}

func TestCBORNegotiation(t *testing.T) {
	const (
		plain = connection.MIMECBOR
		v2    = connection.MIMECBOR2
	)
	cases := []struct {
		name         string
		speaksV2     bool
		fxamacker    bool
		disable      bool
		wantRequests [][2]string // Content-Type and Accept of the two requests
		wantVersion  connection.CBORVersion
		wantEchoed   [2]any // the first request goes out as plain CBOR, so its int8 is widened
	}{
		{"4.0 server", true, false, false, [][2]string{{plain, v2}, {v2, v2}}, connection.CBORv2, [2]any{int64(-5), int8(-5)}},
		{"3.x server", false, false, false, [][2]string{{plain, v2}, {plain, v2}}, connection.CBORv1, [2]any{int64(-5), int64(-5)}},
		{"cbor.2 disabled", true, false, true, [][2]string{{plain, plain}, {plain, plain}}, connection.CBORv1, [2]any{int64(-5), int64(-5)}},
		{"deprecated fxamacker codec", true, true, false, [][2]string{{plain, plain}, {plain, plain}}, connection.CBORv1, [2]any{int64(-5), int64(-5)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, seen := newCBOREchoServer(t, tc.speaksV2)
			conf := &connection.Config{BaseURL: srv.URL, DisableCBORv2: tc.disable}
			if tc.fxamacker {
				conf.Marshaler = &models.CborMarshaler{}     //nolint:staticcheck // testing the deprecated codec
				conf.Unmarshaler = &models.CborUnmarshaler{} //nolint:staticcheck // testing the deprecated codec
			} else {
				c := surrealcbor.New()
				conf.Marshaler, conf.Unmarshaler = c, c
			}
			conn := New(conf)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			require.NoError(t, conn.Use(ctx, "ns", "db"))
			assert.Equal(t, connection.CBORv1, conn.CBORVersion(), "before any request")

			var got any
			for i := 0; i < 2; i++ {
				res, err := conn.Send(ctx, "echo", int8(-5))
				require.NoError(t, err)
				require.NoError(t, conn.GetUnmarshaler().Unmarshal(*res.Result, &got))
				assert.Equal(t, tc.wantEchoed[i], got, "request %d", i)
			}
			assert.Equal(t, tc.wantVersion, conn.CBORVersion())

			requests := seen()
			require.Len(t, requests, 2)
			for i, want := range tc.wantRequests {
				assert.Equal(t, want[0], requests[i].contentType, "Content-Type of request %d", i)
				assert.Equal(t, want[1], requests[i].accept, "Accept of request %d", i)
				// Only a cbor.2 request carries the tag.
				assert.Equal(t, want[0] == v2, strings.Contains(hex.EncodeToString(requests[i].body), "d8b424"), "body of request %d", i)
			}
		})
	}
}

func TestCBORVersionFromContentType(t *testing.T) {
	for ct, want := range map[string]connection.CBORVersion{
		"application/cbor":          connection.CBORv1,
		"application/cbor; v=1":     connection.CBORv1,
		"application/cbor; v=2":     connection.CBORv2,
		"application/cbor;v=2":      connection.CBORv2,
		`application/cbor; v="2"`:   connection.CBORv2,
		"application/cbor; V=2":     connection.CBORv2,
		"Application/CBOR; v=2":     connection.CBORv2,
		"application/cbor; v=2; x=": 0,
		"application/json":          0,
		"application/cbor; v=3":     0,
		"":                          0,
	} {
		got, ok := connection.CBORVersionFromContentType(ct)
		assert.Equal(t, want, got, ct)
		assert.Equal(t, want != 0, ok, ct)
	}
}

// TestCBORNegotiationStepsBack checks that a connection sends each request in
// the version of the latest response, as behind a load balancer that mixes a
// 4.0 server with a 3.x one during a rolling upgrade or a rollback. The 3.x
// server reads every body as plain CBOR, so refuses the tags of cbor.2.
func TestCBORNegotiationStepsBack(t *testing.T) {
	const (
		plain = connection.MIMECBOR
		v2    = connection.MIMECBOR2
	)
	// The server behind the URL for each request, and what the client sees.
	steps := []struct {
		speaksV2    bool
		contentType string // of the request
		ok          bool
	}{
		{true, plain, true},  // no response yet
		{false, v2, false},   // 3.x cannot read the cbor.2 body
		{true, plain, true},  // the error stepped back
		{false, v2, false},   //
		{false, plain, true}, // a plain request, which 3.x reads
		{true, plain, true},  // the plain response stepped back
		{true, v2, true},     // back on cbor.2
	}

	var mu sync.Mutex
	var requests []seenRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		i := len(requests)
		requests = append(requests, seenRequest{r.Header.Get("Content-Type"), r.Header.Get("Accept"), body})
		mu.Unlock()
		if i >= len(steps) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		speaksV2 := steps[i].speaksV2
		in := surrealcbor.New()
		if speaksV2 && r.Header.Get("Content-Type") == v2 {
			in = surrealcbor.NewV2()
		}
		var req struct {
			ID     any   `cbor:"id"`
			Params []any `cbor:"params"`
		}
		if err := in.Unmarshal(body, &req); err != nil {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Parse error"))
			return
		}
		out, contentType := surrealcbor.New(), plain
		if speaksV2 && r.Header.Get("Accept") == v2 {
			out, contentType = surrealcbor.NewV2(), v2
		}
		res, err := out.Marshal(map[string]any{"id": req.ID, "result": req.Params})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(res)
	}))
	t.Cleanup(srv.Close)

	c := surrealcbor.New()
	conn := New(&connection.Config{BaseURL: srv.URL, Marshaler: c, Unmarshaler: c})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// A variable bound before the connection speaks cbor.2 is sent in cbor.2
	// too, and the other way round.
	require.NoError(t, conn.Let(ctx, "v", int8(-5)))

	for i, step := range steps {
		if i == 3 {
			require.NoError(t, conn.Let(ctx, "w", int8(-5)))
		}
		_, err := conn.Send(ctx, "query", "RETURN $v", map[string]any{"p": int8(-5)})
		if step.ok {
			require.NoError(t, err, "request %d", i)
		} else {
			var statusErr *StatusError
			require.ErrorAs(t, err, &statusErr, "request %d", i)
			assert.Equal(t, http.StatusBadRequest, statusErr.StatusCode, "request %d", i)
		}
		if i > 0 {
			// Once a server has answered in cbor.2, results may be cbor.2.
			assert.Equal(t, connection.CBORv2, conn.CBORVersion(), "after request %d", i)
		}
	}

	mu.Lock()
	seen := append([]seenRequest(nil), requests...)
	mu.Unlock()
	require.Len(t, seen, len(steps))
	for i, step := range steps {
		assert.Equal(t, step.contentType, seen[i].contentType, "Content-Type of request %d", i)
		assert.Equal(t, v2, seen[i].accept, "Accept of request %d", i)
		// The parameter and every variable carry the tag in cbor.2 only.
		tags := strings.Count(hex.EncodeToString(seen[i].body), "d8b424")
		wantTags := 0
		if step.contentType == v2 {
			wantTags = 2
			if i >= 3 {
				wantTags = 3
			}
		}
		assert.Equal(t, wantTags, tags, "tags in request %d", i)
	}
}
