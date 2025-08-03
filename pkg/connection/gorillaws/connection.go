package gorillaws

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/codec"

	"github.com/surrealdb/surrealdb.go/internal/rand"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/rpc"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/logger"

	gorilla "github.com/gorilla/websocket"
)

// DefaultDialer is the default gorilla dialer used by the WebSocketConnection
//
// It uses the default gorilla dialer as of gorilla/websocket v1.5.0 with the following modifications:
// - EnableCompression is set to true
// - Subprotocols is set to ["cbor"]
var DefaultDialer = &gorilla.Dialer{
	Proxy:             gorilla.DefaultDialer.Proxy,
	HandshakeTimeout:  gorilla.DefaultDialer.HandshakeTimeout,
	EnableCompression: true,
	Subprotocols:      []string{"cbor"},
}

type Option func(ws *Connection) error

type Connection struct {
	connection.Toolkit

	Conn *gorilla.Conn
	// connLock is used to ensure that the Conn is not-nil when we try to read or write to it.
	//
	// This lock is meant to not taken while the entire reconnection process is happening,
	// but instead only when we try to read or write to the connection after a successful connection.
	// This is to avoid non-cancellable blocking on the connection read/write operations, like Send.
	connLock sync.Mutex

	// Timeout is the timeout for receiveing the RPC response after
	// you've successfully sent the request.
	//
	// If the timeout is reached, the Send method will return ErrTimeout.
	// You can set it to 0 to disable the timeout, and instead use context.Context and context.WithTimeout
	// to control the timeout. It will be useful if you want to avoid the overhead of wrapping the context
	// with a timeout.
	Timeout time.Duration

	Option []Option
	logger logger.Logger

	// connCloseCh signals that the connection is being closed.
	// It is used to stop the readLoop goroutine and prevent Send from writing to a closed (i.e. nil) connection.
	connCloseCh chan int

	connCloseError error

	// closed is used to indicate whether the connection is closed.
	// It is set to true when the connection is closed, and false otherwise.
	// Once this is set to true, it cannot be set to false again.
	//
	// To reconnect, you should create a new instance of Connection
	// and call Connect on it.
	closed bool
}

func New(p *connection.Config) *Connection {
	return &Connection{
		Toolkit: connection.Toolkit{
			BaseURL: p.BaseURL,

			Marshaler:   p.Marshaler,
			Unmarshaler: p.Unmarshaler,

			ResponseChannels:     make(map[string]chan connection.RPCResponse[cbor.RawMessage]),
			NotificationChannels: make(map[string]chan connection.Notification),
		},
		Timeout: constants.DefaultWSTimeout,
		logger:  logger.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

// IsClosed checks if the WebSocket connection is disconnected.
// This is useful to enable the consumer of WebSocketConnection
// to trigger reconnection attempts if the connection is disconnected unexpectedly.
func (c *Connection) IsClosed() bool {
	return c.closed
}

// Connect establishes the WebSocket connection to the SurrealDB server.
// This method must be called from tryConnecting to prevent
// multiple goroutines from trying to connect at the same time.
func (c *Connection) Connect(ctx context.Context) error {
	conn, res, err := DefaultDialer.DialContext(ctx, fmt.Sprintf("%s/rpc", c.BaseURL), nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Delaying the lock until this point reduces
	// the max time Send is blocked on the connLock negligible.
	c.connLock.Lock()
	defer c.connLock.Unlock()

	c.Conn = conn

	for _, option := range c.Option {
		if err := option(c); err != nil {
			return err
		}
	}

	c.connCloseCh = make(chan int)

	// Start a goroutine to read messages from the WebSocket connection.
	// This will run in the background and handle incoming messages,
	// until closeChan is closed, or a read error indicating
	// lost connection occurs.
	go c.readLoop()

	return nil
}

func (c *Connection) SetTimeOut(timeout time.Duration) *Connection {
	c.Option = append(c.Option, func(ws *Connection) error {
		ws.Timeout = timeout
		return nil
	})
	return c
}

// If path is empty it will use os.stdout/os.stderr
func (c *Connection) Logger(logData logger.Logger) *Connection {
	c.logger = logData
	return c
}

func (c *Connection) RawLogger(logData logger.Logger) *Connection {
	c.logger = logData
	return c
}

func (c *Connection) SetCompression(compress bool) *Connection {
	c.Option = append(c.Option, func(ws *Connection) error {
		ws.Conn.EnableWriteCompression(compress)
		return nil
	})
	return c
}

// Close closes the WebSocket connection and stops listening for incoming messages.
//
// The context parameter allows the caller to cancel the close operation if it takes too long.
// This is useful when the underlying network connection is unreliable.
// If the context is canceled, the connection will still be closed in the background.
//
// If you want to make the close operation free of resource-leak as much as possible,
// you should provide a context with a timeout/deadline.
//
// We then propagate the deadline to the WebSocket close message write operation,
// which enables us to clean up everything including the internal goroutine that used to
// try writing to the WebSocket connection, when this function exists.
func (c *Connection) Close(ctx context.Context) error {
	if c.IsClosed() {
		return nil
	}

	// Signal that we're closing so that the goroutine reading from the connection
	// can stop reading messages and exit.
	//
	// TODO: This might not be necessary, because the gorilla.Conn.Close() method
	// will close the connection and that would result in the ReadMessage call in
	// ws.initialize() goroutine to return an error, which will stop the goroutine.
	//
	// This is to prevent concurrent Send fail before trying to lock connLock
	// and try writing a message.
	//
	// This also serves as a guardrail to prevent Send proceeding to write to nil ws.Conn
	close(c.connCloseCh)

	// We defer locking connLock until this point and do state check
	// to prevent Close blocking on repeated Close calls.
	c.connLock.Lock()
	defer c.connLock.Unlock()

	conn := c.Conn
	c.Conn = nil

	// Phase 1: Try to send the close message
	//
	// We assume this is important to let the server know that we're closing the connection.
	// If the write fails, we still try to close the connection locally,
	// so that we don't leak resources locally.

	writeErr := make(chan error, 1)

	go func() {
		// Set write deadline based on context to prevent indefinite blocking
		if deadline, ok := ctx.Deadline(); ok {
			err := conn.SetWriteDeadline(deadline)
			if err != nil {
				writeErr <- fmt.Errorf("BUG: WebSocketConnection.Close: failed to set write deadline, although it must always succeed: %w", err)
				return
			}
			defer func() {
				err := conn.SetWriteDeadline(time.Time{})
				if err != nil {
					writeErr <- fmt.Errorf("BUG: WebSocketConnection.Close: failed to reset write deadline, although it must always succeed: %w", err)
					return
				}
			}()
		}

		err := conn.WriteMessage(gorilla.CloseMessage, gorilla.FormatCloseMessage(constants.CloseMessageCode, ""))

		// Try to send the error, but also check if we should abandon the attempt
		select {
		case writeErr <- err:
		case <-ctx.Done():
			// TODO: This may not be absolutely necessary,
			// because WriteMessage would fail after we call ws.Conn.Close().
			// For now, it's here to be extra cautious and to ensure we don't leave the goroutine hanging.
		}
	}()

	select {
	case err := <-writeErr:
		if err != nil {
			// Write failed, but we don't return here,
			// because we try our best to Close the connection anyway,
			// although it might not be a clean close from the server's perspective.
			c.logger.Error("failed to write close message", "error", err)
		}
	case <-ctx.Done():
		// Again, we don't return here, because we try our best to Close the connection anyway,
		// although it might not be a clean close from the server's perspective.
	}

	// Phase 2: Close the underlying connection.
	//
	// We assume the Close method of the gorilla.Conn is an instantaneous operation,
	// so we don't need to consider the context here, even
	// in case the context is already canceled.
	//
	// We do this regardless of whether the write of the close message succeeded or not,
	// because we want to ensure the local resources are cleaned up anyway.
	// The lack of a close message write might result in the server not knowing
	// that the client is closing the connection in a timely manner,
	// we can't do much about it given we already failed to write it.

	return conn.Close()
}

func (c *Connection) Use(ctx context.Context, namespace, database string) error {
	return connection.Send[any](c, ctx, nil, "use", namespace, database)
}

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

func (c *Connection) Invalidate(ctx context.Context) error {
	return rpc.Invalidate(c, ctx)
}

func (c *Connection) Unset(ctx context.Context, key string) error {
	return connection.Send[any](c, ctx, nil, "unset", key)
}

func (c *Connection) GetUnmarshaler() codec.Unmarshaler {
	return c.Unmarshaler
}

// Send sends a request to SurrealDB and expects a response.
//
// The `ctx` is wrapped with a timeout if `ws.Timeout` is set.
// If you want to avoid this, for eliminating the overhead of wrapping the context,
// you can set `ws.Timeout` to 0.
//
// CAUTION: Although this function returns ErrTimeout in case the timeout is reached now,
// it will instead return context.DeadlineExceeded in upcoming versions of this SDK.
//
// The rationale is that it resulted in two different implementations of the Connection interface,
// HTTP and WebSocket, to behave differently in case of a timeout.
// The WebSocketConnection would return ErrTimeout, while the HTTPConnection would return context.DeadlineExceeded.
func (c *Connection) Send(ctx context.Context, method string, params ...any) (*connection.RPCResponse[cbor.RawMessage], error) {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	select {
	case <-c.connCloseCh:
		return nil, c.connCloseError
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
	case res, open := <-responseChan:
		if !open {
			return nil, errors.New("response channel closed")
		}

		if res.Error != nil {
			return nil, res.Error
		}
		return &res, nil
	}
}

func (c *Connection) write(v any) error {
	data, err := c.Marshaler.Marshal(v)
	if err != nil {
		return err
	}

	c.connLock.Lock()
	defer c.connLock.Unlock()
	err = c.Conn.WriteMessage(gorilla.BinaryMessage, data)

	// Check if we got ErrCloseSent, which means the connection is closed
	if errors.Is(err, gorilla.ErrCloseSent) {
		// Transition to disconnected state so IsClosed() returns true
		// This allows rews to detect the closed connection and reconnect
		c.closeWithError(err)
	}

	return err
}

func (c *Connection) closeWithError(err error) {
	if c.closed {
		return
	}

	c.closed = true
	c.connCloseError = err
	select {
	case <-c.connCloseCh:
		// Already closed
	default:
		close(c.connCloseCh)
	}
}

func (c *Connection) readLoop() {
	for {
		select {
		case <-c.connCloseCh:
			return
		default:
			_, data, err := c.Conn.ReadMessage()
			if err != nil {
				shouldExit := c.handleError(err)
				if shouldExit {
					c.closeWithError(err)
					return
				}
				continue
			}
			go c.handleResponse(data)
		}
	}
}

// handleError returns true if the error indicates that the connection is closed
// and the readLoop should exit, false otherwise.
func (c *Connection) handleError(err error) bool {
	if errors.Is(err, net.ErrClosed) {
		c.connCloseError = net.ErrClosed
		return true
	}
	if gorilla.IsUnexpectedCloseError(err) {
		c.connCloseError = io.ErrClosedPipe
		<-c.connCloseCh
		return true
	}

	c.logger.Error(err.Error())
	return false
}

func (c *Connection) handleResponse(res []byte) {
	var rpcRes connection.RPCResponse[cbor.RawMessage]
	if err := c.Unmarshaler.Unmarshal(res, &rpcRes); err != nil {
		panic(err)
	}

	if rpcRes.ID != nil && rpcRes.ID != "" {
		// Try to resolve message as response to query
		responseChan, ok := c.GetResponseChannel(fmt.Sprintf("%v", rpcRes.ID))
		if !ok {
			err := fmt.Errorf("unavailable ResponseChannel %+v", rpcRes.ID)
			c.logger.Error(err.Error())
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
		c.logger.Error(
			fmt.Sprintf("error in response: %v", rpcRes.Error),
			"result", fmt.Sprint(rpcRes.Result),
		)
		return
	} else {
		// todo: find a surefire way to confirm a notification

		notificationRes, err := rpcRes.Result.MarshalCBOR()
		if err != nil {
			c.logger.Error(
				fmt.Sprintf("error marshaling notification result: %v", err),
				"result", fmt.Sprint(rpcRes.Result),
			)
			return
		}

		var notification connection.Notification
		if err := c.Unmarshaler.Unmarshal(notificationRes, &notification); err != nil {
			c.logger.Error(
				fmt.Sprintf("error unmarshaling as notification: %v", err),
				"result", fmt.Sprint(rpcRes.Result),
			)
			return
		}

		channelID := notification.ID

		if channelID == nil {
			c.logger.Error(
				"response did not contain an 'id' field",
				"result", fmt.Sprint(rpcRes.Result),
			)
			return
		}

		LiveNotificationChan, ok := c.GetNotificationChannel(channelID.String())
		if !ok {
			c.logger.Error(
				fmt.Sprintf("unavailable ResponseChannel %+v", channelID.String()),
				"result", fmt.Sprint(rpcRes.Result),
			)
			return
		}

		LiveNotificationChan <- notification
	}
}
