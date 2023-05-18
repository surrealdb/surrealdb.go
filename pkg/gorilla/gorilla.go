package gorilla

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

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

	close chan int
}

func Create() *WebSocket {
	return &WebSocket{
		Conn:             nil,
		close:            make(chan int),
		responseChannels: make(map[string]chan rpc.RPCResponse),
		Timeout:          DefaultTimeout * time.Second,
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
	case res := <-responseChan:
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
				responseChan, ok := ws.getResponseChannel(fmt.Sprintf("%v", res.ID))
				if !ok {
					err = errors.New("ResponseChannel is not ok")
					ws.logger.Logger.Err(err)
					ws.logger.LogChannel <- err.Error()
					continue
				}
				responseChan <- res
				close(responseChan)
			}
		}
	}()
}
