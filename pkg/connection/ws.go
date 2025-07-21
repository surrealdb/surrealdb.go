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

type WebSocketConnection struct {
	BaseConnection

	Conn     *gorilla.Conn
	connLock sync.Mutex
	// Timeout is the timeout for receiveing the RPC response after
	// you've successfully sent the request.
	//
	// If the timeout is reached, the Send method will return ErrTimeout.
	// You can set it to 0 to disable the timeout, and instead use context.Context and context.WithTimeout
	// to control the timeout. It will be useful if you want to avoid the overhead of wrapping the context
	// with a timeout.
	Timeout time.Duration
	Option  []Option
	logger  logger.Logger

	closeChan  chan int
	closeError error
}

func NewWebSocketConnection(p NewConnectionParams) *WebSocketConnection {
	return &WebSocketConnection{
		BaseConnection: BaseConnection{
			baseURL: p.BaseURL,

			marshaler:   p.Marshaler,
			unmarshaler: p.Unmarshaler,

			responseChannels:     make(map[string]chan RPCResponse[cbor.RawMessage]),
			notificationChannels: make(map[string]chan Notification),
		},

		Conn:      nil,
		closeChan: make(chan int),
		Timeout:   constants.DefaultWSTimeout,
		logger:    logger.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

func (ws *WebSocketConnection) Connect(ctx context.Context) error {
	if err := ws.preConnectionChecks(); err != nil {
		return err
	}

	connection, res, err := DefaultDialer.DialContext(ctx, fmt.Sprintf("%s/rpc", ws.baseURL), nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	ws.Conn = connection

	for _, option := range ws.Option {
		if err := option(ws); err != nil {
			return err
		}
	}

	go ws.initialize()
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
func (ws *WebSocketConnection) Close(ctx context.Context) error {
	ws.connLock.Lock()
	defer ws.connLock.Unlock()

	// Signal that we're closing so that the goroutine reading from the connection
	// can stop reading messages and exit.
	//
	// TODO: This might not be necessary, because the gorilla.Conn.Close() method
	// will close the connection and that would result in the ReadMessage call in
	// ws.initialize() goroutine to return an error, which will stop the goroutine.
	close(ws.closeChan)

	// Phase 1: Try to send the close message
	//
	// We assume this is important to let the server know that we're closing the connection.
	// If the write fails, we still try to close the connection locally,
	// so that we don't leak resources locally.

	writeErr := make(chan error, 1)
	go func() {
		writeErr <- ws.Conn.WriteMessage(gorilla.CloseMessage, gorilla.FormatCloseMessage(constants.CloseMessageCode, ""))
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
		// Again, we don't return here, because we try out best to Close the connection anyway,
		// although it might not be a clean close from the server's perspective.
	}

	// Phase 2: Close the underlying connection.
	//
	// We assume the Close method of the gorilla.Conn is an instantaneous operation,
	// so we don't need to consider the context here, even
	// in case the context is already canceled.
	//
	// We do this regardless of whether the write of the close message succeeded or not,
	// because we want to ensure the local resources are cleaned up anyway,
	// although the lack of a close message write might result in the server not knowing
	// that the client is closing the connection in a timely manner.

	return ws.Conn.Close()
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
	return ws.unmarshaler
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
	case <-ws.closeChan:
		return ws.closeError
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

	responseChan, err := ws.createResponseChannel(id)
	if err != nil {
		return err
	}
	defer ws.removeResponseChannel(id)

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
			return fmt.Errorf("error unmarshaing response: %w", err)
		}

		return eliminateTypedNilError(res.Error)
	}
}

// unmarshalRes try our best to avoid unmarshaling the entire CBOR response twice,
// once in the WebSocketConnection.handleResponse and once here.
//
// With the approach implemented in this function,
// we only unmarshal the ID and the Error fields of the RPCResponse once in handleResponse,
// and then we only unmarshal the Result field here.
//
// Assuming `dest` points to RPCResponse[SomeTypeParam],
// we need to set the ID, Error and Result fields of the `dest` struct,
// so that we can make this function generic enough to work with any RPCResponse[T] type.
func (ws *WebSocketConnection) unmarshalRes(res RPCResponse[cbor.RawMessage], dest interface{}) error {
	// Although this looks marshaling unnmarshaled data again, it is not.
	// The `res.Result` is of type `cbor.RawMessage`, which is
	// a type that implements `cbor.Unmarshaller` that returns the raw CBOR bytes
	// contained in the `cbor.RawMessage` itself, instead of actually marshaling anything,
	// so it is low-cost.
	rawCBORBytes, err := res.Result.MarshalCBOR()
	if err != nil {
		return fmt.Errorf("Send: error marshaling result: %w", err)
	}

	kind := reflect.TypeOf(dest).Kind()
	if kind != reflect.Ptr {
		return fmt.Errorf("Send: dest must be a pointer, got %T", dest)
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
	switch structOrIfacePtrStruct := reflect.ValueOf(dest).Elem(); structOrIfacePtrStruct.Kind() {
	case reflect.Interface:
		// If dest was a pointer to an interface,
		// we need to get the underlying pointer that is wrapped in the interface.
		ptrStruct := structOrIfacePtrStruct.Elem()

		if ptrStruct.Kind() == reflect.Ptr {
			// If dest is an interface that points to a pointer, we need to get the underlying struct type.
			destStruct = ptrStruct.Elem()
		} else {
			return fmt.Errorf("Send: dest must be a pointer to a struct, got %T", dest)
		}
	case reflect.Struct:
		// If dest was a pointer to a struct,
		// destStructOrIface is the struct we want to use.
		destStruct = structOrIfacePtrStruct
	default:
		return fmt.Errorf("Send: dest must be a pointer to a struct or an interface, got %T", dest)
	}

	// At this point, we assume `destStruct` points to a struct with ID and Result fields.
	// If it does not, we will panic like:
	//   panic: reflect: call of reflect.Value.FieldByName on interface Value

	destStruct.FieldByName(FieldID).Set(reflect.ValueOf(res.ID))
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
	if err := ws.unmarshaler.Unmarshal(rawCBORBytes, destStructDotResult); err != nil {
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
	data, err := ws.marshaler.Marshal(v)
	if err != nil {
		return err
	}

	ws.connLock.Lock()
	defer ws.connLock.Unlock()
	return ws.Conn.WriteMessage(gorilla.BinaryMessage, data)
}

func (ws *WebSocketConnection) initialize() {
	for {
		select {
		case <-ws.closeChan:
			return
		default:
			_, data, err := ws.Conn.ReadMessage()
			if err != nil {
				shouldExit := ws.handleError(err)
				if shouldExit {
					return
				}
				continue
			}
			go ws.handleResponse(data)
		}
	}
}

func (ws *WebSocketConnection) handleError(err error) bool {
	if errors.Is(err, net.ErrClosed) {
		ws.closeError = net.ErrClosed
		return true
	}
	if gorilla.IsUnexpectedCloseError(err) {
		ws.closeError = io.ErrClosedPipe
		<-ws.closeChan
		return true
	}

	ws.logger.Error(err.Error())
	return false
}

func (ws *WebSocketConnection) handleResponse(res []byte) {
	var rpcRes RPCResponse[cbor.RawMessage]
	if err := ws.unmarshaler.Unmarshal(res, &rpcRes); err != nil {
		panic(err)
	}

	if rpcRes.ID != nil && rpcRes.ID != "" {
		// Try to resolve message as response to query
		responseChan, ok := ws.getResponseChannel(fmt.Sprintf("%v", rpcRes.ID))
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
		if err := ws.unmarshaler.Unmarshal(notificationRes, &notification); err != nil {
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

		LiveNotificationChan, ok := ws.getNotificationChannel(channelID.String())
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
