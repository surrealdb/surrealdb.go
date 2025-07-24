package gws

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/lxzan/gws"
	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/internal/rand"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

type GwsConnection struct {
	BaseConnection

	conn     *gws.Conn
	connLock sync.Mutex

	Timeout time.Duration

	connCloseCh    chan struct{}
	connCloseError error

	handler *websocketHandler
}

var _ connection.Connection = (*GwsConnection)(nil)

type websocketHandler struct {
	conn *GwsConnection
}

func (h *websocketHandler) OnOpen(socket *gws.Conn) {
	// Connection opened successfully
}

func (h *websocketHandler) OnClose(socket *gws.Conn, err error) {
	h.conn.connLock.Lock()
	defer h.conn.connLock.Unlock()

	if h.conn.connCloseCh != nil {
		select {
		case <-h.conn.connCloseCh:
			// Already closed
		default:
			close(h.conn.connCloseCh)
			h.conn.connCloseError = err
		}
	}
}

func (h *websocketHandler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	h.conn.handleResponse(message.Bytes())
}

func (h *websocketHandler) OnPing(socket *gws.Conn, payload []byte) {

}

func (h *websocketHandler) OnPong(socket *gws.Conn, payload []byte) {

}

func (c *GwsConnection) write(v interface{}) error {
	data, err := c.marshaler.Marshal(v)
	if err != nil {
		return err
	}

	c.connLock.Lock()
	defer c.connLock.Unlock()

	if c.conn == nil {
		return errors.New("connection is closed")
	}

	return c.conn.WriteMessage(gws.OpcodeBinary, data)
}

func (c *GwsConnection) handleResponse(data []byte) {
	var rpcRes connection.RPCResponse[cbor.RawMessage]
	if err := c.unmarshaler.Unmarshal(data, &rpcRes); err != nil {
		return
	}

	if rpcRes.ID != nil && rpcRes.ID != "" {
		// Handle RPC response
		responseChan, ok := c.getResponseChannel(fmt.Sprintf("%v", rpcRes.ID))
		if !ok {
			return
		}
		defer close(responseChan)
		responseChan <- rpcRes
	} else {
		// Handle notification
		notificationRes, err := rpcRes.Result.MarshalCBOR()
		if err != nil {
			return
		}

		var notification connection.Notification
		if err := c.unmarshaler.Unmarshal(notificationRes, &notification); err != nil {
			return
		}

		if notification.ID == nil {
			return
		}

		notificationChan, ok := c.getNotificationChannel(notification.ID.String())
		if !ok {
			return
		}

		notificationChan <- notification
	}
}

func (c *GwsConnection) unmarshalRes(res connection.RPCResponse[cbor.RawMessage], dest interface{}) error {
	return unmarshalRes(c.unmarshaler, res, dest)
}

func unmarshalRes(unmarshaler codec.Unmarshaler, res connection.RPCResponse[cbor.RawMessage], dest interface{}) error {
	rawCBORBytes, err := res.Result.MarshalCBOR()
	if err != nil {
		return fmt.Errorf("Send: error marshaling result: %w", err)
	}

	kind := reflect.TypeOf(dest).Kind()
	if kind != reflect.Ptr {
		return fmt.Errorf("Send: dest must be a pointer, got %T", dest)
	}

	const (
		FieldID     = "ID"
		FieldResult = "Result"
	)

	var destStruct reflect.Value
	switch structOrIfacePtrStruct := reflect.ValueOf(dest).Elem(); structOrIfacePtrStruct.Kind() {
	case reflect.Interface:
		ptrStruct := structOrIfacePtrStruct.Elem()
		if ptrStruct.Kind() == reflect.Ptr {
			destStruct = ptrStruct.Elem()
		} else {
			return fmt.Errorf("Send: dest must be a pointer to a struct, got %T", dest)
		}
	case reflect.Struct:
		destStruct = structOrIfacePtrStruct
	default:
		return fmt.Errorf("Send: dest must be a pointer to a struct or an interface, got %T", dest)
	}

	if res.ID != nil {
		destStruct.FieldByName(FieldID).Set(reflect.ValueOf(res.ID))
	}

	destStructDotResult := destStruct.FieldByName(FieldResult).Interface()

	if destStructDotResult == nil || reflect.ValueOf(destStructDotResult).IsNil() {
		destStructDotResult = reflect.New(destStruct.FieldByName(FieldResult).Type().Elem()).Interface()
		destStruct.FieldByName(FieldResult).Set(reflect.ValueOf(destStructDotResult))
	}

	if err := unmarshaler.Unmarshal(rawCBORBytes, destStructDotResult); err != nil {
		return fmt.Errorf("Send: error unmarshaling result: %w", err)
	}

	return nil
}

func eliminateTypedNilError(err error) error {
	if err == nil || reflect.ValueOf(err).IsNil() {
		return nil
	}
	return err
}

func New(params connection.NewConnectionParams) *GwsConnection {
	conn := &GwsConnection{
		Timeout: constants.DefaultWSTimeout,
	}
	conn.baseURL = params.BaseURL
	conn.marshaler = params.Marshaler
	conn.unmarshaler = params.Unmarshaler
	conn.responseChannels = make(map[string]chan connection.RPCResponse[cbor.RawMessage])
	conn.notificationChannels = make(map[string]chan connection.Notification)

	return conn
}

// SetTimeout sets the timeout for RPC responses
func (c *GwsConnection) SetTimeout(timeout time.Duration) *GwsConnection {
	c.Timeout = timeout
	return c
}

// Close implements connection.Connection.
func (c *GwsConnection) Close(ctx context.Context) error {
	c.connLock.Lock()
	defer c.connLock.Unlock()

	if c.conn == nil {
		return nil
	}

	// Signal closing
	if c.connCloseCh != nil {
		select {
		case <-c.connCloseCh:
			// Already closed
		default:
			close(c.connCloseCh)
		}
	}

	err := c.conn.WriteClose(constants.CloseMessageCode, []byte(""))
	if err != nil {
		// Log error but continue with close
	}

	c.conn.NetConn().Close()
	c.conn = nil

	return nil
}

// Connect implements connection.Connection.
func (c *GwsConnection) Connect(ctx context.Context) error {
	if err := c.preConnectionChecks(); err != nil {
		return err
	}

	c.handler = &websocketHandler{conn: c}

	option := &gws.ClientOption{
		Addr: fmt.Sprintf("%s/rpc", c.baseURL),
		RequestHeader: http.Header{
			"Sec-WebSocket-Protocol": []string{"cbor"},
		},
		PermessageDeflate: gws.PermessageDeflate{
			Enabled: true,
		},
	}

	conn, _, err := gws.NewClient(c.handler, option)
	if err != nil {
		return err
	}

	c.connLock.Lock()
	defer c.connLock.Unlock()

	c.conn = conn
	c.connCloseCh = make(chan struct{})

	// Start reading messages
	go conn.ReadLoop()

	return nil
}

// GetUnmarshaler implements connection.Connection.
func (c *GwsConnection) GetUnmarshaler() codec.Unmarshaler {
	return c.unmarshaler
}

// Let implements connection.Connection.
func (c *GwsConnection) Let(ctx context.Context, key string, value interface{}) error {
	return c.Send(ctx, nil, "let", key, value)
}

// LiveNotifications implements connection.Connection.
func (c *GwsConnection) LiveNotifications(id string) (chan connection.Notification, error) {
	return c.createNotificationChannel(id)
}

// Send implements connection.Connection.
func (c *GwsConnection) Send(ctx context.Context, res interface{}, method string, params ...interface{}) error {
	if c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	select {
	case <-c.connCloseCh:
		return c.connCloseError
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	id := rand.String(constants.RequestIDLength)
	request := &connection.RPCRequest{
		ID:     id,
		Method: method,
		Params: params,
	}

	responseChan, err := c.createResponseChannel(id)
	if err != nil {
		return err
	}
	defer c.removeResponseChannel(id)

	if err := c.write(request); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case rpcRes, open := <-responseChan:
		if !open {
			return errors.New("response channel closed")
		}

		if res == nil || rpcRes.Result == nil || rpcRes.Error != nil {
			return eliminateTypedNilError(rpcRes.Error)
		}

		if err := c.unmarshalRes(rpcRes, res); err != nil {
			return fmt.Errorf("error unmarshaling response: %w", err)
		}

		return eliminateTypedNilError(rpcRes.Error)
	}
}

// Unset implements connection.Connection.
func (c *GwsConnection) Unset(ctx context.Context, key string) error {
	return c.Send(ctx, nil, "unset", key)
}

// Use implements connection.Connection.
func (c *GwsConnection) Use(ctx context.Context, namespace string, database string) error {
	return c.Send(ctx, nil, "use", namespace, database)
}
