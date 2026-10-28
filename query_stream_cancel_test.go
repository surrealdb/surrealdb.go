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
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// scriptedStreamer answers query_stream with fixed frames, and records the
// other requests it is sent. Methods it does not override panic.
type scriptedStreamer struct {
	connection.Connection
	tk     connection.Toolkit
	codec  *surrealcbor.Codec
	frames []map[string]any
	// onCancel are the frames sent once the stream is canceled.
	onCancel []map[string]any
	calls    chan *connection.RPCRequest
	stream   *connection.ResponseStream
	// timeout is the stream's FirstResponseTimeout.
	timeout time.Duration
	// opened is closed once the stream is opened.
	opened chan struct{}
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
		opened: make(chan struct{}),
	}
}

func (f *scriptedStreamer) GetUnmarshaler() codec.Unmarshaler { return f.codec }

func (f *scriptedStreamer) Call(_ context.Context, req *connection.RPCRequest) (*connection.RPCResponse[cbor.RawMessage], error) {
	f.calls <- req
	if req.Method == methodQueryCancel {
		if err := f.deliver(f.onCancel); err != nil {
			return nil, err
		}
	}
	return &connection.RPCResponse[cbor.RawMessage]{}, nil
}

func (f *scriptedStreamer) deliver(frames []map[string]any) error {
	for _, frame := range frames {
		b, err := f.codec.Marshal(frame)
		if err != nil {
			return err
		}
		raw := cbor.RawMessage(b)
		f.tk.DeliverResponse(f.stream.ID, connection.RPCResponse[cbor.RawMessage]{ID: f.stream.ID, Result: &raw})
	}
	return nil
}

func (f *scriptedStreamer) CallStream(_ context.Context, req *connection.RPCRequest) (*connection.ResponseStream, error) {
	id := "stream-1"
	req.ID = id
	s, err := f.tk.OpenResponseStream(id, make(chan struct{}), func() error { return nil })
	if err != nil {
		return nil, err
	}
	s.FirstResponseTimeout = f.timeout
	f.stream = s
	if err := f.deliver(f.frames); err != nil {
		return nil, err
	}
	close(f.opened)
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

func TestQueryStream_transactionWaitsForTheCanceledQuery(t *testing.T) {
	con := newScriptedStreamer(
		map[string]any{"stream": "begin", "statements": 1, "version": 1},
		map[string]any{"stream": "rows", "index": 0, "values": []any{1, 2}},
	)
	con.onCancel = []map[string]any{
		{"stream": "rows", "index": 0, "values": []any{3}},
		{"stream": "end", "results": 1},
	}
	tx := &Transaction{db: &DB{con: con}, id: &models.UUID{}}

	for _, err := range QueryStream[int](context.Background(), tx, "SELECT * FROM big", nil) {
		require.NoError(t, err)
		break
	}

	// By the time the loop is left, the query was canceled and its end frame
	// read, so a Commit sent now comes after it.
	select {
	case req := <-con.calls:
		assert.Equal(t, "query_cancel", req.Method)
	default:
		t.Fatal("leaving the loop did not wait for the query to be canceled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := con.stream.Next(ctx)
	require.ErrorIs(t, err, connection.ErrStreamClosed, "the stream was drained to its end frame")
}

// A server which stops answering is caught by the connection's timeout while
// the stream waits for its first frame, even when ctx has no deadline.
func TestQueryStream_timesOutWaitingForTheFirstFrame(t *testing.T) {
	con := newScriptedStreamer()
	con.timeout = 50 * time.Millisecond
	db := &DB{con: con}

	done := make(chan error, 1)
	go func() {
		var got error
		for _, err := range QueryStream[any](context.Background(), db, "SELECT * FROM big", nil) {
			got = err
		}
		done <- got
	}()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(5 * time.Second):
		t.Fatal("the stream did not time out waiting for its first frame")
	}
}

// After the first frame the timeout no longer applies, as a statement can
// run for longer than it without sending anything.
func TestQueryStream_noTimeoutAfterTheFirstFrame(t *testing.T) {
	con := newScriptedStreamer(map[string]any{"stream": "begin", "statements": 1, "version": 1})
	con.timeout = 50 * time.Millisecond
	db := &DB{con: con}

	go func() {
		<-con.opened
		time.Sleep(4 * con.timeout)
		_ = con.deliver([]map[string]any{
			{"stream": "value", "index": 0, "value": 1},
			{"stream": "finished", "index": 0, "single": true},
			{"stream": "end", "results": 1},
		})
	}()

	var values []any
	for frame, err := range QueryStream[any](context.Background(), db, "SLEEP 200ms; RETURN 1", nil) {
		require.NoError(t, err)
		values = append(values, frame.Value)
	}
	assert.Equal(t, []any{uint64(1)}, values)
}
