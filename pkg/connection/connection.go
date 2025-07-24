package connection

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

type LiveHandler interface {
	Kill(id string) error
	Live(table models.Table, diff bool) (*models.UUID, error)
}

type Connection interface {
	Connect(ctx context.Context) error
	Close(ctx context.Context) error
	// Send sends a request to SurrealDB and expects a response.
	//
	// It requires `res` to be of type `*RPCResponse[T]` where T is a type that implements `cbor.Unmarshaller`,
	// or any type that `cbor.Unmarshal` can decode into.
	// The `method` is the SurrealDB method to call, and `params` are the parameters for the method.
	//
	// The `ctx` is used to cancel the request if the context is canceled.
	Send(ctx context.Context, res interface{}, method string, params ...interface{}) error
	Use(ctx context.Context, namespace string, database string) error
	Let(ctx context.Context, key string, value interface{}) error
	Unset(ctx context.Context, key string) error
	LiveNotifications(id string) (chan Notification, error)
	GetUnmarshaler() codec.Unmarshaler
}

type NewConnectionParams struct {
	Marshaler   codec.Marshaler
	Unmarshaler codec.Unmarshaler
	BaseURL     string
	Logger      logger.Logger

	URL url.URL

	// ReconnectInterval indicates the interval at which to automatically reconnect
	// to the SurrealDB server if the connection is considered lost.
	//
	// This is effective only when the connection is a WebSocket connection.
	// If the connection is an HTTP connection, this option is ignored.
	//
	// If this option is not set, the reconnection is disabled.
	ReconnectInterval time.Duration
}

type BaseConnection struct {
	BaseURL     string
	Marshaler   codec.Marshaler
	Unmarshaler codec.Unmarshaler
	Logger      logger.Logger

	ResponseChannels     map[string]chan RPCResponse[cbor.RawMessage]
	ResponseChannelsLock sync.RWMutex

	NotificationChannels     map[string]chan Notification
	NotificationChannelsLock sync.RWMutex
}

func (bc *BaseConnection) CreateResponseChannel(id string) (chan RPCResponse[cbor.RawMessage], error) {
	bc.ResponseChannelsLock.Lock()
	defer bc.ResponseChannelsLock.Unlock()

	if _, ok := bc.ResponseChannels[id]; ok {
		return nil, fmt.Errorf("%w: %v", constants.ErrIDInUse, id)
	}

	ch := make(chan RPCResponse[cbor.RawMessage]) // Buffered channel to avoid blocking on send
	bc.ResponseChannels[id] = ch

	return ch, nil
}

func (bc *BaseConnection) CreateNotificationChannel(liveQueryID string) (chan Notification, error) {
	bc.NotificationChannelsLock.Lock()
	defer bc.NotificationChannelsLock.Unlock()

	if _, ok := bc.NotificationChannels[liveQueryID]; ok {
		return nil, fmt.Errorf("%w: %v", constants.ErrIDInUse, liveQueryID)
	}

	ch := make(chan Notification)
	bc.NotificationChannels[liveQueryID] = ch

	return ch, nil
}

func (bc *BaseConnection) GetNotificationChannel(id string) (chan Notification, bool) {
	bc.NotificationChannelsLock.RLock()
	defer bc.NotificationChannelsLock.RUnlock()
	ch, ok := bc.NotificationChannels[id]

	return ch, ok
}

func (bc *BaseConnection) RemoveResponseChannel(id string) {
	bc.ResponseChannelsLock.Lock()
	defer bc.ResponseChannelsLock.Unlock()
	delete(bc.ResponseChannels, id)
}

func (bc *BaseConnection) GetResponseChannel(id string) (chan RPCResponse[cbor.RawMessage], bool) {
	bc.ResponseChannelsLock.RLock()
	defer bc.ResponseChannelsLock.RUnlock()
	ch, ok := bc.ResponseChannels[id]
	return ch, ok
}

func (bc *BaseConnection) PreConnectionChecks() error {
	if bc.BaseURL == "" {
		return constants.ErrNoBaseURL
	}

	if bc.Marshaler == nil {
		return constants.ErrNoMarshaler
	}

	if bc.Unmarshaler == nil {
		return constants.ErrNoUnmarshaler
	}

	return nil
}

func (bc *BaseConnection) LiveNotifications(liveQueryID string) (chan Notification, error) {
	c, err := bc.CreateNotificationChannel(liveQueryID)
	if err != nil {
		bc.Logger.Error(err.Error())
	}
	return c, err
}
