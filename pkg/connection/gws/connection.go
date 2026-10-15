package gws

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/lxzan/gws"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/internal/rand"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/rpc"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

type Connection struct {
	connection.Toolkit

	conn     *gws.Conn
	connLock sync.Mutex

	Timeout time.Duration

	// done is closed when the connection closes, by Close or because it was
	// lost, and closeError then says why.
	done       chan struct{}
	closeOnce  sync.Once
	closeMu    sync.RWMutex
	closeError error
	closed     atomic.Bool

	handler *websocketHandler
}

var (
	_ connection.Connection    = (*Connection)(nil)
	_ connection.CloseNotifier = (*Connection)(nil)
	_ connection.Redialer      = (*Connection)(nil)
)

type websocketHandler struct {
	conn *Connection
}

func (h *websocketHandler) OnOpen(socket *gws.Conn) {
	// Connection opened successfully
}

// Received a close frame from the other end of the network connection, or disconnected voluntarily due to an error in the IO process
// In the former case, err can be asserted as *gws.CloseError.
func (h *websocketHandler) OnClose(socket *gws.Conn, err error) {
	if err == nil {
		err = connection.ErrConnectionClosed
	}
	h.conn.markClosed(err)
}

// markClosed records that the connection closed, with err nil when Close was
// called. Only the first call counts.
func (c *Connection) markClosed(err error) {
	c.closeOnce.Do(func() {
		c.closeMu.Lock()
		c.closeError = err
		c.closeMu.Unlock()
		c.closed.Store(true)
		if c.done != nil {
			close(c.done)
		}
	})
}

// Closed returns a channel which is closed when the connection closes.
func (c *Connection) Closed() <-chan struct{} {
	return c.done
}

// CloseError returns why the connection closed: nil after Close.
func (c *Connection) CloseError() error {
	c.closeMu.RLock()
	defer c.closeMu.RUnlock()
	return c.closeError
}

// closedError is the error for a request on a closed connection.
func (c *Connection) closedError() error {
	if err := c.CloseError(); err != nil && !errors.Is(err, connection.ErrConnectionClosed) {
		return fmt.Errorf("%w: %w", connection.ErrConnectionClosed, err)
	}
	return connection.ErrConnectionClosed
}

// Redial returns a new connection to the same server, with the same
// configuration, for reconnecting after this one is lost.
func (c *Connection) Redial() connection.Connection {
	n := New(&connection.Config{
		BaseURL:     c.BaseURL,
		Marshaler:   c.Marshaler,
		Unmarshaler: c.Unmarshaler,
		Logger:      c.Logger,
	})
	n.Timeout = c.Timeout
	return n
}

func (h *websocketHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	h.conn.handleResponse(message.Bytes())
}

func (h *websocketHandler) OnPing(socket *gws.Conn, payload []byte) {
}

func (h *websocketHandler) OnPong(socket *gws.Conn, payload []byte) {
}

func (c *Connection) write(v any) error {
	data, err := c.Marshaler.Marshal(v)
	if err != nil {
		return err
	}

	c.connLock.Lock()
	defer c.connLock.Unlock()

	if c.conn == nil {
		return errors.New("connection is closed")
	}

	return c.conn.WriteMessage(gws.OpcodeBinary, data)
}

func (c *Connection) handleResponse(data []byte) {
	var rpcRes connection.RPCResponse[cbor.RawMessage]
	if err := c.Unmarshaler.Unmarshal(data, &rpcRes); err != nil {
		// This can happen when SurrealDB sent an invalid response (unlikely),
		// or the cbor library failed unmarshaling Result into even cbor.RawMessage due to
		// configurable limits like fxamacker/cbor.MaxArrayElements that applies to RPCResponse.Result (more likely).
		//
		// We log the error and return to prevent:
		// 1. DoS attacks from malformed CBOR data causing panics
		// 2. Memory exhaustion from falling back to default unmarshaler with no limits
		c.logError(
			"Failed to unmarshal response",
			"error", err,
		)
		return
	}

	if rpcRes.ID != nil && rpcRes.ID != "" {
		// Handle RPC response
		responseChan, ok := c.GetResponseChannel(fmt.Sprintf("%v", rpcRes.ID))
		if !ok {
			return
		}
		defer close(responseChan)
		responseChan <- rpcRes
	} else if rpcRes.Result == nil && rpcRes.Error != nil {
		// Some error cases result in the lack of an ID field in the response,
		// so we handle the error here,
		// rather than unintentionally treating it as a notification.
		// See https://github.com/surrealdb/surrealdb.go/issues/273

		// Note that we cannot send the response to the response channel,
		// because the response did not have an ID field,
		// so we cannot find the response channel to send it to.
		// Instead, we just log the error and return,
		// in the hope that the caller notices the programming error
		// by the RPC timing out, and the error being logged here.
		c.logError(
			fmt.Sprintf("error in response: %v", rpcRes.Error),
			"result", fmt.Sprint(rpcRes.Result),
		)
		return
	} else {
		// Handle notification
		notificationRes, err := rpcRes.Result.MarshalCBOR()
		if err != nil {
			return
		}

		var notification connection.Notification
		if err := c.Unmarshaler.Unmarshal(notificationRes, &notification); err != nil {
			return
		}

		if notification.ID == nil {
			return
		}

		if !c.SendNotification(notification.ID.String(), notification) {
			c.logError(
				fmt.Sprintf("unavailable NotificationChannel %+v", notification.ID.String()),
			)
		}
	}
}

// New creates a new WebSocket connection based on gws
func New(params *connection.Config) *Connection {
	conn := &Connection{
		Timeout: constants.DefaultWSTimeout,
	}
	conn.BaseURL = params.BaseURL
	conn.Marshaler = params.Marshaler
	conn.Unmarshaler = params.Unmarshaler
	conn.ResponseChannels = make(map[string]chan connection.RPCResponse[cbor.RawMessage])
	conn.NotificationChannels = make(map[string]chan connection.Notification)
	conn.Logger = params.Logger
	conn.done = make(chan struct{})

	return conn
}

// SetTimeout sets the timeout for RPC responses
func (c *Connection) SetTimeout(timeout time.Duration) *Connection {
	c.Timeout = timeout
	return c
}

// Close implements connection.Connection.
func (c *Connection) Close(ctx context.Context) error {
	c.connLock.Lock()
	defer c.connLock.Unlock()

	if c.conn == nil {
		return nil
	}

	// Signal closing
	c.markClosed(nil)

	err := c.conn.WriteClose(constants.CloseMessageCode, []byte(""))
	if err != nil {
		c.logError("failed to close WebSocket connection", "error", err)
	}

	if err := c.conn.NetConn().Close(); err != nil {
		c.logError("failed to close underlying network connection", "error", err)
	}
	c.conn = nil

	return nil
}

func (c *Connection) IsClosed() bool {
	return c.closed.Load()
}

// Connect tries to establish a WebSocket connection to SurrealDB.
// This method must be called after New and before any other operations.
//
// For simplicity, this method does not check if the connection is already being
// established, has been established, or has been closed.
// It will always try to establish a new connection.
// So, it is the caller's responsibility to ensure that
// this method is called only when the connection is not established yet.
//
// Do not call this method to reconnect already closed connections.
// To reconnect, you should create a new instance of Connection
// and call Connect on it.
//
// This is to prevent it to have many internal states that would lead to complexity and bugs.
// You should assume gws.Connection to have only three states:
// 1. Pending: gws.Connection is instantiated but not connected yet, or it is in the process of connecting.
// 2. Connected: gws.Connection is connected and ready to use.
// 3. Closed: gws.Connection is closed and cannot be used anymore.
//
// Once you reach 3, you can use IsClosed to check if the connection is closed,
// so that a high-level feature can be implemented to reconnect automatically.
func (c *Connection) Connect(ctx context.Context) error {
	c.handler = &websocketHandler{conn: c}

	option := &gws.ClientOption{
		Addr: fmt.Sprintf("%s/rpc", c.BaseURL),
		RequestHeader: http.Header{
			"Sec-WebSocket-Protocol": []string{"cbor"},
		},
		PermessageDeflate: gws.PermessageDeflate{
			Enabled: true,
		},
	}

	conn, resp, err := gws.NewClient(c.handler, option)
	if resp != nil {
		if closeErr := resp.Body.Close(); closeErr != nil {
			c.logError("failed to close response body", "error", closeErr)
		}
	}
	if err != nil {
		return err
	}

	c.connLock.Lock()
	defer c.connLock.Unlock()

	c.conn = conn

	// Start reading messages
	go conn.ReadLoop()

	return nil
}

// HTTPEndpoint returns the server's base URL and the HTTP client to reach it with.
func (c *Connection) HTTPEndpoint() (baseURL string, client *http.Client) {
	return c.BaseURL, nil
}

// Health asks the server whether it is healthy, with GET /health over HTTP.
func (c *Connection) Health(ctx context.Context) error {
	return connection.CheckHealth(ctx, nil, c.BaseURL)
}

// GetUnmarshaler implements connection.Connection.
func (c *Connection) GetUnmarshaler() codec.Unmarshaler {
	return c.Unmarshaler
}

// Let implements connection.Connection.
func (c *Connection) Let(ctx context.Context, key string, value any) error {
	return connection.Send[any](c, ctx, nil, "let", key, value)
}

func (c *Connection) Authenticate(ctx context.Context, token string) error {
	return rpc.Authenticate(c, ctx, token)
}

func (c *Connection) SignUp(ctx context.Context, authData any) (string, error) {
	return rpc.SignUp(c, ctx, authData)
}

func (c *Connection) SignIn(ctx context.Context, authData any) (string, error) {
	return rpc.SignIn(c, ctx, authData)
}

func (c *Connection) SignUpWithRefresh(ctx context.Context, authData any) (*connection.Tokens, error) {
	return rpc.SignUpWithRefresh(c, ctx, authData)
}

func (c *Connection) SignInWithRefresh(ctx context.Context, authData any) (*connection.Tokens, error) {
	return rpc.SignInWithRefresh(c, ctx, authData)
}

func (c *Connection) Invalidate(ctx context.Context) error {
	return rpc.Invalidate(c, ctx)
}

// LiveNotifications implements connection.Connection.
func (c *Connection) LiveNotifications(id string) (chan connection.Notification, error) {
	return c.CreateNotificationChannel(id)
}

// CloseLiveNotifications implements connection.Connection.
func (c *Connection) CloseLiveNotifications(id string) error {
	return c.Toolkit.CloseLiveNotifications(id)
}

// Send implements connection.Connection.
func (c *Connection) Send(ctx context.Context, method string, params ...any) (*connection.RPCResponse[cbor.RawMessage], error) {
	request := &connection.RPCRequest{
		Method: method,
		Params: params,
	}
	return c.Call(ctx, request)
}

// Call implements connection.Connection.
// Unlike Send, Call accepts an RPCRequest directly, allowing you to set
// Session and Txn fields for session-scoped or transaction-scoped operations (SurrealDB v3+).
func (c *Connection) Call(ctx context.Context, req *connection.RPCRequest) (*connection.RPCResponse[cbor.RawMessage], error) {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	select {
	case <-c.done:
		return nil, c.closedError()
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Set request ID if not already set
	id := req.ID
	if id == nil || id == "" {
		id = rand.NewRequestID(constants.RequestIDLength)
		req.ID = id
	}

	c.logDebug("Sending RPC request", "id", id, "method", req.Method)

	responseChan, err := c.CreateResponseChannel(fmt.Sprintf("%v", id))
	if err != nil {
		return nil, err
	}
	defer c.RemoveResponseChannel(fmt.Sprintf("%v", id))

	if err := c.write(req); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, c.closedError()
	case rpcRes, open := <-responseChan:
		if !open {
			return nil, errors.New("response channel closed")
		}

		if rpcRes.Error != nil {
			return nil, rpcRes.Error
		}
		return &rpcRes, nil
	}
}

// Unset implements connection.Connection.
func (c *Connection) Unset(ctx context.Context, key string) error {
	return connection.Send[any](c, ctx, nil, "unset", key)
}

// Use implements connection.Connection.
func (c *Connection) Use(ctx context.Context, namespace, database string) error {
	return connection.Send[any](c, ctx, nil, "use", namespace, database)
}

func (c *Connection) logDebug(msg string, args ...any) {
	if c.Logger != nil {
		c.Logger.Debug(msg, args...)
	}
}

func (c *Connection) logError(msg string, args ...any) {
	if c.Logger != nil {
		c.Logger.Error(msg, args...)
	}
}
