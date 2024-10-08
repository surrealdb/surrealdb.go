package connection

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go/v2/internal/rand"
	"github.com/surrealdb/surrealdb.go/v2/pkg/constants"
	"github.com/surrealdb/surrealdb.go/v2/pkg/logger"

	gorilla "github.com/gorilla/websocket"
)

type Option func(ws *WebSocketConnection) error

type WebSocketConnection struct {
	BaseConnection

	Conn     *gorilla.Conn
	connLock sync.Mutex
	Timeout  time.Duration
	Option   []Option
	logger   logger.Logger

	closeChan  chan int
	closeError error
}

func NewWebSocketConnection(p NewConnectionParams) *WebSocketConnection {
	return &WebSocketConnection{
		BaseConnection: BaseConnection{
			baseURL: p.BaseURL,

			marshaler:   p.Marshaler,
			unmarshaler: p.Unmarshaler,

			responseChannels:     make(map[string]chan []byte),
			notificationChannels: make(map[string]chan Notification),
		},

		Conn:      nil,
		closeChan: make(chan int),
		Timeout:   constants.DefaultTimeout * time.Second,
		logger:    logger.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

func (ws *WebSocketConnection) Connect() error {
	if err := ws.preConnectionChecks(); err != nil {
		return err
	}

	dialer := gorilla.DefaultDialer
	dialer.EnableCompression = true
	dialer.Subprotocols = append(dialer.Subprotocols, "cbor")

	connection, _, err := dialer.Dial(fmt.Sprintf("%s/rpc", ws.baseURL), nil)
	if err != nil {
		return err
	}

	ws.Conn = connection

	for _, option := range ws.Option {
		if err := option(ws); err != nil {
			return err
		}
	}

	go ws.initialize()
	return nil
}

func (ws *WebSocketConnection) SetTimeOut(timeout time.Duration) *WebSocketConnection {
	ws.Option = append(ws.Option, func(ws *WebSocketConnection) error {
		ws.Timeout = timeout
		return nil
	})
	return ws
}

// If path is empty it will use os.stdout/os.stderr
func (ws *WebSocketConnection) Logger(logData logger.Logger) *WebSocketConnection {
	ws.logger = logData
	return ws
}

func (ws *WebSocketConnection) RawLogger(logData logger.Logger) *WebSocketConnection {
	ws.logger = logData
	return ws
}

func (ws *WebSocketConnection) SetCompression(compress bool) *WebSocketConnection {
	ws.Option = append(ws.Option, func(ws *WebSocketConnection) error {
		ws.Conn.EnableWriteCompression(compress)
		return nil
	})
	return ws
}

func (ws *WebSocketConnection) Close() error {
	ws.connLock.Lock()
	defer ws.connLock.Unlock()
	close(ws.closeChan)
	err := ws.Conn.WriteMessage(gorilla.CloseMessage, gorilla.FormatCloseMessage(constants.CloseMessageCode, ""))
	if err != nil {
		return err
	}

	return ws.Conn.Close()
}

func (ws *WebSocketConnection) Use(namespace, database string) error {
	err := ws.Send(nil, "use", namespace, database)
	if err != nil {
		return err
	}

	return nil
}

func (ws *WebSocketConnection) Let(key string, value interface{}) error {
	return ws.Send(nil, "let", key, value)
}

func (ws *WebSocketConnection) Unset(key string) error {
	return ws.Send(nil, "unset", key)
}

func (ws *WebSocketConnection) Send(dest interface{}, method string, params ...interface{}) error {
	select {
	case <-ws.closeChan:
		return ws.closeError
	default:
	}

	id := rand.String(constants.RequestIDLength)
	request := &RPCRequest{
		ID:     id,
		Method: method,
		Params: params,
	}

	responseChan, err := ws.createResponseChannel(id)
	if err != nil {
		return err
	}
	defer ws.removeResponseChannel(id)

	if err := ws.write(request); err != nil {
		return err
	}
	timeout := time.After(ws.Timeout)

	select {
	case <-timeout:
		return constants.ErrTimeout
	case resBytes, open := <-responseChan:
		if !open {
			return errors.New("channel closed")
		}
		if dest != nil {
			return ws.unmarshaler.Unmarshal(resBytes, dest)
		}
		return nil
	}
}

func (ws *WebSocketConnection) write(v interface{}) error {
	data, err := ws.marshaler.Marshal(v)
	if err != nil {
		return err
	}

	ws.connLock.Lock()
	defer ws.connLock.Unlock()
	return ws.Conn.WriteMessage(gorilla.BinaryMessage, data)
}

func (ws *WebSocketConnection) initialize() {
	for {
		select {
		case <-ws.closeChan:
			return
		default:
			_, data, err := ws.Conn.ReadMessage()
			if err != nil {
				shouldExit := ws.handleError(err)
				if shouldExit {
					return
				}
				continue
			}
			go ws.handleResponse(data)
		}
	}
}

func (ws *WebSocketConnection) handleError(err error) bool {
	if errors.Is(err, net.ErrClosed) {
		ws.closeError = net.ErrClosed
		return true
	}
	if gorilla.IsUnexpectedCloseError(err) {
		ws.closeError = io.ErrClosedPipe
		<-ws.closeChan
		return true
	}

	ws.logger.Error(err.Error())
	return false
}

func (ws *WebSocketConnection) handleResponse(res []byte) {
	var rpcRes RPCResponse[interface{}]
	if err := ws.unmarshaler.Unmarshal(res, &rpcRes); err != nil {
		panic(err)
	}

	if rpcRes.Error != nil {
		err := fmt.Errorf("rpc request err %w", rpcRes.Error)
		ws.logger.Error(err.Error())
		return
	}

	if rpcRes.ID != nil && rpcRes.ID != "" {
		// Try to resolve message as response to query
		responseChan, ok := ws.getResponseChannel(fmt.Sprintf("%v", rpcRes.ID))
		if !ok {
			err := fmt.Errorf("unavailable ResponseChannel %+v", rpcRes.ID)
			ws.logger.Error(err.Error())
			return
		}
		defer close(responseChan)
		responseChan <- res
	} else {
		// todo: find a surefire way to confirm a notification

		var notificationRes RPCResponse[Notification]
		if err := ws.unmarshaler.Unmarshal(res, &notificationRes); err != nil {
			panic(err)
		}

		if notificationRes.Result.ID == nil {
			err := fmt.Errorf("response did not contain an 'id' field")
			ws.logger.Error(err.Error(), "result", fmt.Sprint(rpcRes.Result))
			return
		}

		channelID := notificationRes.Result.ID

		LiveNotificationChan, ok := ws.getLiveChannel(channelID.String())
		if !ok {
			err := fmt.Errorf("unavailable ResponseChannel %+v", channelID.String())
			ws.logger.Error(err.Error(), "result", fmt.Sprint(rpcRes.Result))
			return
		}

		var notification RPCResponse[Notification]
		if err := ws.unmarshaler.Unmarshal(res, &notification); err != nil {
			err := fmt.Errorf("error unmarshalling notification %+v", channelID.String())
			ws.logger.Error(err.Error(), "result", fmt.Sprint(rpcRes.Result))
			return
		}

		LiveNotificationChan <- *notification.Result
	}
}
