package gws

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/lxzan/gws"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/internal/rand"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/rpc"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

type State int

type Connection struct {
	connection.Toolkit

	conn     *gws.Conn
	connLock sync.Mutex

	Timeout time.Duration

	closeCh    chan struct{}
	closeError error
	closed     bool

	handler *websocketHandler
}

var _ connection.Connection = (*Connection)(nil)

type websocketHandler struct {
	conn *Connection
}

func (h *websocketHandler) OnOpen(socket *gws.Conn) {
	// Connection opened successfully
}

// Received a close frame from the other end of the network connection, or disconnected voluntarily due to an error in the IO process
// In the former case, err can be asserted as *gws.CloseError.
func (h *websocketHandler) OnClose(socket *gws.Conn, err error) {
	h.conn.connLock.Lock()
	defer h.conn.connLock.Unlock()

	if h.conn.closeCh != nil {
		select {
		case <-h.conn.closeCh:
			// Already closed
		default:
			close(h.conn.closeCh)
			h.conn.closeError = err
		}
	}

	h.conn.closed = true
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
		c.Logger.Error(
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

		notificationChan, ok := c.GetNotificationChannel(notification.ID.String())
		if !ok {
			return
		}

		notificationChan <- notification
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
	if c.closeCh != nil {
		select {
		case <-c.closeCh:
			// Already closed
		default:
			close(c.closeCh)
		}
	}

	err := c.conn.WriteClose(constants.CloseMessageCode, []byte(""))
	if err != nil {
		c.Logger.Error("failed to close WebSocket connection", "error", err)
	}

	if err := c.conn.NetConn().Close(); err != nil {
		c.Logger.Error("failed to close underlying network connection", "error", err)
	}
	c.conn = nil

	return nil
}

func (c *Connection) IsClosed() bool {
	return c.closed
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
			c.Logger.Error("failed to close response body", "error", closeErr)
		}
	}
	if err != nil {
		return err
	}

	c.connLock.Lock()
	defer c.connLock.Unlock()

	c.conn = conn
	c.closeCh = make(chan struct{})

	// Start reading messages
	go conn.ReadLoop()

	return nil
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

func (c *Connection) Invalidate(ctx context.Context) error {
	return rpc.Invalidate(c, ctx)
}

// LiveNotifications implements connection.Connection.
func (c *Connection) LiveNotifications(id string) (chan connection.Notification, error) {
	return c.CreateNotificationChannel(id)
}

// Send implements connection.Connection.
func (c *Connection) Send(ctx context.Context, method string, params ...any) (*connection.RPCResponse[cbor.RawMessage], error) {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	select {
	case <-c.closeCh:
		return nil, c.closeError
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	id := rand.NewRequestID(constants.RequestIDLength)
	request := &connection.RPCRequest{
		ID:     id,
		Method: method,
		Params: params,
	}

	responseChan, err := c.CreateResponseChannel(id)
	if err != nil {
		return nil, err
	}
	defer c.RemoveResponseChannel(id)

	if err := c.write(request); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
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
