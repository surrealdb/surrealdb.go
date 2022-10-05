package websocket

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// CloseMessageCode identifier the message id for a close request
	CloseMessageCode = 1000
	// DefaultTimeout timeout in seconds
	DefaultTimeout = 30
)

type Option func(ws *WebSocket) error

type WebSocket struct {
	conn    *websocket.Conn
	timeout time.Duration

	respChan chan RPCResponse
	close    chan int
}

func NewWebsocket(url string) (*WebSocket, error) {
	return NewWebsocketWithOptions(url, Timeout(DefaultTimeout))
}

func NewWebsocketWithOptions(url string, options ...Option) (*WebSocket, error) {
	dialer := websocket.DefaultDialer
	dialer.EnableCompression = true

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	ws := &WebSocket{
		conn:     conn,
		respChan: make(chan RPCResponse),
		close:    make(chan int),
	}

	for _, option := range options {
		if err := option(ws); err != nil {
			return nil, err
		}
	}

	ws.initialize()
	return ws, nil
}

func Timeout(timeout float64) Option {
	return func(ws *WebSocket) error {
		ws.timeout = time.Duration(timeout) * time.Second
		return nil
	}
}

func (ws *WebSocket) Close() error {
	defer func() {
		close(ws.close)
	}()

	return ws.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(CloseMessageCode, ""))
}

func (ws *WebSocket) Send(id, method string, params []interface{}) (interface{}, error) {
	request := &RPCRequest{
		ID:     id,
		Method: method,
		Params: params,
	}

	if err := ws.write(request); err != nil {
		return nil, err
	}

	tick := time.NewTicker(ws.timeout)
	for {
		select {
		case <-tick.C:
			return nil, errors.New("timeout")
		case res := <-ws.respChan:
			if res.ID != id {
				continue
			}

			if res.Error != nil {
				return nil, res.Error
			}

			return res.Result, nil
		}
	}
}

func (ws *WebSocket) read(v interface{}) error {
	_, data, err := ws.conn.ReadMessage()
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

	return ws.conn.WriteMessage(websocket.TextMessage, data)
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

				ws.respChan <- res
			}
		}
	}()
}
