package surrealdb

import (
	"context"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/internal/codec"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// scriptedStreamer answers query_stream with fixed frames, and records the
// other requests it is sent. Methods it does not override panic.
type scriptedStreamer struct {
	connection.Connection
	tk     connection.Toolkit
	codec  *surrealcbor.Codec
	frames []map[string]any
	calls  chan *connection.RPCRequest
}

func newScriptedStreamer(frames ...map[string]any) *scriptedStreamer {
	return &scriptedStreamer{
		tk: connection.Toolkit{
			ResponseChannels:     map[string]chan connection.RPCResponse[cbor.RawMessage]{},
			NotificationChannels: map[string]chan connection.Notification{},
		},
		codec:  surrealcbor.New(),
		frames: frames,
		calls:  make(chan *connection.RPCRequest, 8),
	}
}

func (f *scriptedStreamer) GetUnmarshaler() codec.Unmarshaler { return f.codec }

func (f *scriptedStreamer) Call(_ context.Context, req *connection.RPCRequest) (*connection.RPCResponse[cbor.RawMessage], error) {
	f.calls <- req
	return &connection.RPCResponse[cbor.RawMessage]{}, nil
}

func (f *scriptedStreamer) CallStream(_ context.Context, req *connection.RPCRequest) (*connection.ResponseStream, error) {
	id := "stream-1"
	req.ID = id
	s, err := f.tk.OpenResponseStream(id, make(chan struct{}), func() error { return nil })
	if err != nil {
		return nil, err
	}
	for _, frame := range f.frames {
		b, err := f.codec.Marshal(frame)
		if err != nil {
			return nil, err
		}
		raw := cbor.RawMessage(b)
		f.tk.DeliverResponse(id, connection.RPCResponse[cbor.RawMessage]{ID: id, Result: &raw})
	}
	return s, nil
}

func TestQueryStream_cancelsAfterAProtocolError(t *testing.T) {
	con := newScriptedStreamer(map[string]any{"stream": "begin", "statements": 1, "version": 2})
	db := &DB{con: con}

	var got error
	for _, err := range QueryStream[any](context.Background(), db, "SELECT * FROM big", nil) {
		got = err
	}
	require.ErrorIs(t, got, ErrStreamProtocol)

	select {
	case req := <-con.calls:
		assert.Equal(t, "query_cancel", req.Method)
		assert.Equal(t, []any{"stream-1"}, req.Params, "the query the client gave up on is canceled")
	case <-time.After(5 * time.Second):
		t.Fatal("the query was not canceled on the server")
	}
}

func TestQueryStream_noCancelAfterTheEnd(t *testing.T) {
	con := newScriptedStreamer(
		map[string]any{"stream": "begin", "statements": 1, "version": 1},
		map[string]any{"stream": "finished", "index": 0},
		map[string]any{"stream": "end", "results": 1},
	)
	db := &DB{con: con}

	for _, err := range QueryStream[any](context.Background(), db, "RETURN []", nil) {
		require.NoError(t, err)
	}

	select {
	case req := <-con.calls:
		t.Fatalf("a finished query was sent %s", req.Method)
	case <-time.After(200 * time.Millisecond):
	}
}
