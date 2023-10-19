package mock

import (
	"errors"

	"github.com/surrealdb/surrealdb.go/pkg/conn"
	"github.com/surrealdb/surrealdb.go/pkg/model"
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

func (w *ws) LiveNotifications(id string) (chan model.Notification, error) {
	return nil, errors.New("live queries are unimplemented for mocks")
}

func Create() *ws {
	return &ws{}
}
