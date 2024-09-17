package connection

import (
	"errors"
	"fmt"
	"github.com/surrealdb/surrealdb.go/internal/rand"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"io"
	"net"
	"reflect"
	"strconv"
	"sync"
	"time"

	gorilla "github.com/gorilla/websocket"
)

const (
	// RequestIDLength size of id sent on WS request
	RequestIDLength = 16
	// CloseMessageCode identifier the message id for a close request
	CloseMessageCode = 1000
	// DefaultTimeout timeout in seconds
	DefaultTimeout = 30
)

type Option func(ws *WebSocketConnection) error

type WebSocketConnection struct {
	BaseConnection

	Conn     *gorilla.Conn
	connLock sync.Mutex
	Timeout  time.Duration
	Option   []Option
	logger   logger.Logger

	responseChannels     map[string]chan RPCResponse
	responseChannelsLock sync.RWMutex

	notificationChannels     map[string]chan Notification
	notificationChannelsLock sync.RWMutex

	closeChan  chan int
	closeError error
}

func NewWebSocketConnection(p NewConnectionParams) *WebSocketConnection {
	return &WebSocketConnection{
		BaseConnection: BaseConnection{
			marshaler:   p.Marshaler,
			unmarshaler: p.Unmarshaler,
			baseURL:     p.BaseURL,
		},

		Conn:                 nil,
		closeChan:            make(chan int),
		responseChannels:     make(map[string]chan RPCResponse),
		notificationChannels: make(map[string]chan Notification),
		Timeout:              DefaultTimeout * time.Second,
	}
}

func (ws *WebSocketConnection) Connect() error {
	if ws.baseURL == "" {
		return fmt.Errorf("base url not set")
	}

	dialer := gorilla.DefaultDialer
	dialer.EnableCompression = true
	dialer.Subprotocols = append(dialer.Subprotocols, "cbor")

	connection, _, err := dialer.Dial(fmt.Sprintf("%s/rpc", ws.baseURL), nil)
	if err != nil {
		return err
	}

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
	err := ws.Conn.WriteMessage(gorilla.CloseMessage, gorilla.FormatCloseMessage(CloseMessageCode, ""))
	if err != nil {
		return err
	}

	return ws.Conn.Close()
}

func (ws *WebSocketConnection) LiveNotifications(liveQueryID string) (chan Notification, error) {
	c, err := ws.createNotificationChannel(liveQueryID)
	if err != nil {
		ws.logger.Error(err.Error())
	}
	return c, err
}

func (ws *WebSocketConnection) Kill(id string) (interface{}, error) {
	return ws.Send("kill", []interface{}{id})
}

var (
	ErrIDInUse           = errors.New("id already in use")
	ErrTimeout           = errors.New("timeout")
	ErrInvalidResponseID = errors.New("invalid response id")
)

func (ws *WebSocketConnection) createResponseChannel(id string) (chan RPCResponse, error) {
	ws.responseChannelsLock.Lock()
	defer ws.responseChannelsLock.Unlock()

	if _, ok := ws.responseChannels[id]; ok {
		return nil, fmt.Errorf("%w: %v", ErrIDInUse, id)
	}

	ch := make(chan RPCResponse)
	ws.responseChannels[id] = ch

	return ch, nil
}

func (ws *WebSocketConnection) createNotificationChannel(liveQueryID string) (chan Notification, error) {
	ws.notificationChannelsLock.Lock()
	defer ws.notificationChannelsLock.Unlock()

	if _, ok := ws.notificationChannels[liveQueryID]; ok {
		return nil, fmt.Errorf("%w: %v", ErrIDInUse, liveQueryID)
	}

	ch := make(chan Notification)
	ws.notificationChannels[liveQueryID] = ch

	return ch, nil
}

func (ws *WebSocketConnection) removeResponseChannel(id string) {
	ws.responseChannelsLock.Lock()
	defer ws.responseChannelsLock.Unlock()
	delete(ws.responseChannels, id)
}

func (ws *WebSocketConnection) getResponseChannel(id string) (chan RPCResponse, bool) {
	ws.responseChannelsLock.RLock()
	defer ws.responseChannelsLock.RUnlock()
	ch, ok := ws.responseChannels[id]
	return ch, ok
}

func (ws *WebSocketConnection) getLiveChannel(id string) (chan Notification, bool) {
	ws.notificationChannelsLock.RLock()
	defer ws.notificationChannelsLock.RUnlock()
	ch, ok := ws.notificationChannels[id]
	return ch, ok
}

func (ws *WebSocketConnection) Use(namespace, database string) error {
	_, err := ws.Send("use", []interface{}{namespace, database})
	if err != nil {
		return err
	}

	return nil
}

func (ws *WebSocketConnection) Let(key string, value interface{}) error {
	_, err := ws.Send("let", []interface{}{key, value})
	return err
}

func (ws *WebSocketConnection) Unset(key string) error {
	_, err := ws.Send("unset", []interface{}{key})
	return err
}

func (ws *WebSocketConnection) Send(method string, params []interface{}) (interface{}, error) {
	select {
	case <-ws.closeChan:
		return nil, ws.closeError
	default:
	}

	id := rand.String(RequestIDLength)
	request := &RPCRequest{
		ID:     id,
		Method: method,
		Params: params,
	}

	responseChan, err := ws.createResponseChannel(id)
	if err != nil {
		return nil, err
	}
	defer ws.removeResponseChannel(id)

	if err := ws.write(request); err != nil {
		return nil, err
	}
	timeout := time.After(ws.Timeout)

	select {
	case <-timeout:
		return nil, ErrTimeout
	case res, open := <-responseChan:
		if !open {
			return nil, errors.New("channel closed")
		}
		if res.ID != id {
			return nil, ErrInvalidResponseID
		}
		if res.Error != nil {
			return nil, res.Error
		}
		return res.Result, nil
	}
}

func (ws *WebSocketConnection) read(v interface{}) error {
	_, data, err := ws.Conn.ReadMessage()
	if err != nil {
		return err
	}
	return ws.unmarshaler.Unmarshal(data, v)
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
			var res RPCResponse
			err := ws.read(&res)
			if err != nil {
				shouldExit := ws.handleError(err)
				if shouldExit {
					return
				}
				continue
			}
			go ws.handleResponse(res)
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

func (ws *WebSocketConnection) handleResponse(res RPCResponse) {
	if res.ID != nil && res.ID != "" {
		// Try to resolve message as response to query
		responseChan, ok := ws.getResponseChannel(fmt.Sprintf("%v", res.ID))
		if !ok {
			err := fmt.Errorf("unavailable ResponseChannel %+v", res.ID)
			ws.logger.Error(err.Error())
			return
		}
		defer close(responseChan)
		responseChan <- res
	} else {
		// Try to resolve response as live query notification
		mappedRes, _ := res.Result.(map[string]interface{})
		resolvedID, ok := mappedRes["id"]
		if !ok {
			err := fmt.Errorf("response did not contain an 'id' field")

			ws.logger.Error(err.Error(), "result", fmt.Sprint(res.Result))
			return
		}
		var notification Notification
		err := unmarshalMapToStruct(mappedRes, &notification)
		if err != nil {
			ws.logger.Error(err.Error(), "result", fmt.Sprint(res.Result))
			return
		}
		LiveNotificationChan, ok := ws.getLiveChannel(notification.ID)
		if !ok {
			err := fmt.Errorf("unavailable ResponseChannel %+v", resolvedID)
			ws.logger.Error(err.Error(), "result", fmt.Sprint(res.Result))
			return
		}
		LiveNotificationChan <- notification
	}
}

func unmarshalMapToStruct(data map[string]interface{}, outStruct interface{}) error {
	outValue := reflect.ValueOf(outStruct)
	if outValue.Kind() != reflect.Ptr || outValue.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("outStruct must be a pointer to a struct")
	}

	structValue := outValue.Elem()
	structType := structValue.Type()

	for i := 0; i < structValue.NumField(); i++ {
		field := structType.Field(i)
		fieldName := field.Name
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			fieldName = jsonTag
		}
		mapValue, ok := data[fieldName]
		if !ok {
			return fmt.Errorf("missing field in map: %s", fieldName)
		}

		fieldValue := structValue.Field(i)
		if !fieldValue.CanSet() {
			return fmt.Errorf("cannot set field: %s", fieldName)
		}

		if mapValue == nil {
			// Handle nil values appropriately for your struct fields
			// For simplicity, we skip nil values in this example
			continue
		}

		// Type conversion based on the field type
		switch fieldValue.Kind() {
		case reflect.String:
			fieldValue.SetString(fmt.Sprint(mapValue))
		case reflect.Int:
			intVal, err := strconv.Atoi(fmt.Sprint(mapValue))
			if err != nil {
				return err
			}
			fieldValue.SetInt(int64(intVal))
		case reflect.Bool:
			boolVal, err := strconv.ParseBool(fmt.Sprint(mapValue))
			if err != nil {
				return err
			}
			fieldValue.SetBool(boolVal)
		case reflect.Interface:
			fieldValue.Set(reflect.ValueOf(mapValue))
		// Add cases for other types as needed
		default:
			return fmt.Errorf("unsupported field type: %s", fieldName)
		}
	}

	return nil
}
