package connection

import (
	"context"
	"fmt"
	"net/url"
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

// Tokens contains access and refresh tokens returned by SignInWithRefresh/SignUpWithRefresh.
// This is only returned when using TYPE RECORD access methods with WITH REFRESH enabled (SurrealDB v3+).
type Tokens struct {
	// Access is the JWT token used for authentication.
	// Use this with Authenticate() to establish a session on a new connection.
	Access string `cbor:"access"`
	// Refresh is the refresh token used to obtain new tokens without credentials.
	// Use this with SignInWithRefresh to get new Tokens.
	Refresh string `cbor:"refresh"`
}

type Connection interface {
	Connect(ctx context.Context) error
	Close(ctx context.Context) error
	// Send sends a request to SurrealDB and expects a response.
	//
	// The `method` is the SurrealDB method to call, and `params` are the parameters for the method.
	//
	// The `ctx` is used to cancel the request if the context is canceled.
	Send(ctx context.Context, method string, params ...any) (*RPCResponse[cbor.RawMessage], error)
	// Call sends a custom RPC request to SurrealDB and expects a response.
	// Unlike Send, Call accepts an RPCRequest directly, allowing you to set
	// Session and Txn fields for session-scoped or transaction-scoped operations (SurrealDB v3+).
	//
	// The `req` is the RPC request to send. The ID field will be set automatically if empty.
	// The `ctx` is used to cancel the request if the context is canceled.
	//
	// For HTTP connections, this method returns an error if req.Session or req.Txn is set,
	// as sessions and transactions require WebSocket connections.
	Call(ctx context.Context, req *RPCRequest) (*RPCResponse[cbor.RawMessage], error)
	Use(ctx context.Context, namespace string, database string) error
	Let(ctx context.Context, key string, value any) error
	Authenticate(ctx context.Context, token string) error
	SignUp(ctx context.Context, authData any) (string, error)
	SignUpWithRefresh(ctx context.Context, authData any) (*Tokens, error)
	SignIn(ctx context.Context, authData any) (string, error)
	SignInWithRefresh(ctx context.Context, authData any) (*Tokens, error)
	Invalidate(ctx context.Context) error
	Unset(ctx context.Context, key string) error
	LiveNotifications(id string) (chan Notification, error)
	CloseLiveNotifications(id string) error
	GetUnmarshaler() codec.Unmarshaler
}

// Config holds the configuration for a connection.
type Config struct {
	Marshaler   codec.Marshaler
	Unmarshaler codec.Unmarshaler
	BaseURL     string
	Logger      logger.Logger

	URL url.URL
}

// Toolkit contains common fields and methods that is useful to
// implement Connection interface for transports that use
// socket-like connections such as WebSocket.
type Toolkit struct {
	BaseURL     string
	Marshaler   codec.Marshaler
	Unmarshaler codec.Unmarshaler
	Logger      logger.Logger

	ResponseChannels     map[string]chan RPCResponse[cbor.RawMessage]
	ResponseChannelsLock sync.RWMutex
	// delivered holds the IDs of the response channels which have had their
	// response, under ResponseChannelsLock.
	delivered map[string]struct{}

	// responseStreams are the requests answered by many responses, such as
	// query_stream, by ID. ResponseChannelsLock guards it too.
	responseStreams map[string]*ResponseStream

	NotificationChannels     map[string]chan Notification
	NotificationChannelsLock sync.RWMutex
}

func (bc *Toolkit) CreateResponseChannel(id string) (chan RPCResponse[cbor.RawMessage], error) {
	bc.ResponseChannelsLock.Lock()
	defer bc.ResponseChannelsLock.Unlock()

	if _, ok := bc.ResponseChannels[id]; ok {
		return nil, fmt.Errorf("%w: %v", constants.ErrIDInUse, id)
	}
	// Responses for an ID go to its stream first, so it cannot also have a
	// one-shot channel.
	if _, ok := bc.responseStreams[id]; ok {
		return nil, fmt.Errorf("%w: %v", constants.ErrIDInUse, id)
	}

	// The channel holds the one response it will ever carry, so delivering it
	// never blocks the read loop, even when the caller has already given up.
	ch := make(chan RPCResponse[cbor.RawMessage], 1)
	bc.ResponseChannels[id] = ch

	return ch, nil
}

func (bc *Toolkit) CreateNotificationChannel(liveQueryID string) (chan Notification, error) {
	bc.NotificationChannelsLock.Lock()
	defer bc.NotificationChannelsLock.Unlock()

	if _, ok := bc.NotificationChannels[liveQueryID]; ok {
		return nil, fmt.Errorf("%w: %v", constants.ErrIDInUse, liveQueryID)
	}

	ch := make(chan Notification, 16)
	bc.NotificationChannels[liveQueryID] = ch

	return ch, nil
}

func (bc *Toolkit) GetNotificationChannel(id string) (chan Notification, bool) {
	bc.NotificationChannelsLock.RLock()
	defer bc.NotificationChannelsLock.RUnlock()
	ch, ok := bc.NotificationChannels[id]

	return ch, ok
}

// RemoveResponseChannel releases the response channel for id, which the call
// that created it does once it is done with it. Until then the id stays taken,
// even after its response is delivered, so another call cannot register it and
// then lose its channel to this one's cleanup.
func (bc *Toolkit) RemoveResponseChannel(id string) {
	bc.ResponseChannelsLock.Lock()
	defer bc.ResponseChannelsLock.Unlock()
	delete(bc.ResponseChannels, id)
	delete(bc.delivered, id)
}

// DeliverResponse hands res to the caller waiting on the response channel for id.
//
// The channel is marked delivered in the same critical section that finds it,
// so a response is delivered at most once, and the send cannot block: the channel
// is buffered for exactly one response. A caller that timed out and stopped
// listening therefore never stalls the goroutine delivering responses.
//
// A response to a request answered by many, registered with
// OpenResponseStream, is added to its stream instead.
//
// It returns false if no caller is waiting for id.
func (bc *Toolkit) DeliverResponse(id string, res RPCResponse[cbor.RawMessage]) bool {
	if bc.deliverToStream(id, res) {
		return true
	}

	bc.ResponseChannelsLock.Lock()
	ch, ok := bc.ResponseChannels[id]
	if ok {
		if _, done := bc.delivered[id]; done {
			ok = false
		} else {
			if bc.delivered == nil {
				bc.delivered = make(map[string]struct{})
			}
			bc.delivered[id] = struct{}{}
		}
	}
	bc.ResponseChannelsLock.Unlock()

	if !ok {
		return false
	}

	select {
	case ch <- res:
	default:
		// Unreachable while every response channel comes from CreateResponseChannel,
		// but a full channel must never block the caller.
	}
	return true
}

func (bc *Toolkit) GetResponseChannel(id string) (chan RPCResponse[cbor.RawMessage], bool) {
	bc.ResponseChannelsLock.RLock()
	defer bc.ResponseChannelsLock.RUnlock()
	ch, ok := bc.ResponseChannels[id]
	return ch, ok
}

func (bc *Config) Validate() error {
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

func (bc *Toolkit) LiveNotifications(liveQueryID string) (chan Notification, error) {
	c, err := bc.CreateNotificationChannel(liveQueryID)
	if err != nil {
		bc.Logger.Error(err.Error())
	}
	return c, err
}

func (bc *Toolkit) CloseLiveNotifications(liveQueryID string) error {
	bc.NotificationChannelsLock.Lock()
	defer bc.NotificationChannelsLock.Unlock()

	ch, ok := bc.NotificationChannels[liveQueryID]
	if !ok {
		return fmt.Errorf("notification channel not found for live query ID: %s", liveQueryID)
	}

	close(ch)
	delete(bc.NotificationChannels, liveQueryID)
	return nil
}

// SendNotification safely sends a notification to the channel identified by id.
// It holds the read lock during the send to prevent CloseLiveNotifications from
// closing the channel concurrently, which would cause a "send on closed channel" panic.
// Returns true if the notification was sent, false if the channel was not found
// or was full (dropped).
func (bc *Toolkit) SendNotification(id string, notification Notification) bool {
	bc.NotificationChannelsLock.RLock()
	ch, ok := bc.NotificationChannels[id]
	if !ok {
		bc.NotificationChannelsLock.RUnlock()
		return false
	}

	select {
	case ch <- notification:
		bc.NotificationChannelsLock.RUnlock()
		return true
	default:
		// Channel is full, drop the notification to avoid blocking
		// while holding the read lock.
		bc.NotificationChannelsLock.RUnlock()
		return false
	}
}
