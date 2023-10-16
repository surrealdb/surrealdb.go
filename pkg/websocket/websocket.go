package websocket

import (
	"github.com/surrealdb/surrealdb.go/pkg/model"
)

type WebSocket interface {
	Connect(url string) (WebSocket, error)
	Send(method string, params []interface{}) (interface{}, error)
	Close() error
	LiveNotifications(id string) (chan model.Notification, error)
}
