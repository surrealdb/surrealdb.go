package mock

import "github.com/surrealdb/surrealdb.go/pkg/iwebsocket"

type websocket struct {
}

func (w *websocket) Connect(url string) (iwebsocket.IWebSocket, error) {
	return w, nil
}

func (w *websocket) Send(method string, params []interface{}) (interface{}, error) {
	return nil, nil
}

func (w *websocket) Close() error {
	return nil
}

func Create() *websocket {
	return &websocket{}
}
