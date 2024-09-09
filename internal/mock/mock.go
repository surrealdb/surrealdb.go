package mock

import (
	"errors"
	conn "github.com/surrealdb/surrealdb.go/internal/connection"
)

type ws struct {
}

func (w *ws) Connect(url string) (conn.Connection, error) {
	return w, nil
}

func (w *ws) Send(method string, params []interface{}) (interface{}, error) {
	return nil, nil
}

func (w *ws) Close() error {
	return nil
}

func (w *ws) LiveNotifications(id string) (chan conn.Notification, error) {
	return nil, errors.New("live queries are unimplemented for mocks")
}

func Create() *ws {
	return &ws{}
}
