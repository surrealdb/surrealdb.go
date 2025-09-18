// Package fakesdb provides a fake SurrealDB WebSocket server for testing purposes.
// It speaks the SurrealDB RPC protocol over WebSocket using CBOR encoding and includes
// various failure injection capabilities.
//
// We don't currently have an executable binary for this package,
// but it can be used as a library to create a fake SurrealDB server
// for integration tests.
//
// The WebSocket server is implemented using the `gws` library.
//
// To flexibly inject failures, you can configure stub responses
// that match specific RPC methods and parameters, along with failure configurations
// that specify how it fails (e.g., delays, invalid responses, TCP resets).
package fakesdb

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/lxzan/gws"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// cryptoRandInt generates a cryptographically secure random integer in [0, max)
func cryptoRandInt(rMax int) int {
	if rMax <= 0 {
		return 0
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(rMax)))
	return int(n.Int64())
}

// cryptoRandInt64 generates a cryptographically secure random int64 in [0, max)
func cryptoRandInt64(rMax int64) int64 {
	if rMax <= 0 {
		return 0
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(rMax))
	return n.Int64()
}

// cryptoRandFloat64 generates a cryptographically secure random float64 in [0.0, 1.0)
func cryptoRandFloat64() float64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(1<<53))
	return float64(n.Int64()) / float64(1<<53)
}

// FailureType represents the type of failure to inject during request processing
type FailureType string

const (
	// FailureNone indicates no failure injection
	FailureNone FailureType = "none"
	// FailureRequestDelay delays before processing the request
	FailureRequestDelay FailureType = "request_delay"
	// FailureResponseDelay delays the response (sent in background)
	FailureResponseDelay FailureType = "response_delay"
	// FailureInvalidResponse sends random binary data instead of valid response
	FailureInvalidResponse FailureType = "invalid_response"
	// FailureTCPTimeout sets TCP read/write deadline to immediate timeout
	FailureTCPTimeout FailureType = "tcp_timeout"
	// FailureTCPReset forcefully resets the TCP connection
	FailureTCPReset FailureType = "tcp_reset"
	// FailureWebSocketClose sends WebSocket close frame with configurable code/reason
	FailureWebSocketClose FailureType = "websocket_close"
	// FailureRandomDelay applies random delay up to 5 seconds
	FailureRandomDelay FailureType = "random_delay"
	// FailureDropConnection immediately closes the underlying network connection
	FailureDropConnection FailureType = "drop_connection"
	// FailurePartialMessage sends only half of the response message
	FailurePartialMessage FailureType = "partial_message"
	// FailureCorruptedMessage corrupts random bytes in the response
	FailureCorruptedMessage FailureType = "corrupted_message"
)

// RequestMatcher defines criteria for matching incoming RPC requests.
// It can match by method name and optionally by parameter values.
type RequestMatcher struct {
	// Method is the RPC method name to match
	Method string
	// Matcher is an optional function to match based on request parameters.
	// If nil, only the method name is used for matching.
	Matcher func(params []any) bool
}

// StubResponse defines a pre-configured RPC response for matching requests.
// It can return either a successful result or an error, and optionally
// inject failures during processing.
type StubResponse struct {
	// Matcher determines which requests this stub should handle
	Matcher RequestMatcher
	// Result is the successful RPCResponse result to return (mutually exclusive with Error)
	Result any
	// Error is the error to return (mutually exclusive with Result)
	Error *connection.RPCError
	// Failures defines failure injection configurations for this response
	Failures []FailureConfig
}

// FailureConfig defines how and when to inject a specific failure type
type FailureConfig struct {
	// Type specifies the type of failure to inject
	Type FailureType
	// Probability of triggering this failure (0.0 to 1.0)
	Probability float64
	// MinDelay is the minimum delay for delay-based failures
	MinDelay time.Duration
	// MaxDelay is the maximum delay for delay-based failures
	MaxDelay time.Duration
	// CloseCode is the WebSocket close code for FailureWebSocketClose
	CloseCode uint16
	// CloseReason is the WebSocket close reason for FailureWebSocketClose
	CloseReason string
}

// AuthType represents the type of authentication used in a session
type AuthType string

const (
	// AuthTypeToken indicates token-based authentication
	AuthTypeToken AuthType = "token"
	// AuthTypePassword indicates password-based authentication
	AuthTypePassword AuthType = "password"
)

// Session represents an authenticated connection session with namespace/database context
type Session struct {
	// ID is the unique session identifier
	ID string
	// Namespace is the SurrealDB namespace for this session
	Namespace string
	// Database is the SurrealDB database for this session
	Database string
	// AuthType indicates how the session was authenticated
	AuthType AuthType
	// Token is the authentication token for this session
	Token string
	// Username is the authenticated user's name
	Username string
	// ExpiresAt is when the session expires (nil means no expiration)
	ExpiresAt *time.Time // nil means no expiration
	// Variables can be set using `Let` RPC method
	// and unset using `Unset` RPC method
	Vars map[string]any
}

// Server is a fake SurrealDB WebSocket server that implements the RPC protocol
// with support for stub responses and failure injection
type Server struct {
	addr           string
	listener       net.Listener
	server         *gws.Server
	mu             sync.RWMutex
	stubResponses  []StubResponse
	globalFailures []FailureConfig
	connections    map[*gws.Conn]bool
	connSessions   map[*gws.Conn]*Session
	sessions       []*Session
	ctx            context.Context
	cancel         context.CancelFunc
	marshaler      codec.Marshaler
	unmarshaler    codec.Unmarshaler

	// TokenSignUp is the token returned by any successful SignUp operation
	// This is used to verify that the SignUp operation works correctly.
	TokenSignUp string

	// TokenSignIn is the token returned by any successful SignIn operation
	// This is used to verify that the SignIn operation works correctly.
	TokenSignIn string

	// sessionIdCounter is used to generate unique session IDs
	sessionIdCounter int
}

// Handler implements the gws.Handler interface for WebSocket connections
type Handler struct {
	server *Server
}

// NewServer creates a new fake SurrealDB server.
// Use "127.0.0.1:0" to bind to a random available port.
func NewServer(addr string) *Server {
	ctx, cancel := context.WithCancel(context.Background())

	c := surrealcbor.New()

	s := &Server{
		addr:         addr,
		connections:  make(map[*gws.Conn]bool),
		connSessions: make(map[*gws.Conn]*Session),
		sessions:     make([]*Session, 0),
		ctx:          ctx,
		cancel:       cancel,
		marshaler:    c,
		unmarshaler:  c,
	}

	handler := &Handler{server: s}
	s.server = gws.NewServer(handler, &gws.ServerOption{
		// Don't enforce sub-protocol for testing flexibility
	})
	s.server.OnError = func(_ net.Conn, err error) {
		if !errors.Is(err, net.ErrClosed) && !isUseOfClosedNetworkError(err) {
			log.Printf("Server error: %v", err)
		}
	}

	return s
}

// AddStubResponse adds a stub response configuration to the server.
// Stub responses are matched in the order they were added.
func (s *Server) AddStubResponse(stub StubResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stubResponses = append(s.stubResponses, stub)
}

// GenerateTokenWithExpiration creates a new token session with specified expiration.
// This is useful for testing authentication flows and token expiration scenarios.
func (s *Server) GenerateTokenWithExpiration(username, token string, duration time.Duration) (string, error) {
	if token == "" {
		return "", fmt.Errorf("GenerateTokenWithExpiration: token cannot be empty")
	}

	expiresAt := time.Now().Add(duration)
	session := &Session{
		ID:        fmt.Sprintf("session_%d", s.sessionIdCounter),
		AuthType:  AuthTypeToken,
		Token:     token,
		Username:  username,
		ExpiresAt: &expiresAt,
	}

	s.mu.Lock()
	s.sessions = append(s.sessions, session)
	s.sessionIdCounter++
	s.mu.Unlock()

	return token, nil
}

// SetGlobalFailures sets failure configurations that apply to all requests.
// These are checked before stub-specific failures.
func (s *Server) SetGlobalFailures(failures []FailureConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.globalFailures = failures
}

// Start starts the server and begins accepting WebSocket connections.
// Returns an error if the server cannot bind to the specified address.
func (s *Server) Start() error {
	var lc net.ListenConfig
	listener, err := lc.Listen(context.Background(), "tcp", s.addr)
	if err != nil {
		return err
	}
	s.listener = listener

	go func() {
		if err := s.server.RunListener(listener); err != nil {
			// Ignore "use of closed network connection" errors which are expected on shutdown
			if !errors.Is(err, net.ErrClosed) && !isUseOfClosedNetworkError(err) {
				log.Printf("Server error: %v", err)
			}
		}
	}()

	return nil
}

// Stop gracefully shuts down the server and closes all connections
func (s *Server) Stop() error {
	s.cancel()
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// Address returns the actual address the server is listening on.
// This is useful when using "127.0.0.1:0" to get the assigned port.
func (s *Server) Address() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.addr
}

func (h *Handler) OnOpen(socket *gws.Conn) {
	h.server.mu.Lock()
	h.server.connections[socket] = true
	// Connection starts without a session until Use is called
	h.server.mu.Unlock()
}

func (h *Handler) OnClose(socket *gws.Conn, err error) {
	h.server.mu.Lock()
	delete(h.server.connSessions, socket)
	delete(h.server.connections, socket)
	h.server.mu.Unlock()
}

func (h *Handler) OnPing(socket *gws.Conn, payload []byte) {
	if err := socket.WritePong(payload); err != nil {
		log.Printf("Error writing Pong: %v", err)
	}
}

func (h *Handler) OnPong(socket *gws.Conn, payload []byte) {
}

//nolint:gocyclo,funlen
func (h *Handler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()

	h.server.mu.RLock()
	globalFailures := h.server.globalFailures
	h.server.mu.RUnlock()

	for _, failure := range globalFailures {
		if shouldTriggerFailure(failure.Probability) {
			if err := h.applyFailure(socket, failure, nil, nil); err != nil {
				return
			}
		}
	}

	var req connection.RPCRequest
	if err := h.server.unmarshaler.Unmarshal(message.Bytes(), &req); err != nil {
		h.sendError(socket, nil, -32700, "Parse error")
		return
	}

	// Check for stubbed responses first
	h.server.mu.RLock()
	var matchedStub *StubResponse
	for _, stub := range h.server.stubResponses {
		if stub.Matcher.Method == req.Method {
			if stub.Matcher.Matcher == nil || stub.Matcher.Matcher(req.Params) {
				matchedStub = &stub
				break
			}
		}
	}
	h.server.mu.RUnlock()

	// Handle authentication-related methods with default behavior
	switch req.Method {
	case "use":
		h.handleUse(socket, &req)
		return
	case "signin":
		h.handleSignIn(socket, &req)
		return
	case "signup":
		h.handleSignUp(socket, &req)
		return
	case "authenticate":
		h.handleAuthenticate(socket, &req)
		return
	case "let":
		h.handleLet(socket, &req)
		return
	case "unset":
		h.handleUnset(socket, &req)
		return
	}

	// For other methods, check if namespace/database is set and authenticated
	h.server.mu.RLock()
	session, authenticated := h.server.connSessions[socket]
	h.server.mu.RUnlock()

	if !authenticated {
		h.sendError(socket, req.ID, -32000, "There was a problem with the database: There was a problem with authentication: Session not found")
		return
	}

	if session.Namespace == "" || session.Database == "" {
		h.sendError(socket, req.ID, -32000,
			"There was a problem with the database: There was a problem with authentication: Specify a namespace and database",
		)
		return
	}

	if session.Username == "" {
		h.sendError(socket, req.ID, -32000, "There was a problem with the database: There was a problem with authentication: Not signed in")
		return
	}

	// Check if session is expired
	if session.ExpiresAt != nil && time.Now().After(*session.ExpiresAt) {
		h.sendError(socket, req.ID, -32000, "There was a problem with the database: There was a problem with authentication: Expired")
		return
	}

	// If we have a stub, use it regardless of the method
	if matchedStub != nil {
		for _, failure := range matchedStub.Failures {
			if shouldTriggerFailure(failure.Probability) {
				if err := h.applyFailure(socket, failure, &req, matchedStub); err != nil {
					return
				}
			}
		}

		if matchedStub.Error != nil {
			h.sendError(socket, req.ID, matchedStub.Error.Code, matchedStub.Error.Message)
		} else {
			h.sendResponse(socket, req.ID, matchedStub.Result)
		}
		return
	}

	// If no stub was found, return a default response
	h.sendResponse(socket, req.ID, map[string]any{
		"default": "response",
		"method":  req.Method,
		"params":  req.Params,
	})
}

//nolint:gocyclo,funlen
func (h *Handler) applyFailure(socket *gws.Conn, failure FailureConfig, req *connection.RPCRequest, stub *StubResponse) error {
	switch failure.Type {
	case FailureRequestDelay:
		delay := randomDuration(failure.MinDelay, failure.MaxDelay)
		time.Sleep(delay)

	case FailureResponseDelay:
		go func() {
			delay := randomDuration(failure.MinDelay, failure.MaxDelay)
			time.Sleep(delay)
			if req != nil && stub != nil {
				if stub.Error != nil {
					h.sendError(socket, req.ID, stub.Error.Code, stub.Error.Message)
				} else {
					h.sendResponse(socket, req.ID, stub.Result)
				}
			}
		}()
		return fmt.Errorf("response delayed")

	case FailureInvalidResponse:
		data := make([]byte, 100)
		if _, err := rand.Read(data); err != nil {
			log.Printf("Error generating invalid response: %v", err)
		}
		if err := socket.WriteMessage(gws.OpcodeBinary, data); err != nil {
			log.Printf("Error writing invalid response: %v", err)
		}
		return fmt.Errorf("invalid response sent")

	case FailureTCPTimeout:
		conn := socket.NetConn()
		if err := conn.SetReadDeadline(time.Now()); err != nil {
			log.Printf("Error setting TCP read deadline: %v", err)
		}
		if err := conn.SetWriteDeadline(time.Now()); err != nil {
			log.Printf("Error setting TCP write deadline: %v", err)
		}
		return fmt.Errorf("tcp timeout")

	case FailureTCPReset:
		conn := socket.NetConn()
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			if err := tcpConn.SetLinger(0); err != nil {
				log.Printf("Error setting TCP linger: %v", err)
			}
		}
		conn.Close()
		return fmt.Errorf("tcp reset")

	case FailureWebSocketClose:
		code := failure.CloseCode
		if code == 0 {
			code = 1001
		}
		reason := failure.CloseReason
		if reason == "" {
			reason = "failure injection"
		}
		socket.WriteClose(code, []byte(reason))
		return fmt.Errorf("websocket close")

	case FailureRandomDelay:
		delay := time.Duration(cryptoRandInt64(int64(5 * time.Second)))
		time.Sleep(delay)

	case FailureDropConnection:
		socket.NetConn().Close()
		return fmt.Errorf("connection dropped")

	case FailurePartialMessage:
		if req != nil && stub != nil {
			var resp connection.RPCResponse[any]
			resp.ID = req.ID
			resp.Result = &stub.Result

			data, err := h.server.marshaler.Marshal(resp)
			if err != nil {
				log.Printf("Error marshaling partial message: %v", err)
				return fmt.Errorf("failed to send partial message: %w", err)
			}
			partialLen := len(data) / 2
			if err := socket.WriteMessage(gws.OpcodeBinary, data[:partialLen]); err != nil {
				log.Printf("Error writing partial message: %v", err)
				return fmt.Errorf("failed to send partial message: %w", err)
			}
			return fmt.Errorf("partial message sent")
		}

	case FailureCorruptedMessage:
		if req != nil && stub != nil {
			var resp connection.RPCResponse[any]
			resp.ID = req.ID
			resp.Result = &stub.Result

			data, err := h.server.marshaler.Marshal(resp)
			if err != nil {
				log.Printf("Error marshaling corrupted message: %v", err)
				return fmt.Errorf("failed to send corrupted message: %w", err)
			}
			for i := 0; i < len(data) && i < 10; i++ {
				data[cryptoRandInt(len(data))] = byte(cryptoRandInt(256))
			}
			if err := socket.WriteMessage(gws.OpcodeBinary, data); err != nil {
				log.Printf("Error writing corrupted message: %v", err)
				return fmt.Errorf("failed to send corrupted message: %w", err)
			}
			return fmt.Errorf("corrupted message sent")
		}
	}

	return nil
}

func (h *Handler) sendResponse(socket *gws.Conn, id, result any) {
	var resp connection.RPCResponse[any]
	resp.ID = id
	resp.Result = &result

	data, err := h.server.marshaler.Marshal(resp)
	if err != nil {
		h.sendError(socket, id, -32603, fmt.Sprintf("sendResponse: %v", err))
		return
	}

	if err := socket.WriteMessage(gws.OpcodeBinary, data); err != nil {
		log.Printf("Error writing response: %v", err)
		return
	}
}

func (h *Handler) sendError(socket *gws.Conn, id any, code int, message string) {
	var resp connection.RPCResponse[any]
	resp.ID = id
	resp.Error = &connection.RPCError{
		Code:    code,
		Message: message,
	}

	responseData, err := h.server.marshaler.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal error response: %v", err)
		return
	}

	if err := socket.WriteMessage(gws.OpcodeBinary, responseData); err != nil {
		log.Printf("Error writing error response: %v", err)
		return
	}
}

func shouldTriggerFailure(probability float64) bool {
	if probability <= 0 {
		return false
	}
	if probability >= 1 {
		return true
	}
	return cryptoRandFloat64() < probability
}

func randomDuration(dMin, dMax time.Duration) time.Duration {
	if dMin >= dMax {
		return dMin
	}
	return dMin + time.Duration(cryptoRandInt64(int64(dMax-dMin)))
}

// MatchMethod creates a RequestMatcher that matches only by method name
func MatchMethod(method string) RequestMatcher {
	return RequestMatcher{
		Method:  method,
		Matcher: nil,
	}
}

// MatchMethodWithParams creates a RequestMatcher that matches by method name
// and parameter values using a custom matcher function
func MatchMethodWithParams(method string, matcher func(params []any) bool) RequestMatcher {
	return RequestMatcher{
		Method:  method,
		Matcher: matcher,
	}
}

// SimpleStubResponse creates a basic stub response for a method without failure injection
func SimpleStubResponse(method string, response any) StubResponse {
	return StubResponse{
		Matcher: MatchMethod(method),
		Result:  response,
	}
}

// ErrorStubResponse creates a stub response that returns an RPC error
func ErrorStubResponse(method string, code int, message string) StubResponse {
	return StubResponse{
		Matcher: MatchMethod(method),
		Error: &connection.RPCError{
			Code:    code,
			Message: message,
		},
	}
}

func (h *Handler) handleUse(socket *gws.Conn, req *connection.RPCRequest) {
	if len(req.Params) < 2 {
		h.sendError(socket, req.ID, -32602, "handleUse: invalid params: use requires namespace and database parameters")
		return
	}

	namespace, ok := req.Params[0].(string)
	if !ok {
		h.sendError(socket, req.ID, -32602, "handleUse: invalid params: namespace must be a string")
		return
	}

	database, ok := req.Params[1].(string)
	if !ok {
		h.sendError(socket, req.ID, -32602, "handleUse: invalid params: database must be a string")
		return
	}

	h.server.mu.Lock()
	session := h.server.connSessions[socket]
	if session == nil {
		session = &Session{}
		h.server.connSessions[socket] = session
	}
	session.Namespace = namespace
	session.Database = database
	h.server.mu.Unlock()

	h.sendResponse(socket, req.ID, nil)
}

func (h *Handler) handleSignUp(socket *gws.Conn, req *connection.RPCRequest) {
	if len(req.Params) < 1 {
		h.sendError(socket, req.ID, -32602, "handleSignUp: invalid params: signup requires auth data")
		return
	}

	// Extract username from auth data if available
	username := ""
	ns := ""
	if authData, ok := req.Params[0].(map[string]any); ok {
		if user, ok := authData["user"].(string); ok {
			username = user
		} else if user, ok := authData["username"].(string); ok {
			username = user
		}
		if n, ok := authData["NS"].(string); ok {
			ns = n
		}
	}
	if username == "" {
		h.sendError(socket, req.ID, -32602, "UpIn: Signin requires username in auth data")
		return
	}
	if ns == "" {
		h.sendError(socket, req.ID, -32602, "handleSignIn: Signin requires namespace in auth data")
		return
	}
	db := ""
	if session, ok := req.Params[0].(map[any]any); ok {
		if d, ok := session["DB"].(string); ok {
			db = d
		}
	}
	if db == "" {
		h.sendError(socket, req.ID, -32602, "handleSignIn: Signin requires database in auth data")
		return
	}

	token, err := h.server.GenerateTokenWithExpiration(username, h.server.TokenSignUp, 1*time.Hour)
	if err != nil {
		h.sendError(socket, req.ID, -32000, "handleSignUp: "+err.Error())
		return
	}

	h.server.mu.Lock()
	session := h.server.connSessions[socket]
	if session == nil {
		session = &Session{
			ID:        fmt.Sprintf("session_%x", h.server.sessionIdCounter),
			Namespace: ns,
			Database:  db,
			AuthType:  AuthTypeToken,
			Token:     token,
			Username:  username,
		}
		h.server.connSessions[socket] = session
	} else {
		session.Namespace = ns
		session.Database = db
		session.AuthType = AuthTypeToken
		session.Token = token
		session.Username = username
	}
	h.server.mu.Unlock()

	h.sendResponse(socket, req.ID, token)
}

func (h *Handler) handleSignIn(socket *gws.Conn, req *connection.RPCRequest) {
	if len(req.Params) < 1 {
		h.sendError(socket, req.ID, -32602, "handleSignIn: invalid params: signin requires auth data")
		return
	}

	// Extract username from auth data if available
	username := ""
	if authData, ok := req.Params[0].(map[string]any); ok {
		if user, ok := authData["user"].(string); ok {
			username = user
		} else if user, ok := authData["username"].(string); ok {
			username = user
		}
	}
	if username == "" {
		h.sendError(socket, req.ID, -32602, "handleSignIn: Signin requires username in auth data")
		return
	}

	token, err := h.server.GenerateTokenWithExpiration(username, h.server.TokenSignIn, 1*time.Hour)
	if err != nil {
		h.sendError(socket, req.ID, -32000, "handleSignIn: "+err.Error())
		h.server.mu.Unlock()
		return
	}

	h.server.mu.Lock()
	session := h.server.connSessions[socket]
	if session == nil {
		h.sendError(socket, req.ID, -32000, "handleSignIn: Specify a namespace and database to use")
		h.server.mu.Unlock()
		return
	}

	// Create authenticated session
	session.AuthType = AuthTypePassword
	session.Token = token
	session.Username = username
	session.ID = fmt.Sprintf("session_%x", h.server.sessionIdCounter)
	// Password auth doesn't expire
	session.ExpiresAt = nil

	// Add to global sessions
	h.server.sessions = append(h.server.sessions, session)
	h.server.sessionIdCounter++
	h.server.mu.Unlock()

	h.sendResponse(socket, req.ID, token)
}

func (h *Handler) handleAuthenticate(socket *gws.Conn, req *connection.RPCRequest) {
	if len(req.Params) < 1 {
		h.sendError(socket, req.ID, -32602, "handleAuthenticate: invalid params: authenticate requires token parameter")
		return
	}

	token, ok := req.Params[0].(string)
	if !ok {
		h.sendError(socket, req.ID, -32602, "handleAuthenticate: invalid params: token must be a string")
		return
	}

	h.server.mu.Lock()
	defer h.server.mu.Unlock()

	// Check if we have a session for this connection with namespace/database set
	connSession := h.server.connSessions[socket]
	if connSession == nil || connSession.Namespace == "" || connSession.Database == "" {
		h.sendError(socket, req.ID, -32000, "handleAuthenticate: Specify a namespace and database to use")
		return
	}

	// Find the session by token
	var foundSession *Session
	now := time.Now()
	for _, s := range h.server.sessions {
		if s.Token == token {
			// Check if token is expired
			if s.ExpiresAt != nil && now.After(*s.ExpiresAt) {
				h.sendError(socket, req.ID, -32000, "handleAuthenticate: Authentication failed: Token expired")
				return
			}
			foundSession = s
			break
		}
	}

	if foundSession == nil {
		h.sendError(socket, req.ID, -32000, "handleAuthenticate: Authentication failed: Not session found for token")
		return
	}

	// Update connection session with auth info
	connSession.AuthType = AuthTypeToken
	connSession.Token = token
	connSession.Username = foundSession.Username
	connSession.ID = foundSession.ID
	connSession.ExpiresAt = foundSession.ExpiresAt

	h.sendResponse(socket, req.ID, nil)
}

func (h *Handler) handleLet(socket *gws.Conn, req *connection.RPCRequest) {
	if len(req.Params) < 1 {
		h.sendError(socket, req.ID, -32602, "handleLet: invalid params: let requires key-value pairs")
		return
	}

	if len(req.Params)%2 != 0 {
		h.sendError(socket, req.ID, -32602, "handleLet: invalid params: let requires even number of parameters (key-value pairs)")
		return
	}

	h.server.mu.Lock()
	defer h.server.mu.Unlock()

	session := h.server.connSessions[socket]
	if session == nil {
		h.sendError(socket, req.ID, -32000, "handleLet: Specify a namespace and database to use")
		return
	}

	if session.Vars == nil {
		session.Vars = make(map[string]any)
	}

	for i := 0; i < len(req.Params); i += 2 {
		key, ok := req.Params[i].(string)
		if !ok {
			h.sendError(socket, req.ID, -32602, "handleLet: invalid params: let key must be a string")
			return
		}
		value := req.Params[i+1]
		session.Vars[key] = value
	}

	h.sendResponse(socket, req.ID, nil)
}

func (h *Handler) handleUnset(socket *gws.Conn, req *connection.RPCRequest) {
	if len(req.Params) < 1 {
		h.sendError(socket, req.ID, -32602, "handleUnset: invalid params: unset requires at least one key")
		return
	}

	h.server.mu.Lock()
	defer h.server.mu.Unlock()

	session := h.server.connSessions[socket]
	if session == nil {
		h.sendError(socket, req.ID, -32000, "handleUnset: Specify a namespace and database to use")
		return
	}

	for _, key := range req.Params {
		if keyStr, ok := key.(string); ok {
			delete(session.Vars, keyStr)
		} else {
			h.sendError(socket, req.ID, -32602, "handleUnset: invalid params: unset keys must be strings")
			return
		}
	}

	h.sendResponse(socket, req.ID, nil)
}

func isUseOfClosedNetworkError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return errStr == "use of closed network connection" ||
		errStr == "accept tcp 127.0.0.1:0: use of closed network connection" ||
		(len(errStr) > 30 && errStr[len(errStr)-30:] == "use of closed network connection")
}
