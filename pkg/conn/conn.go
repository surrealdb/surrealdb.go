package conn

import (
	"context"

	"github.com/surrealdb/surrealdb.go/pkg/model"
)

type Connection interface {
	Connect(ctx context.Context, url string) (Connection, error)
	Send(ctx context.Context, method string, params []interface{}) (interface{}, error)
	Close() error
	LiveNotifications(ctx context.Context, id string) (chan model.Notification, error)
}
