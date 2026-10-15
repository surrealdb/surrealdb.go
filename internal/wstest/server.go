// Package wstest provides a WebSocket server for testing how connections
// behave when the server goes away.
package wstest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	gorilla "github.com/gorilla/websocket"
)

// waitTimeout bounds how long the helpers wait, so that a test which would
// otherwise hang fails instead.
const waitTimeout = 5 * time.Second

// Server accepts WebSocket connections on /rpc, reads requests without
// answering them, and closes connections when told to.
type Server struct {
	*httptest.Server

	mu    sync.Mutex
	conns []*gorilla.Conn
	// accepted counts the connections accepted so far, and acceptedCh is
	// closed and replaced each time it grows.
	accepted   int
	acceptedCh chan struct{}
	// Requests receives a value for each request a connection sends.
	Requests chan struct{}
}

// NewServer starts a server, which is stopped when the test ends.
func NewServer(t *testing.T) *Server {
	t.Helper()
	s := &Server{Requests: make(chan struct{}, 64), acceptedCh: make(chan struct{})}
	upgrader := gorilla.Upgrader{Subprotocols: []string{"cbor"}}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// The client may finish its handshake before this runs, so tests
		// wait for it with WaitAccepted before relying on the connection
		// being known, as GoAway does.
		s.mu.Lock()
		s.conns = append(s.conns, conn)
		s.accepted++
		close(s.acceptedCh)
		s.acceptedCh = make(chan struct{})
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

// Accepted returns how many connections the server has accepted so far.
func (s *Server) Accepted() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.accepted
}

// WaitAccepted waits until the server has accepted n connections in all, and
// fails the test if it does not soon. A client's handshake can complete before
// the server records the connection.
func (s *Server) WaitAccepted(t testing.TB, n int) {
	t.Helper()
	timeout := time.After(waitTimeout)
	for {
		s.mu.Lock()
		accepted, ch := s.accepted, s.acceptedCh
		s.mu.Unlock()
		if accepted >= n {
			return
		}
		select {
		case <-ch:
		case <-timeout:
			t.Fatalf("the server accepted %d connections, not %d", accepted, n)
		}
	}
}

// GoAway sends a close frame on every connection, as a server shutting down
// does, and closes them. It closes only the connections the server has
// accepted: see WaitAccepted.
func (s *Server) GoAway() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.conns {
		_ = c.WriteMessage(gorilla.CloseMessage, gorilla.FormatCloseMessage(gorilla.CloseGoingAway, "shutting down"))
		_ = c.Close()
	}
	s.conns = nil
}

// Recv receives from ch, and fails the test if nothing comes soon.
func Recv[T any](t testing.TB, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(waitTimeout):
		t.Fatalf("timed out waiting for %s", what)
	}
	var zero T
	return zero
}
