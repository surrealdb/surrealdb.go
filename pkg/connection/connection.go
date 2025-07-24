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
	baseURL     string
	marshaler   codec.Marshaler
	unmarshaler codec.Unmarshaler
	logger      logger.Logger

	responseChannels     map[string]chan RPCResponse[cbor.RawMessage]
	responseChannelsLock sync.RWMutex

	notificationChannels     map[string]chan Notification
	notificationChannelsLock sync.RWMutex
}

func (bc *BaseConnection) createResponseChannel(id string) (chan RPCResponse[cbor.RawMessage], error) {
	bc.responseChannelsLock.Lock()
	defer bc.responseChannelsLock.Unlock()

	if _, ok := bc.responseChannels[id]; ok {
		return nil, fmt.Errorf("%w: %v", constants.ErrIDInUse, id)
	}

	ch := make(chan RPCResponse[cbor.RawMessage]) // Buffered channel to avoid blocking on send
	bc.responseChannels[id] = ch

	return ch, nil
}

func (bc *BaseConnection) createNotificationChannel(liveQueryID string) (chan Notification, error) {
	bc.notificationChannelsLock.Lock()
	defer bc.notificationChannelsLock.Unlock()

	if _, ok := bc.notificationChannels[liveQueryID]; ok {
		return nil, fmt.Errorf("%w: %v", constants.ErrIDInUse, liveQueryID)
	}

	ch := make(chan Notification)
	bc.notificationChannels[liveQueryID] = ch

	return ch, nil
}

func (bc *BaseConnection) getNotificationChannel(id string) (chan Notification, bool) {
	bc.notificationChannelsLock.RLock()
	defer bc.notificationChannelsLock.RUnlock()
	ch, ok := bc.notificationChannels[id]

	return ch, ok
}

func (bc *BaseConnection) removeResponseChannel(id string) {
	bc.responseChannelsLock.Lock()
	defer bc.responseChannelsLock.Unlock()
	delete(bc.responseChannels, id)
}

func (bc *BaseConnection) getResponseChannel(id string) (chan RPCResponse[cbor.RawMessage], bool) {
	bc.responseChannelsLock.RLock()
	defer bc.responseChannelsLock.RUnlock()
	ch, ok := bc.responseChannels[id]
	return ch, ok
}

func (bc *BaseConnection) preConnectionChecks() error {
	if bc.baseURL == "" {
		return constants.ErrNoBaseURL
	}

	if bc.marshaler == nil {
		return constants.ErrNoMarshaler
	}

	if bc.unmarshaler == nil {
		return constants.ErrNoUnmarshaler
	}

	return nil
}

func (bc *BaseConnection) LiveNotifications(liveQueryID string) (chan Notification, error) {
	c, err := bc.createNotificationChannel(liveQueryID)
	if err != nil {
		bc.logger.Error(err.Error())
	}
	return c, err
}
