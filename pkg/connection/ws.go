package connection

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/codec"

	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go/internal/rand"
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

type Option func(ws *WebSocketConnection) error

const (
	// WebSocketStateUnknown indicates that the WebSocket connection is unknown.
	//
	// This is intentionally the zero value of WebSocketConnectionState,
	// so that we can use it as an indicator that WebSocketConnection has been
	// initialized in an unexpected way.
	WebSocketStateUnknown WebSocketConnectionState = iota
	// WebSocketStatePending indicates that the WebSocket connection is pending.
	//
	// This is the initial state of the WebSocketConnection before it has been connected.
	// It will transition to WebSocketStateConnecting once it starts connecting.
	//
	// To make the connection usable, you must call Connect to transition from this state
	// to WebSocketStateConnecting (and then to WebSocketStateConnected).
	WebSocketStatePending
	// WebSocketStateConnecting indicates that the WebSocket connection is in the process of connecting.
	//
	// It will transition to WebSocketStateConnected once the connection is established,
	// or to WebSocketStateDisconnected if the connection fails.
	WebSocketStateConnecting
	// WebSocketStateConnected indicates that the WebSocket connection is established and ready to use.
	//
	// It will transition to WebSocketStateDisconnected if the connection is closed manually or due to an error.
	WebSocketStateConnected
	// WebSocketStateDisconnecting indicates that the WebSocket connection is being manually disconnected.
	WebSocketStateDisconnecting
	// WebSocketStateDisconnected indicates that the WebSocket connection is closed or disconnected,
	// either manually or due to an error.
	//
	// It can transition to WebSocketStateConnecting if a reconnection attempt is made.
	WebSocketStateDisconnected
)

// WebSocketConnectionState represents the state of the WebSocket connection.
//
// We assume the following state transitions:
//
//	WebSocketStatePending
//	  -> WebSocketStateConnecting (Initial connection attempt)
//
//	WebSocketStateConnecting
//	  -> WebSocketStateConnected (Successful connection)
//	  -> WebSocketStateDisconnected (Failed connection attempt)
//
//	WebSocketStateConnected
//	  -> WebSocketStateDisconnecting (Manual disconnection attempt)
//	  -> WebSocketStateDisconnected (Disconnected by an error)
//
//	WebSocketStateDisconnecting
//	  -> WebSocketStateDisconnected (Graceful disconnection process completed)
//
//	WebSocketStateDisconnected
//	  -> WebSocketStateConnecting (Reconnection attempt)
//
// Any other states and transitions are considered invalid
// and may result in an error.
type WebSocketConnectionState int

type WebSocketConnection struct {
	BaseConnection

	Conn *gorilla.Conn
	// connLock is used to ensure that the Conn is not-nil when we try to read or write to it.
	//
	// This lock is meant to not taken while the entire reconnection process is happening,
	// but instead only when we try to read or write to the connection after a successful connection.
	// This is to avoid non-cancellable blocking on the connection read/write operations, like Send.
	connLock sync.Mutex

	// stateLock is used to ensure that we don't try to reconnect while we're already reconnecting.
	// This is intentionally a separate lock from connLock,
	// because we want to allow multiple goroutines to try read/write to an already failed connection
	// via Send, and receive errors immediately, without waiting dozens of seconds for the reconnection to finish.
	stateLock sync.RWMutex

	// Timeout is the timeout for receiveing the RPC response after
	// you've successfully sent the request.
	//
	// If the timeout is reached, the Send method will return ErrTimeout.
	// You can set it to 0 to disable the timeout, and instead use context.Context and context.WithTimeout
	// to control the timeout. It will be useful if you want to avoid the overhead of wrapping the context
	// with a timeout.
	Timeout time.Duration

	state WebSocketConnectionState

	Option []Option
	logger logger.Logger

	// connCloseCh signals that the connection is being closed.
	// It is used to stop the readLoop goroutine and prevent Send from writing to a closed (i.e. nil) connection.
	connCloseCh chan int

	connCloseError error
}

func NewWebSocketConnection(p NewConnectionParams) *WebSocketConnection {
	return &WebSocketConnection{
		BaseConnection: BaseConnection{
			BaseURL: p.BaseURL,

			Marshaler:   p.Marshaler,
			Unmarshaler: p.Unmarshaler,

			ResponseChannels:     make(map[string]chan RPCResponse[cbor.RawMessage]),
			NotificationChannels: make(map[string]chan Notification),
		},
		Timeout: constants.DefaultWSTimeout,
		logger:  logger.New(slog.NewJSONHandler(os.Stdout, nil)),
		state:   WebSocketStatePending,
	}
}

func (ws *WebSocketConnection) Connect(ctx context.Context) error {
	if err := ws.PreConnectionChecks(); err != nil {
		return err
	}

	return ws.tryConnecting(ctx)
}

// IsDisconnected checks if the WebSocket connection is disconnected.
// This is useful to enable the consumer of WebSocketConnection
// to trigger reconnection attempts if the connection is disconnected unexpectedly.
func (ws *WebSocketConnection) IsDisconnected() bool {
	ws.stateLock.RLock()
	defer ws.stateLock.RUnlock()

	return ws.state == WebSocketStateDisconnected
}

func (ws *WebSocketConnection) transitionToConnecting() error {
	ws.stateLock.Lock()
	defer ws.stateLock.Unlock()

	switch ws.state {
	case WebSocketStateConnected:
		ws.logger.Debug("WebSocketConnection is already connected, skipping reconnection")
		return errors.New("WebSocketConnection is already connected")
	case WebSocketStateConnecting:
		ws.logger.Debug("WebSocketConnection is already connecting, skipping reconnection")
		return errors.New("WebSocketConnection is already connecting")
	case WebSocketStateDisconnected:
		ws.logger.Debug("WebSocketConnection is disconnected, trying to reconnect")
	case WebSocketStatePending:
		ws.logger.Debug("WebSocketConnection is pending, trying to connect")
	default:
		ws.logger.Warn("BUG: WebSocketConnection is in an unknown state, trying to reconnect anyway",
			"state", ws.state,
		)
	}

	ws.state = WebSocketStateConnecting

	return nil
}

func (ws *WebSocketConnection) transitionToDisconnecting() error {
	ws.stateLock.Lock()
	defer ws.stateLock.Unlock()

	switch ws.state {
	case WebSocketStateConnected:
		ws.logger.Debug("WebSocketConnection is connected, trying to disconnect")
	case WebSocketStateConnecting:
		ws.logger.Debug("WebSocketConnection is connecting, but we cannot disconnect until it is connected")
		return errors.New("WebSocketConnection is connecting, cannot disconnect")
	case WebSocketStateDisconnected:
		ws.logger.Debug("WebSocketConnection is already disconnected, skipping disconnection")
		return errors.New("WebSocketConnection is already disconnected")
	case WebSocketStatePending:
		ws.logger.Debug("WebSocketConnection is pending, no need to disconnect")
		return errors.New("WebSocketConnection is pending, no need to disconnect")
	default:
		ws.logger.Warn("BUG: WebSocketConnection is in an unknown state, nothing to do",
			"state", ws.state,
		)
		return errors.New("WebSocketConnection is in an unknown state, nothing to do")
	}

	ws.state = WebSocketStateDisconnecting

	return nil
}

func (ws *WebSocketConnection) tryConnecting(ctx context.Context) error {
	if err := ws.transitionToConnecting(); err != nil {
		return err
	}

	if err := ws.connect(ctx); err != nil {
		ws.state = WebSocketStateDisconnected
		ws.logger.Error("failed to connect WebSocketConnection", "error", err)
		return err
	}

	ws.state = WebSocketStateConnected
	ws.logger.Debug("WebSocketConnection is connected")

	return nil
}

// connect establishes the WebSocket connection to the SurrealDB server.
// This method must be called from tryConnecting to prevent
// multiple goroutines from trying to connect at the same time.
func (ws *WebSocketConnection) connect(ctx context.Context) error {
	connection, res, err := DefaultDialer.DialContext(ctx, fmt.Sprintf("%s/rpc", ws.BaseURL), nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Delaying the lock until this point reduces
	// the max time Send is blocked on the connLock negligible.
	ws.connLock.Lock()
	defer ws.connLock.Unlock()

	ws.Conn = connection

	for _, option := range ws.Option {
		if err := option(ws); err != nil {
			return err
		}
	}

	ws.connCloseCh = make(chan int)

	// Start a goroutine to read messages from the WebSocket connection.
	// This will run in the background and handle incoming messages,
	// until closeChan is closed, or a read error indicating
	// lost connection occurs.
	go ws.readLoop()

	return nil
}

func (ws *WebSocketConnection) SetTimeOut(timeout time.Duration) *WebSocketConnection {
	ws.Option = append(ws.Option, func(ws *WebSocketConnection) error {
		ws.Timeout = timeout
		return nil
	})
	return ws
}

// If path is empty it will use os.stdout/os.stderr
func (ws *WebSocketConnection) Logger(logData logger.Logger) *WebSocketConnection {
	ws.logger = logData
	return ws
}

func (ws *WebSocketConnection) RawLogger(logData logger.Logger) *WebSocketConnection {
	ws.logger = logData
	return ws
}

func (ws *WebSocketConnection) SetCompression(compress bool) *WebSocketConnection {
	ws.Option = append(ws.Option, func(ws *WebSocketConnection) error {
		ws.Conn.EnableWriteCompression(compress)
		return nil
	})
	return ws
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
func (ws *WebSocketConnection) Close(ctx context.Context) error {
	if err := ws.transitionToDisconnecting(); err != nil {
		return err
	}
	defer func() {
		// We assume the connection is disconnected anyway,
		// regardless of whether the write of the close message succeeded or not,
		// or the connection close succeeded or not.
		//
		// It may theoretically result in a resource leak in the lower layers of the system,
		// like the OS or the network stack.
		//
		// But we accept this risk, because we want to prioritize enabling the caller to
		// choose reconnecting in the hope that the connection will be re-established successfully,
		// reducing the downtime.

		// Also note that we have no need to lock the stateLock here,
		// because we already locked it in transitionToDisconnecting,
		// and while WebSocketStateDisconnecting is set,
		// no other goroutine can try to connect or disconnect.
		ws.state = WebSocketStateDisconnected
	}()

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
	close(ws.connCloseCh)

	// We defer locking connLock until this point and do state check
	// to prevent Close blocking on repeated Close calls.
	ws.connLock.Lock()
	defer ws.connLock.Unlock()

	conn := ws.Conn
	ws.Conn = nil

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
			ws.logger.Error("failed to write close message", "error", err)
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

func (ws *WebSocketConnection) Use(ctx context.Context, namespace, database string) error {
	return ws.Send(ctx, nil, "use", namespace, database)
}

func (ws *WebSocketConnection) Let(ctx context.Context, key string, value interface{}) error {
	return ws.Send(ctx, nil, "let", key, value)
}

func (ws *WebSocketConnection) Unset(ctx context.Context, key string) error {
	return ws.Send(ctx, nil, "unset", key)
}

func (ws *WebSocketConnection) GetUnmarshaler() codec.Unmarshaler {
	return ws.Unmarshaler
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
func (ws *WebSocketConnection) Send(ctx context.Context, dest interface{}, method string, params ...interface{}) error {
	if ws.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, ws.Timeout)
		defer cancel()
	}

	select {
	case <-ws.connCloseCh:
		return ws.connCloseError
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	id := rand.String(constants.RequestIDLength)
	request := &RPCRequest{
		ID:     id,
		Method: method,
		Params: params,
	}

	responseChan, err := ws.CreateResponseChannel(id)
	if err != nil {
		return err
	}
	defer ws.RemoveResponseChannel(id)

	if err := ws.write(request); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case res, open := <-responseChan:
		if !open {
			return errors.New("response channel closed")
		}

		// In case the caller designated to throw away the result by specifying `nil` as `dest`,
		// OR the response Result says its nowherey by being nil,
		// we cannot proceed with unmarshaling the Result field,
		// because it would always fail.
		// The only thing we can do is to return the error if any.
		if nilOrTypedNil(dest) || res.Result == nil || res.Error != nil {
			return eliminateTypedNilError(res.Error)
		}

		if err := ws.unmarshalRes(res, dest); err != nil {
			return fmt.Errorf("error unmarshaling response: %w", err)
		}

		return eliminateTypedNilError(res.Error)
	}
}

func (ws *WebSocketConnection) unmarshalRes(res RPCResponse[cbor.RawMessage], dest interface{}) error {
	return UnmarshalResult(ws.Unmarshaler, res, dest)
}

// UnmarshalResult unmarshals the RPC response result to the destination's Result field.
//
// We try our best to avoid unmarshaling the entire CBOR response twice,
// once in the WebSocketConnection.handleResponse and once here.
//
// With the approach implemented in this function,
// we only unmarshal the ID and the Error fields of the RPCResponse once in handleResponse,
// and then we only unmarshal the Result field here.
//
// Assuming `dest` points to RPCResponse[SomeTypeParam],
// we need to set the ID, Error and Result fields of the `dest` struct,
// so that we can make this function generic enough to work with any RPCResponse[T] type.
func UnmarshalResult(unmarshaler codec.Unmarshaler, responseRaw RPCResponse[cbor.RawMessage], responseDest interface{}) error {
	// Although this looks marshaling unnmarshaled data again, it is not.
	// The `res.Result` is of type `cbor.RawMessage`, which is
	// a type that implements `cbor.Unmarshaller` that returns the raw CBOR bytes
	// contained in the `cbor.RawMessage` itself, instead of actually marshaling anything,
	// so it is low-cost.
	rawCBORBytes, err := responseRaw.Result.MarshalCBOR()
	if err != nil {
		return fmt.Errorf("Send: error marshaling result: %w", err)
	}

	kind := reflect.TypeOf(responseDest).Kind()
	if kind != reflect.Ptr {
		return fmt.Errorf("Send: dest must be a pointer, got %T", responseDest)
	}

	const (
		FieldID     = "ID"
		FieldResult = "Result"
	)

	// Depending on how you called it,
	// dest could be either of the following:
	// 1. *connection.RPCResponse[T]
	// 2. *interface {}(*connection.RPCResponse[T])
	//
	// For the first case, we need to do reflect.Value.Elem() once to
	// get the underlying struct type.
	//
	// For second case, we need to do it thrice to get the underlying struct type.
	//
	// The first case is the most common one, which is when you used Send indirectly from
	// one of the methods like `Select`, `Create`, `Update`, etc.
	//
	// The second case is when you used Send directly, or via a custom method that calls Send.
	// See https://github.com/surrealdb/surrealdb.go/issues/246 for more context.
	var destStruct reflect.Value
	switch structOrIfacePtrStruct := reflect.ValueOf(responseDest).Elem(); structOrIfacePtrStruct.Kind() {
	case reflect.Interface:
		// If dest was a pointer to an interface,
		// we need to get the underlying pointer that is wrapped in the interface.
		ptrStruct := structOrIfacePtrStruct.Elem()

		if ptrStruct.Kind() == reflect.Ptr {
			// If dest is an interface that points to a pointer, we need to get the underlying struct type.
			destStruct = ptrStruct.Elem()
		} else {
			return fmt.Errorf("Send: dest must be a pointer to a struct, got %T", responseDest)
		}
	case reflect.Struct:
		// If dest was a pointer to a struct,
		// destStructOrIface is the struct we want to use.
		destStruct = structOrIfacePtrStruct
	default:
		return fmt.Errorf("Send: dest must be a pointer to a struct or an interface, got %T", responseDest)
	}

	// At this point, we assume `destStruct` points to a struct with ID and Result fields.
	// If it does not, we will panic like:
	//   panic: reflect: call of reflect.Value.FieldByName on interface Value

	// HTTP-only:
	//
	// This nil check prevents the following panic when this function is unmarshaling the RPC response
	// over HTTP, where the ID field is not set in the response:
	//
	//   panic: reflect: call of reflect.Value.Set on zero Value
	if responseRaw.ID != nil {
		destStruct.FieldByName(FieldID).Set(reflect.ValueOf(responseRaw.ID))
	}
	// `destStructDotResult` is basically `dest.Result` in case `dest` was of type `*RPCResponse[T]`.
	destStructDotResult := destStruct.FieldByName(FieldResult).Interface()

	// destValue could be (*T)nil, like (*string)nil, where (*string)nil != nil!
	// That's why we nil-check using nilOrTypedNil, rather than just `if destResult == nil`.
	if nilOrTypedNil(destStructDotResult) {
		destStructDotResult = reflect.New(destStruct.FieldByName(FieldResult).Type().Elem()).Interface()
		destStruct.FieldByName(FieldResult).Set(reflect.ValueOf(destStructDotResult))
	}

	// We unmarshal only the `Result` portion of the response into the `destStructDotResult`.
	// The unmarshaling of ID and Result happened in handleResponse,
	// and the unmarshaling of Result happened here.
	// Finally, we avoided unmarshaling the entire response twice, once in handleResponse and once here.
	if err := unmarshaler.Unmarshal(rawCBORBytes, destStructDotResult); err != nil {
		return fmt.Errorf("Send: error unmarshaling result: %w", err)
	}

	return nil
}

// eliminatedTypedNilError is required because otherwise the caller cannot just use `if err != nil { ... }`
// to check for errors, because it would return true for typed nils like (*SomeErrorType)(nil).
func eliminateTypedNilError(err error) error {
	if nilOrTypedNil(err) {
		return nil
	}

	return err
}

func nilOrTypedNil(val any) bool {
	if val == nil {
		return true
	}

	return reflectiveNilOrTypedNil(reflect.ValueOf(val))
}

func reflectiveNilOrTypedNil(v reflect.Value) bool {
	k := v.Kind()
	switch k {
	case reflect.Chan, reflect.Func, reflect.Map,
		reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return v.IsNil()
	case reflect.Pointer:
		// This is for the case like val is `interface{}(*sometype) nil`
		if v.IsNil() {
			return true
		}

		// This is for the case val is `interface{}(*interface {})*nil`
		elm := v.Elem()
		return reflectiveNilOrTypedNil(elm)
	}

	return false
}

func (ws *WebSocketConnection) write(v interface{}) error {
	data, err := ws.Marshaler.Marshal(v)
	if err != nil {
		return err
	}

	ws.connLock.Lock()
	defer ws.connLock.Unlock()
	return ws.Conn.WriteMessage(gorilla.BinaryMessage, data)
}

func (ws *WebSocketConnection) readLoop() {
	for {
		select {
		case <-ws.connCloseCh:
			return
		default:
			_, data, err := ws.Conn.ReadMessage()
			if err != nil {
				shouldExit := ws.handleError(err)
				if shouldExit {
					ws.state = WebSocketStateDisconnected
					ws.logger.Error("WebSocketConnection readLoop: connection closed", "error", err)
					return
				}
				continue
			}
			go ws.handleResponse(data)
		}
	}
}

// handleError returns true if the error indicates that the connection is closed
// and the readLoop should exit, false otherwise.
func (ws *WebSocketConnection) handleError(err error) bool {
	if errors.Is(err, net.ErrClosed) {
		ws.connCloseError = net.ErrClosed
		return true
	}
	if gorilla.IsUnexpectedCloseError(err) {
		ws.connCloseError = io.ErrClosedPipe
		<-ws.connCloseCh
		return true
	}

	ws.logger.Error(err.Error())
	return false
}

func (ws *WebSocketConnection) handleResponse(res []byte) {
	var rpcRes RPCResponse[cbor.RawMessage]
	if err := ws.Unmarshaler.Unmarshal(res, &rpcRes); err != nil {
		panic(err)
	}

	if rpcRes.ID != nil && rpcRes.ID != "" {
		// Try to resolve message as response to query
		responseChan, ok := ws.GetResponseChannel(fmt.Sprintf("%v", rpcRes.ID))
		if !ok {
			err := fmt.Errorf("unavailable ResponseChannel %+v", rpcRes.ID)
			ws.logger.Error(err.Error())
			return
		}
		defer close(responseChan)
		responseChan <- rpcRes
	} else {
		// todo: find a surefire way to confirm a notification

		notificationRes, err := rpcRes.Result.MarshalCBOR()
		if err != nil {
			ws.logger.Error(
				fmt.Sprintf("error marshaling notification result: %v", err),
				"result", fmt.Sprint(rpcRes.Result),
			)
			return
		}

		var notification Notification
		if err := ws.Unmarshaler.Unmarshal(notificationRes, &notification); err != nil {
			ws.logger.Error(
				fmt.Sprintf("error unmarshaling as notification: %v", err),
				"result", fmt.Sprint(rpcRes.Result),
			)
			return
		}

		channelID := notification.ID

		if channelID == nil {
			ws.logger.Error(
				"response did not contain an 'id' field",
				"result", fmt.Sprint(rpcRes.Result),
			)
			return
		}

		LiveNotificationChan, ok := ws.GetNotificationChannel(channelID.String())
		if !ok {
			ws.logger.Error(
				fmt.Sprintf("unavailable ResponseChannel %+v", channelID.String()),
				"result", fmt.Sprint(rpcRes.Result),
			)
			return
		}

		LiveNotificationChan <- notification
	}
}
