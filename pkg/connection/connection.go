package connection

import (
	"github.com/surrealdb/surrealdb.go/internal/codec"
)

type Connection interface {
	Connect() error
	Close() error
	Send(method string, params []interface{}) (interface{}, error)
	Use(namespace string, database string) error
	Let(key string, value interface{}) error
	Unset(key string) error
}

type LiveHandler interface {
	LiveNotifications(id string) (chan Notification, error)
	Kill(id string) (interface{}, error)
}

type BaseConnection struct {
	marshaler   codec.Marshaler
	unmarshaler codec.Unmarshaler
	baseURL     string
}

type NewConnectionParams struct {
	Marshaler   codec.Marshaler
	Unmarshaler codec.Unmarshaler
	BaseURL     string
}
