package gorilla

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/model"

	gorilla "github.com/gorilla/websocket"
	"github.com/surrealdb/surrealdb.go/internal/rpc"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/rand"
	"github.com/surrealdb/surrealdb.go/pkg/websocket"
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
	Conn     *gorilla.Conn
	connLock sync.Mutex
	Timeout  time.Duration
	Option   []Option
	logger   *logger.LogData

	responseChannels     map[string]chan rpc.RPCResponse
	responseChannelsLock sync.RWMutex

	notificationChannels     map[string]chan model.Notification
	notificationChannelsLock sync.RWMutex

	close chan int
}

func Create() *WebSocket {
	return &WebSocket{
		Conn:                 nil,
		close:                make(chan int),
		responseChannels:     make(map[string]chan rpc.RPCResponse),
		notificationChannels: make(map[string]chan model.Notification),
		Timeout:              DefaultTimeout * time.Second,
	}
}

func (ws *WebSocket) Connect(url string) (websocket.WebSocket, error) {
	dialer := gorilla.DefaultDialer
	dialer.EnableCompression = true

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	ws.Conn = conn

	for _, option := range ws.Option {
		if err := option(ws); err != nil {
			return ws, err
		}
	}

	ws.initialize()
	return ws, nil
}

func (ws *WebSocket) SetTimeOut(timeout time.Duration) *WebSocket {
	ws.Option = append(ws.Option, func(ws *WebSocket) error {
		ws.Timeout = timeout
		return nil
	})
	return ws
}

// If path is empty it will use os.stdout/os.stderr
func (ws *WebSocket) Logger(logData *logger.LogData) *WebSocket {
	ws.logger = logData
	return ws
}

func (ws *WebSocket) RawLogger(logData *logger.LogData) *WebSocket {
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
	defer func() {
		close(ws.close)
	}()

	return ws.Conn.WriteMessage(gorilla.CloseMessage, gorilla.FormatCloseMessage(CloseMessageCode, ""))
}

func (ws *WebSocket) LiveNotifications(liveQueryID string) (chan model.Notification, error) {
	c, err := ws.createNotificationChannel(liveQueryID)
	if err != nil {
		ws.logger.Logger.Err(err)
		ws.logger.LogChannel <- err.Error()
	}
	return c, err
}

var (
	ErrIDInUse           = errors.New("id already in use")
	ErrTimeout           = errors.New("timeout")
	ErrInvalidResponseID = errors.New("invalid response id")
)

func (ws *WebSocket) createResponseChannel(id string) (chan rpc.RPCResponse, error) {
	ws.responseChannelsLock.Lock()
	defer ws.responseChannelsLock.Unlock()

	if _, ok := ws.responseChannels[id]; ok {
		return nil, fmt.Errorf("%w: %v", ErrIDInUse, id)
	}

	ch := make(chan rpc.RPCResponse)
	ws.responseChannels[id] = ch

	return ch, nil
}

func (ws *WebSocket) createNotificationChannel(liveQueryID string) (chan model.Notification, error) {
	ws.notificationChannelsLock.Lock()
	defer ws.notificationChannelsLock.Unlock()

	if _, ok := ws.notificationChannels[liveQueryID]; ok {
		return nil, fmt.Errorf("%w: %v", ErrIDInUse, liveQueryID)
	}

	ch := make(chan model.Notification)
	ws.notificationChannels[liveQueryID] = ch

	return ch, nil
}

func (ws *WebSocket) removeResponseChannel(id string) {
	ws.responseChannelsLock.Lock()
	defer ws.responseChannelsLock.Unlock()
	delete(ws.responseChannels, id)
}

func (ws *WebSocket) getResponseChannel(id string) (chan rpc.RPCResponse, bool) {
	ws.responseChannelsLock.RLock()
	defer ws.responseChannelsLock.RUnlock()
	ch, ok := ws.responseChannels[id]
	return ch, ok
}

func (ws *WebSocket) getLiveChannel(id string) (chan model.Notification, bool) {
	ws.notificationChannelsLock.RLock()
	defer ws.notificationChannelsLock.RUnlock()
	ch, ok := ws.notificationChannels[id]
	return ch, ok
}

func (ws *WebSocket) Send(method string, params []interface{}) (interface{}, error) {
	id := rand.String(RequestIDLength)
	request := &rpc.RPCRequest{
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
	return json.Unmarshal(data, v)
}

func (ws *WebSocket) write(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	ws.connLock.Lock()
	defer ws.connLock.Unlock()
	return ws.Conn.WriteMessage(gorilla.TextMessage, data)
}

func (ws *WebSocket) initialize() {
	go func() {
		for {
			select {
			case <-ws.close:
				return
			default:
				var res rpc.RPCResponse
				err := ws.read(&res)
				if err != nil {
					ws.logger.Logger.Err(err)
					ws.logger.LogChannel <- err.Error()
					continue
				}
				go ws.handleResponse(res)
			}
		}
	}()
}

func (ws *WebSocket) handleResponse(res rpc.RPCResponse) {
	if res.ID != nil && res.ID != "" {
		// Try to resolve message as response to query
		responseChan, ok := ws.getResponseChannel(fmt.Sprintf("%v", res.ID))
		if !ok {
			err := fmt.Errorf("unavailable ResponseChannel %+v", res.ID)
			ws.logger.Logger.Err(err)
			ws.logger.LogChannel <- err.Error()
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
			ws.logger.Logger.With().Str("result", fmt.Sprintf("%s", res.Result)).Err(err)
			ws.logger.LogChannel <- err.Error()
			return
		}
		var notification model.Notification
		err := unmarshalMapToStruct(mappedRes, &notification)
		if err != nil {
			ws.logger.Logger.With().Str("result", fmt.Sprintf("%s", res.Result)).Err(err)
			ws.logger.LogChannel <- err.Error()
			return
		}
		LiveNotificationChan, ok := ws.getLiveChannel(notification.ID)
		if !ok {
			err := fmt.Errorf("unavailable ResponseChannel %+v", resolvedID)
			ws.logger.Logger.Err(err)
			ws.logger.LogChannel <- err.Error()
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
