package connection

import (
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/model"
)

type Connection interface {
	Connect() error
	Close() error
	Send(method string, params []interface{}) (interface{}, error)
	Use(namespace string, database string) error
	SignIn(auth model.Auth) (string, error)
}

type LiveHandler interface {
	LiveNotifications(id string) (chan Notification, error)
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
