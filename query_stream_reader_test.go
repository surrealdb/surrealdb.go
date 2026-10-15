package surrealdb

import (
	"errors"
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/surrealcbor"
)

// readFrames feeds frames to a stream reader as the server would send them,
// and returns what it yields until a frame ends the stream.
func readFrames[T any](t *testing.T, frames ...map[string]any) (out []StreamFrame[T], ended bool, err error) {
	t.Helper()
	codec := surrealcbor.New()
	r := newStreamReader[T](codec.Unmarshal)
	for _, f := range frames {
		b, merr := codec.Marshal(f)
		require.NoError(t, merr)
		raw := cbor.RawMessage(b)
		got, last, rerr := r.read(&raw)
		out = append(out, got...)
		if rerr != nil || last {
			return out, last, rerr
		}
	}
	return out, false, nil
}

func TestStreamReader(t *testing.T) {
	begin := map[string]any{"stream": "begin", "statements": 2, "version": 1}

	t.Run("rows then a single value", func(t *testing.T) {
		frames, ended, err := readFrames[int](t,
			begin,
			map[string]any{"stream": "rows", "index": 0, "values": []int{1, 2}},
			map[string]any{"stream": "rows", "index": 0, "values": []int{3}},
			map[string]any{"stream": "finished", "index": 0, "single": false, "time": "1ms"},
			map[string]any{"stream": "value", "index": 1, "value": 9},
			map[string]any{"stream": "finished", "index": 1, "single": true, "time": "2ms"},
			map[string]any{"stream": "end", "results": 2},
		)
		require.NoError(t, err)
		assert.True(t, ended)
		assert.Equal(t, []StreamFrame[int]{
			{Statement: 0, Rows: []int{1, 2}},
			{Statement: 0, Rows: []int{3}},
			{Statement: 0, Done: true, Time: "1ms"},
			{Statement: 1, Done: true, Single: true, Value: 9, Time: "2ms"},
		}, frames)
	})

	t.Run("a failed statement retracts its rows", func(t *testing.T) {
		frames, ended, err := readFrames[int](t,
			begin,
			map[string]any{"stream": "rows", "index": 0, "values": []int{1}},
			map[string]any{"stream": "finished", "index": 0, "error": map[string]any{"code": -32000, "message": "failed"}},
			map[string]any{"stream": "end", "results": 1},
		)
		require.NoError(t, err)
		assert.True(t, ended)
		require.Len(t, frames, 2)
		var qe *QueryError
		require.ErrorAs(t, frames[1].Err, &qe)
		assert.Equal(t, "failed", qe.Message)
		assert.True(t, frames[1].Done)
	})

	t.Run("an end error ends the stream", func(t *testing.T) {
		frames, ended, err := readFrames[int](t,
			begin,
			map[string]any{"stream": "rows", "index": 0, "values": []int{1}},
			map[string]any{"stream": "end", "error": map[string]any{"code": -32000, "message": "aborted"}},
		)
		assert.True(t, ended)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "aborted")
		assert.Len(t, frames, 1, "the rows already yielded stay unconfirmed")
	})

	t.Run("frames after a statement finished are dropped", func(t *testing.T) {
		frames, _, err := readFrames[int](t,
			begin,
			map[string]any{"stream": "finished", "index": 0},
			map[string]any{"stream": "rows", "index": 0, "values": []int{1}},
			map[string]any{"stream": "finished", "index": 0},
		)
		require.NoError(t, err)
		assert.Equal(t, []StreamFrame[int]{{Statement: 0, Done: true}}, frames)
	})

	t.Run("unknown frames are skipped", func(t *testing.T) {
		frames, _, err := readFrames[int](t,
			begin,
			map[string]any{"stream": "progress", "index": 0},
		)
		require.NoError(t, err)
		assert.Empty(t, frames)
	})

	for name, frames := range map[string][]map[string]any{
		"a newer protocol revision": {
			{"stream": "begin", "version": 2},
		},
		"a single statement without a value": {
			begin,
			{"stream": "finished", "index": 0, "single": true},
		},
		"a value for a statement of rows": {
			begin,
			{"stream": "value", "index": 0, "value": 1},
			{"stream": "finished", "index": 0, "single": false},
		},
		"rows then a single value": {
			begin,
			{"stream": "rows", "index": 0, "values": []int{1}},
			{"stream": "value", "index": 0, "value": 1},
			{"stream": "finished", "index": 0, "single": true},
		},
		"a single value then rows": {
			begin,
			{"stream": "value", "index": 0, "value": 1},
			{"stream": "rows", "index": 0, "values": []int{1}},
			{"stream": "finished", "index": 0, "single": true},
		},
		"a statement count which disagrees": {
			begin,
			{"stream": "finished", "index": 0},
			{"stream": "end", "results": 2},
		},
		"a statement frame without an index": {
			begin,
			{"stream": "rows", "values": []int{1}},
		},
		"a response which is not a frame": {
			{"status": "OK"},
		},
	} {
		t.Run("fails on "+name, func(t *testing.T) {
			_, ended, err := readFrames[int](t, frames...)
			assert.ErrorIs(t, err, ErrStreamProtocol)
			// Only the server's end frame means the query stopped; after any
			// other failure it must still be canceled.
			assert.Equal(t, name == "a statement count which disagrees", ended)
		})
	}
}

func TestRefusesStreaming(t *testing.T) {
	type rpcError = connection.RPCError //nolint:staticcheck // query_stream errors arrive as RPC errors
	assert.True(t, refusesStreaming(&rpcError{Code: -32601, Message: "Method not found"}))
	assert.True(t, refusesStreaming(&rpcError{Code: -32602, Message: "Method query_stream not allowed"}))
	assert.False(t, refusesStreaming(&rpcError{Code: -32602, Message: "Invalid params"}))
	assert.False(t, refusesStreaming(&rpcError{Code: -32000, Message: "There was a problem"}))
}

func TestIsCBORArray(t *testing.T) {
	codec := surrealcbor.New()
	for value, want := range map[any]bool{
		"text": false,
		1:      false,
		nil:    false,
	} {
		b, err := codec.Marshal(value)
		require.NoError(t, err)
		assert.Equal(t, want, isCBORArray(b), "%v", value)
	}
	b, err := codec.Marshal([]int{1})
	require.NoError(t, err)
	assert.True(t, isCBORArray(b))
	assert.False(t, isCBORArray(nil))
	assert.False(t, errors.Is(nil, ErrStreamProtocol))
}
