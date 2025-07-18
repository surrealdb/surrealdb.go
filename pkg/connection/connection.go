package connection

import (
	"fmt"
	"sync"

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
	Connect() error
	Close() error
	// Send requires `res` to be of type `*RPCResponse[T]` where T is a type that implements `cbor.Unmarshaller`.
	// It could be more obvious if Go allowed us to write it like:
	//   Send[T cbor.Unmarshaller](res *RPCResponse[T], method string, params ...interface{}) error
	// But it doesn't, so we have to use `interface{}`.
	// The caller is responsible for ensuring that `res` is of the correct type.
	Send(res interface{}, method string, params ...interface{}) error
	Use(namespace string, database string) error
	Let(key string, value interface{}) error
	Unset(key string) error
	LiveNotifications(id string) (chan Notification, error)
	GetUnmarshaler() codec.Unmarshaler
}

type NewConnectionParams struct {
	Marshaler   codec.Marshaler
	Unmarshaler codec.Unmarshaler
	BaseURL     string
	Logger      logger.Logger
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
