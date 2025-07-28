package connection

type WebSocketConnection interface {
	Connection

	IsClosed() bool
}
