package connection

import (
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
	Timeout  time.Duration
	Option   []Option
	logger   logger.Logger

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

func (ws *WebSocketConnection) Connect() error {
	if err := ws.preConnectionChecks(); err != nil {
		return err
	}

	connection, res, err := DefaultDialer.Dial(fmt.Sprintf("%s/rpc", ws.baseURL), nil)
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

func (ws *WebSocketConnection) Close() error {
	ws.connLock.Lock()
	defer ws.connLock.Unlock()
	close(ws.closeChan)
	err := ws.Conn.WriteMessage(gorilla.CloseMessage, gorilla.FormatCloseMessage(constants.CloseMessageCode, ""))
	if err != nil {
		return err
	}

	return ws.Conn.Close()
}

func (ws *WebSocketConnection) Use(namespace, database string) error {
	return ws.Send(nil, "use", namespace, database)
}

func (ws *WebSocketConnection) Let(key string, value interface{}) error {
	return ws.Send(nil, "let", key, value)
}

func (ws *WebSocketConnection) Unset(key string) error {
	return ws.Send(nil, "unset", key)
}

func (ws *WebSocketConnection) GetUnmarshaler() codec.Unmarshaler {
	return ws.unmarshaler
}

// Send requires `res` to be of type `*RPCResponse[T]` where T is a type that implements `cbor.Unmarshaller`.
// It could be more obvious if Go allowed us to write it like:
//
//	Send[T cbor.Unmarshaller](res *RPCResponse[T], method string, params ...interface{}) error
//
// But it doesn't, so we have to use `interface{}`.
// The caller is responsible for ensuring that `res` is of the correct type.
func (ws *WebSocketConnection) Send(dest interface{}, method string, params ...interface{}) error {
	select {
	case <-ws.closeChan:
		return ws.closeError
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
	timeout := time.After(ws.Timeout)

	select {
	case <-timeout:
		return constants.ErrTimeout
	case res, open := <-responseChan:
		if !open {
			return errors.New("response channel closed")
		}

		// In case the caller designated to throw away the result by specifying `nil` as `dest`,
		// OR the response Result says its nowherey by being nil,
		// we cannot proceed with unmarshaling the Result field,
		// because it would always fail.
		// The only thing we can do is to return the error if any.
		if nilOrTypedNil(dest) || res.Result == nil {
			return eliminateTypedNilError(res.Error)
		}

		if err := ws.unmarshalRes(res, dest); err != nil {
			return fmt.Errorf("error unmarshalling response: %w", err)
		}

		return eliminateTypedNilError(res.Error)
	}
}

func (ws *WebSocketConnection) unmarshalRes(res RPCResponse[cbor.RawMessage], dest interface{}) error {
	// Although this looks marshaling unnmarshaled data again, it is not.
	// The `res.Result` is of type `cbor.RawMessage`, which is
	// a type that implements `cbor.Unmarshaller` that returns the raw CBOR bytes
	// contained in the `cbor.RawMessage` itself, instead of actually marshaling anything,
	// so it is low-cost.
	rawCBORBytes, err := res.Result.MarshalCBOR()
	if err != nil {
		return fmt.Errorf("Send: error marshalling result: %w", err)
	}

	// In the below, we try our best to avoid unmarshaling the entire CBOR response twice,
	// once in the WebSocketConnection.handleResponse and once here.
	//
	// With the approach below, we only unmarshal the ID and the Error fields of the RPCResponse once in handleResponse,
	// and then we only unmarshal the Result field here.

	// In case `dest` is RPCResponse[SomeTypeParam] we need to set the ID and Result fields.
	// Note that we cannot treat it like RPCResponse[any] or RPCResponse[cbor.RawMessage] as well.
	// The alternative is to use reflection like this.
	if reflect.TypeOf(dest).Kind() != reflect.Ptr {
		return fmt.Errorf("Send: dest must be a pointer, got %T", dest)
	}

	const (
		FieldID     = "ID"
		FieldResult = "Result"
	)

	destStruct := reflect.ValueOf(dest).Elem()

	// At this point, we assume `destStruct` points to a struct with ID and Result fields.
	// If it does not, we will panic like:
	//   panic: reflect: call of reflect.Value.FieldByName on interface Value

	destStruct.FieldByName(FieldID).Set(reflect.ValueOf(res.ID))
	// `destStructDotResult` is basically `dest.Result` if `dest` was of type `*RPCResponse[T]`.
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
		return fmt.Errorf("Send: error unmarshalling result: %w", err)
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
			err := fmt.Errorf("error marshalling notification result: %w", err)
			ws.logger.Error(err.Error(), "result", fmt.Sprint(rpcRes.Result))
			return
		}

		var notification Notification
		if err := ws.unmarshaler.Unmarshal(notificationRes, &notification); err != nil {
			err := fmt.Errorf("error unmarshalling as notification: %w", err)
			ws.logger.Error(err.Error(), "result", fmt.Sprint(rpcRes.Result))
			return
		}

		channelID := notification.ID

		if channelID == nil {
			err := fmt.Errorf("response did not contain an 'id' field")
			ws.logger.Error(err.Error(), "result", fmt.Sprint(rpcRes.Result))
			return
		}

		LiveNotificationChan, ok := ws.getNotificationChannel(channelID.String())
		if !ok {
			err := fmt.Errorf("unavailable ResponseChannel %+v", channelID.String())
			ws.logger.Error(err.Error(), "result", fmt.Sprint(rpcRes.Result))
			return
		}

		LiveNotificationChan <- notification
	}
}
