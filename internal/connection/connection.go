package connection

type Connection interface {
	Connect(url string) (Connection, error)
	Close() error
	Send(method string, params []interface{}) (interface{}, error)
}

type LiveHandler interface {
	LiveNotifications(id string) (chan Notification, error)
}

type Encoder func(value interface{}) ([]byte, error)

type Decoder func(encoded []byte, value interface{}) error

type BaseConnection struct {
	encode  Encoder
	decode  Decoder
	baseURL string
}

type NewConnectionParams struct {
	Encoder Encoder
	Decoder Decoder
	BaseURL string
}
