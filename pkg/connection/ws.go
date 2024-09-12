package connection

import (
	"errors"
	"fmt"
	"github.com/surrealdb/surrealdb.go/internal/rand"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/model"
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

type Option func(ws *WebSocket) error

type WebSocket struct {
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

func NewWebSocket(p NewConnectionParams) *WebSocket {
	return &WebSocket{
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

func (ws *WebSocket) Connect() error {
	if ws.baseURL == "" {
		return fmt.Errorf("base url not set")
	}

	dialer := gorilla.DefaultDialer
	dialer.EnableCompression = true

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

func (ws *WebSocket) SetTimeOut(timeout time.Duration) *WebSocket {
	ws.Option = append(ws.Option, func(ws *WebSocket) error {
		ws.Timeout = timeout
		return nil
	})
	return ws
}

// If path is empty it will use os.stdout/os.stderr
func (ws *WebSocket) Logger(logData logger.Logger) *WebSocket {
	ws.logger = logData
	return ws
}

func (ws *WebSocket) RawLogger(logData logger.Logger) *WebSocket {
	ws.logger = logData
	return ws
}

func (ws *WebSocket) SetCompression(compress bool) *WebSocket {
	ws.Option = append(ws.Option, func(ws *WebSocket) error {
		ws.Conn.EnableWriteCompression(compress)
		return nil
	})
	return ws
}

func (ws *WebSocket) Close() error {
	ws.connLock.Lock()
	defer ws.connLock.Unlock()
	close(ws.closeChan)
	err := ws.Conn.WriteMessage(gorilla.CloseMessage, gorilla.FormatCloseMessage(CloseMessageCode, ""))
	if err != nil {
		return err
	}

	return ws.Conn.Close()
}

func (ws *WebSocket) LiveNotifications(liveQueryID string) (chan Notification, error) {
	c, err := ws.createNotificationChannel(liveQueryID)
	if err != nil {
		ws.logger.Error(err.Error())
	}
	return c, err
}

var (
	ErrIDInUse           = errors.New("id already in use")
	ErrTimeout           = errors.New("timeout")
	ErrInvalidResponseID = errors.New("invalid response id")
)

func (ws *WebSocket) createResponseChannel(id string) (chan RPCResponse, error) {
	ws.responseChannelsLock.Lock()
	defer ws.responseChannelsLock.Unlock()

	if _, ok := ws.responseChannels[id]; ok {
		return nil, fmt.Errorf("%w: %v", ErrIDInUse, id)
	}

	ch := make(chan RPCResponse)
	ws.responseChannels[id] = ch

	return ch, nil
}

func (ws *WebSocket) createNotificationChannel(liveQueryID string) (chan Notification, error) {
	ws.notificationChannelsLock.Lock()
	defer ws.notificationChannelsLock.Unlock()

	if _, ok := ws.notificationChannels[liveQueryID]; ok {
		return nil, fmt.Errorf("%w: %v", ErrIDInUse, liveQueryID)
	}

	ch := make(chan Notification)
	ws.notificationChannels[liveQueryID] = ch

	return ch, nil
}

func (ws *WebSocket) removeResponseChannel(id string) {
	ws.responseChannelsLock.Lock()
	defer ws.responseChannelsLock.Unlock()
	delete(ws.responseChannels, id)
}

func (ws *WebSocket) getResponseChannel(id string) (chan RPCResponse, bool) {
	ws.responseChannelsLock.RLock()
	defer ws.responseChannelsLock.RUnlock()
	ch, ok := ws.responseChannels[id]
	return ch, ok
}

func (ws *WebSocket) getLiveChannel(id string) (chan Notification, bool) {
	ws.notificationChannelsLock.RLock()
	defer ws.notificationChannelsLock.RUnlock()
	ch, ok := ws.notificationChannels[id]
	return ch, ok
}

func (ws *WebSocket) Use(namespace string, database string) error {
	params := []interface{}{namespace, database}
	_, err := ws.Send("use", params)
	if err != nil {
		return err
	}

	return nil
}

func (ws *WebSocket) SignIn(auth model.Auth) (string, error) {
	resp, err := ws.Send("signin", []interface{}{auth})
	if err != nil {
		return "", err
	}

	return resp.(string), nil
}

func (ws *WebSocket) Send(method string, params []interface{}) (interface{}, error) {
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

func (ws *WebSocket) read(v interface{}) error {
	_, data, err := ws.Conn.ReadMessage()
	if err != nil {
		return err
	}
	//return json.Unmarshal(data, v)
	return ws.unmarshaler.Unmarshal(data, v)
}

func (ws *WebSocket) write(v interface{}) error {
	//fmt.Printf("%+v\n", v)
	//data, err := json.Marshal(v)
	data, err := ws.marshaler.Marshal(v)
	if err != nil {
		return err
	}

	ws.connLock.Lock()
	defer ws.connLock.Unlock()
	return ws.Conn.WriteMessage(gorilla.TextMessage, data)
}

func (ws *WebSocket) initialize() {
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

func (ws *WebSocket) handleError(err error) bool {
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

func (ws *WebSocket) handleResponse(res RPCResponse) {
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
