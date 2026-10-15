package connection

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/fxamacker/cbor/v2"

	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

// Streamer is implemented by connections which can send a request that is
// answered by a sequence of responses rather than one, such as query_stream.
type Streamer interface {
	// CallStream sends the request and returns the stream its responses arrive
	// on. The caller must Close the stream when it is done with it.
	CallStream(ctx context.Context, req *RPCRequest) (*ResponseStream, error)
}

// ErrStreamClosed is returned by ResponseStream.Next after Close.
var ErrStreamClosed = errors.New("response stream closed")

// ResponseStream holds the responses to one request answered by many, in the
// order they arrive. It buffers them without bound, so the goroutine reading
// the socket never waits on whoever reads the stream.
type ResponseStream struct {
	// ID is the ID of the request.
	ID string

	mu       sync.Mutex
	queue    []RPCResponse[cbor.RawMessage]
	released bool
	wake     chan struct{}

	// connClosed is closed with the connection, and connErr says why.
	connClosed <-chan struct{}
	connErr    func() error
	// release unregisters the stream from its connection.
	release func()
}

// NewResponseStream returns a stream for the request id. connClosed is closed
// when the connection is, and connErr then returns why; release is called once
// when the stream is closed.
func NewResponseStream(id string, connClosed <-chan struct{}, connErr func() error, release func()) *ResponseStream {
	return &ResponseStream{
		ID:         id,
		wake:       make(chan struct{}, 1),
		connClosed: connClosed,
		connErr:    connErr,
		release:    release,
	}
}

// push adds a response to the stream. It returns false once the stream is closed.
func (s *ResponseStream) push(res RPCResponse[cbor.RawMessage]) bool {
	s.mu.Lock()
	if s.released {
		s.mu.Unlock()
		return false
	}
	s.queue = append(s.queue, res)
	s.mu.Unlock()

	select {
	case s.wake <- struct{}{}:
	default:
	}
	return true
}

// Next returns the next response, waiting for it until ctx is done or the
// connection closes.
func (s *ResponseStream) Next(ctx context.Context) (RPCResponse[cbor.RawMessage], error) {
	for {
		s.mu.Lock()
		if s.released {
			s.mu.Unlock()
			return RPCResponse[cbor.RawMessage]{}, ErrStreamClosed
		}
		if len(s.queue) > 0 {
			res := s.queue[0]
			s.queue[0] = RPCResponse[cbor.RawMessage]{}
			s.queue = s.queue[1:]
			s.mu.Unlock()
			return res, nil
		}
		s.mu.Unlock()

		select {
		case <-s.wake:
		case <-ctx.Done():
			return RPCResponse[cbor.RawMessage]{}, ctx.Err()
		case <-s.connClosed:
			// Deliver what arrived before the connection closed first.
			s.mu.Lock()
			pending := len(s.queue) > 0
			s.mu.Unlock()
			if pending {
				continue
			}
			err := s.connErr()
			if err == nil {
				err = errors.New("connection closed")
			}
			return RPCResponse[cbor.RawMessage]{}, err
		}
	}
}

// Close unregisters the stream and drops the responses it still holds.
func (s *ResponseStream) Close() {
	s.mu.Lock()
	if s.released {
		s.mu.Unlock()
		return
	}
	s.released = true
	s.queue = nil
	s.mu.Unlock()
	s.release()
}

// OpenResponseStream registers a stream for the request id, to which
// DeliverResponse hands every response for id until the stream is closed.
func (bc *Toolkit) OpenResponseStream(id string, connClosed <-chan struct{}, connErr func() error) (*ResponseStream, error) {
	bc.ResponseChannelsLock.Lock()
	defer bc.ResponseChannelsLock.Unlock()

	if _, ok := bc.ResponseChannels[id]; ok {
		return nil, fmt.Errorf("%w: %v", constants.ErrIDInUse, id)
	}
	if bc.responseStreams == nil {
		bc.responseStreams = make(map[string]*ResponseStream)
	}
	if _, ok := bc.responseStreams[id]; ok {
		return nil, fmt.Errorf("%w: %v", constants.ErrIDInUse, id)
	}

	s := NewResponseStream(id, connClosed, connErr, func() {
		bc.ResponseChannelsLock.Lock()
		defer bc.ResponseChannelsLock.Unlock()
		delete(bc.responseStreams, id)
	})
	bc.responseStreams[id] = s
	return s, nil
}

// deliverToStream hands res to the stream registered for id, if there is one.
func (bc *Toolkit) deliverToStream(id string, res RPCResponse[cbor.RawMessage]) bool {
	bc.ResponseChannelsLock.RLock()
	s, ok := bc.responseStreams[id]
	bc.ResponseChannelsLock.RUnlock()
	if !ok {
		return false
	}
	// A stream closed in the meantime has nobody to read what arrives for it.
	s.push(res)
	return true
}
