package gws

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// cborEchoServer is a WebSocket /rpc endpoint that speaks the subprotocols it
// is given (a SurrealDB 4.0 server speaks "cbor.2" and "cbor", a 3.x server
// only "cbor") and answers each request with its first parameter.
type cborEchoServer struct {
	*httptest.Server

	mu       sync.Mutex
	offered  []string // the Sec-WebSocket-Protocol header of each handshake
	requests [][]byte
}

func newCBOREchoServer(t *testing.T, subprotocols ...string) *cborEchoServer {
	t.Helper()
	s := &cborEchoServer{}
	upgrader := gorilla.Upgrader{Subprotocols: subprotocols}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.offered = append(s.offered, r.Header.Get("Sec-WebSocket-Protocol"))
		s.mu.Unlock()
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		codec := surrealcbor.New()
		if conn.Subprotocol() == connection.SubprotocolCBOR2 {
			codec = surrealcbor.NewV2()
		}
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			s.mu.Lock()
			s.requests = append(s.requests, data)
			s.mu.Unlock()
			var req struct {
				ID     any   `cbor:"id"`
				Params []any `cbor:"params"`
			}
			if codec.Unmarshal(data, &req) != nil {
				return
			}
			var result any
			if len(req.Params) > 0 {
				result = req.Params[0]
			}
			res, err := codec.Marshal(map[string]any{"id": req.ID, "result": result})
			if err != nil {
				return
			}
			if err := conn.WriteMessage(gorilla.BinaryMessage, res); err != nil {
				return
			}
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *cborEchoServer) baseURL() string {
	return "ws" + strings.TrimPrefix(s.URL, "http")
}

func (s *cborEchoServer) lastRequest() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests[len(s.requests)-1]
}

func (s *cborEchoServer) lastOffer() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.offered[len(s.offered)-1]
}

func TestCBORNegotiation(t *testing.T) {
	cases := []struct {
		name          string
		server        []string
		fxamacker     bool
		disable       bool
		wantOffer     string
		wantVersion   connection.CBORVersion
		wantEchoed    any
		wantTaggedReq bool
	}{
		{"4.0 server", []string{"cbor.2", "cbor"}, false, false, "cbor.2, cbor", connection.CBORv2, int8(-5), true},
		{"3.x server", []string{"cbor"}, false, false, "cbor.2, cbor", connection.CBORv1, int64(-5), false},
		{"cbor.2 disabled", []string{"cbor.2", "cbor"}, false, true, "cbor", connection.CBORv1, int64(-5), false},
		{"deprecated fxamacker codec", []string{"cbor.2", "cbor"}, true, false, "cbor", connection.CBORv1, int64(-5), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newCBOREchoServer(t, tc.server...)
			conf := &connection.Config{BaseURL: srv.baseURL(), DisableCBORv2: tc.disable}
			if tc.fxamacker {
				conf.Marshaler = &models.CborMarshaler{}     //nolint:staticcheck // testing the deprecated codec
				conf.Unmarshaler = &models.CborUnmarshaler{} //nolint:staticcheck // testing the deprecated codec
			} else {
				c := surrealcbor.New()
				conf.Marshaler, conf.Unmarshaler = c, c
			}
			conn := newTestConnection(conf)
			assert.Equal(t, connection.CBORv1, conn.CBORVersion(), "before Connect")

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			require.NoError(t, conn.Connect(ctx))
			defer func() { _ = conn.Close(ctx) }()

			assert.Equal(t, tc.wantOffer, srv.lastOffer())
			assert.Equal(t, tc.wantVersion, conn.CBORVersion())

			res, err := conn.Send(ctx, "echo", int8(-5))
			require.NoError(t, err)
			var got any
			require.NoError(t, conn.GetUnmarshaler().Unmarshal(*res.Result, &got))
			assert.Equal(t, tc.wantEchoed, got)
			assert.Equal(t, tc.wantTaggedReq, strings.Contains(hex.EncodeToString(srv.lastRequest()), "d8b424"),
				"request %x", srv.lastRequest())
		})
	}
}

func newTestConnection(conf *connection.Config) *Connection {
	return New(conf)
}
