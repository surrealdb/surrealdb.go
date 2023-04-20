package iwebsocket

type IWebSocket interface {
	Connect(url string) (IWebSocket, error)
	Send(method string, params []interface{}) (interface{}, error)
	Close() error
}
