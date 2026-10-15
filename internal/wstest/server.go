// Package wstest provides a WebSocket server for testing how connections
// behave when the server goes away.
package wstest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	gorilla "github.com/gorilla/websocket"
)

// Server accepts WebSocket connections on /rpc, reads requests without
// answering them, and closes connections when told to.
type Server struct {
	*httptest.Server

	mu    sync.Mutex
	conns []*gorilla.Conn
	// Requests receives a value for each request a connection sends.
	Requests chan struct{}
}

// NewServer starts a server, which is stopped when the test ends.
func NewServer(t *testing.T) *Server {
	t.Helper()
	s := &Server{Requests: make(chan struct{}, 64)}
	upgrader := gorilla.Upgrader{Subprotocols: []string{"cbor"}}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conns = append(s.conns, conn)
		s.mu.Unlock()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
			s.Requests <- struct{}{}
		}
	}))
	t.Cleanup(s.Close)
	return s
}

// WSURL returns the server's WebSocket base URL.
func (s *Server) WSURL() string {
	return "ws" + strings.TrimPrefix(s.URL, "http")
}

// GoAway sends a close frame on every connection, as a server shutting down
// does, and closes them.
func (s *Server) GoAway() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.conns {
		_ = c.WriteMessage(gorilla.CloseMessage, gorilla.FormatCloseMessage(gorilla.CloseGoingAway, "shutting down"))
		_ = c.Close()
	}
	s.conns = nil
}
