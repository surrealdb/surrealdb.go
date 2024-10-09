package connection

import (
	"fmt"
	"sync"

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
	Send(res interface{}, method string, params ...interface{}) error
	Use(namespace string, database string) error
	Let(key string, value interface{}) error
	Unset(key string) error
	LiveNotifications(id string) (chan Notification, error)
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

	responseChannels     map[string]chan []byte
	responseChannelsLock sync.RWMutex

	notificationChannels     map[string]chan Notification
	notificationChannelsLock sync.RWMutex
}

func (bc *BaseConnection) createResponseChannel(id string) (chan []byte, error) {
	bc.responseChannelsLock.Lock()
	defer bc.responseChannelsLock.Unlock()

	if _, ok := bc.responseChannels[id]; ok {
		return nil, fmt.Errorf("%w: %v", constants.ErrIDInUse, id)
	}

	ch := make(chan []byte)
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

func (bc *BaseConnection) removeResponseChannel(id string) {
	bc.responseChannelsLock.Lock()
	defer bc.responseChannelsLock.Unlock()
	delete(bc.responseChannels, id)
}

func (bc *BaseConnection) getResponseChannel(id string) (chan []byte, bool) {
	bc.responseChannelsLock.RLock()
	defer bc.responseChannelsLock.RUnlock()
	ch, ok := bc.responseChannels[id]
	return ch, ok
}

func (bc *BaseConnection) getLiveChannel(id string) (chan Notification, bool) {
	bc.notificationChannelsLock.RLock()
	defer bc.notificationChannelsLock.RUnlock()
	ch, ok := bc.notificationChannels[id]

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
