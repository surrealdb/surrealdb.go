package connection

import (
	"github.com/surrealdb/surrealdb.go/v2/internal/codec"
	"github.com/surrealdb/surrealdb.go/v2/pkg/logger"
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
	Logger      logger.Logger
}

type BaseConnection struct {
	marshaler   codec.Marshaler
	unmarshaler codec.Unmarshaler
	baseURL     string
}
