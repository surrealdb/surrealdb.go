package gws

import (
	"fmt"
	"sync"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
)

// LiveHandler is an alias for the connection.LiveHandler interface
type LiveHandler = connection.LiveHandler

// Connection is an alias for the connection.Connection interface
type Connection = connection.Connection

// NewConnectionParams is an alias for the connection.NewConnectionParams type
type NewConnectionParams = connection.NewConnectionParams

type BaseConnection struct {
	baseURL     string
	marshaler   codec.Marshaler
	unmarshaler codec.Unmarshaler
	logger      logger.Logger

	responseChannels     map[string]chan connection.RPCResponse[cbor.RawMessage]
	responseChannelsLock sync.RWMutex

	notificationChannels     map[string]chan connection.Notification
	notificationChannelsLock sync.RWMutex
}

func (bc *BaseConnection) createResponseChannel(id string) (chan connection.RPCResponse[cbor.RawMessage], error) {
	bc.responseChannelsLock.Lock()
	defer bc.responseChannelsLock.Unlock()

	if _, ok := bc.responseChannels[id]; ok {
		return nil, fmt.Errorf("%w: %v", constants.ErrIDInUse, id)
	}

	ch := make(chan connection.RPCResponse[cbor.RawMessage]) // Buffered channel to avoid blocking on send
	bc.responseChannels[id] = ch

	return ch, nil
}

func (bc *BaseConnection) createNotificationChannel(liveQueryID string) (chan connection.Notification, error) {
	bc.notificationChannelsLock.Lock()
	defer bc.notificationChannelsLock.Unlock()

	if _, ok := bc.notificationChannels[liveQueryID]; ok {
		return nil, fmt.Errorf("%w: %v", constants.ErrIDInUse, liveQueryID)
	}

	ch := make(chan connection.Notification)
	bc.notificationChannels[liveQueryID] = ch

	return ch, nil
}

func (bc *BaseConnection) getNotificationChannel(id string) (chan connection.Notification, bool) {
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

func (bc *BaseConnection) getResponseChannel(id string) (chan connection.RPCResponse[cbor.RawMessage], bool) {
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

func (bc *BaseConnection) LiveNotifications(liveQueryID string) (chan connection.Notification, error) {
	c, err := bc.createNotificationChannel(liveQueryID)
	if err != nil {
		bc.logger.Error(err.Error())
	}
	return c, err
}
