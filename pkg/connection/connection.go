package connection

import (
	"github.com/surrealdb/surrealdb.go/internal/codec"
)

type Connection interface {
	Connect() error
	Close() error
	Send(res interface{}, method string, params ...interface{}) error
	Use(namespace string, database string) error
	Let(key string, value interface{}) error
	Unset(key string) error
}

type LiveHandler interface {
	LiveNotifications(id string) (chan Notification, error)
	Kill(id string) error
}

type NewConnectionParams struct {
	Marshaler   codec.Marshaler
	Unmarshaler codec.Unmarshaler
	BaseURL     string
}

type BaseConnection struct {
	marshaler   codec.Marshaler
	unmarshaler codec.Unmarshaler
	baseURL     string
}

func (b *BaseConnection) handleResponse(dest interface{}, respData []byte) error {
	var rpcResponse RPCResponse
	err := b.unmarshaler.Unmarshal(respData, &rpcResponse)
	if err != nil {
		return err
	}

	if rpcResponse.Error != nil {
		return rpcResponse.Error
	}

	test, err := b.marshaler.Marshal(rpcResponse.Result)
	err = b.unmarshaler.Unmarshal(test, dest)

	return nil
}
