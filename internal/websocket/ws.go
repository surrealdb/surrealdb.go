package websocket

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/surrealdb/surrealdb.go/internal/rand"
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
	Conn     *websocket.Conn
	connLock sync.Mutex
	Timeout  time.Duration

	responseChannels     map[string]chan RPCResponse
	responseChannelsLock sync.RWMutex

	close chan int
}

func NewWebsocketWithOptions(url string, options ...Option) (*WebSocket, error) {
	dialer := websocket.DefaultDialer
	dialer.EnableCompression = true

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	ws := &WebSocket{
		Conn:             conn,
		close:            make(chan int),
		responseChannels: make(map[string]chan RPCResponse),
		Timeout:          DefaultTimeout * time.Second,
	}

	for _, option := range options {
		if err := option(ws); err != nil {
			return nil, err
		}
	}

	ws.initialize()
	return ws, nil
}

func (ws *WebSocket) Close() error {
	defer func() {
		close(ws.close)
	}()

	return ws.Conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(CloseMessageCode, ""))
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

func (ws *WebSocket) Send(method string, params []interface{}) (interface{}, error) {
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
	return ws.Conn.WriteMessage(websocket.TextMessage, data)
}

func (ws *WebSocket) initialize() {
	go func() {
		for {
			select {
			case <-ws.close:
				return
			default:
				var res RPCResponse
				err := ws.read(&res)
				if err != nil {
					// TODO need to find a proper way to log this error
					continue
				}
				responseChan, ok := ws.getResponseChannel(fmt.Sprintf("%v", res.ID))
				if !ok {
					// TODO need to find a proper way to log this
					continue
				}
				responseChan <- res
				close(responseChan)
			}
		}
	}()
}
