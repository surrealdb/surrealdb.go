package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"time"

	"github.com/fxamacker/cbor/v2"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
)

// QueryStreamVersion is the revision of the streaming query protocol this SDK
// reads. A server announcing a higher revision fails the stream rather than
// being misread.
const QueryStreamVersion = 1

const (
	methodQueryStream = "query_stream"
	methodQueryCancel = "query_cancel"

	codeMethodNotFound   = -32601
	codeMethodNotAllowed = -32602

	// abandonTimeout bounds how long an abandoned stream is drained for its
	// terminal frame, after which it is dropped.
	abandonTimeout = 30 * time.Second
)

// ErrStreamProtocol is wrapped by the error of a stream whose frames break the
// streaming protocol.
var ErrStreamProtocol = errors.New("query stream protocol error")

// StreamFrame is one step of a query's answer, received while the query runs.
//
// Rows are provisional until their statement's Done frame: when it carries Err,
// the statement failed and every row delivered for it is retracted. A
// statement's result is either its rows, in order, or a single Value, which
// only its Done frame carries.
type StreamFrame[T any] struct {
	// Statement is the index of the statement the frame belongs to.
	Statement int
	// Rows are rows of the statement, which more frames may follow.
	Rows []T
	// Done marks the frame which finishes the statement. No frame follows it
	// for the same statement.
	Done bool
	// Err is the statement's error, on its Done frame, as a *QueryError.
	Err error
	// Single is set on the Done frame of a statement whose result is one
	// value, such as SELECT ... FROM ONLY or RETURN, rather than a list.
	Single bool
	// Value is the result of a Single statement.
	Value T
	// Time is how long the statement took, on its Done frame.
	Time string
}

// QueryStream runs a query and yields its answer frame by frame while it runs,
// with the query_stream method of SurrealDB 3.3+ over WebSocket. Leaving the
// loop early cancels the query on the server. In a transaction, leaving the
// loop waits for the server to end the canceled query, so that the
// transaction's Commit or Cancel comes after it. The statements which ran
// before the cancel stay in the transaction, even one whose Done frame was
// not read: call Cancel rather than Commit to discard them.
//
// The connection's Timeout bounds the wait for the first frame only, which the
// server sends at once. After it, ctx is the only bound, as a statement sends
// nothing until it has rows or finishes.
//
// The server does not wait for the reader. Frames which arrive faster than
// the loop reads them are buffered without bound, so a slow reader can hold
// the whole result in memory, as Query does.
//
// When the connection or the server cannot stream, such as over HTTP or with an
// older server, the query runs with the query method instead and its buffered
// results are yielded as frames in the same form. The query is only re-sent
// when the server refused to stream it before running it.
//
// An error which belongs to no single statement ends the iteration: it retracts
// every statement which has no Done frame yet.
// S can be *DB, *Session, or *Transaction.
func QueryStream[T any, S sendable](ctx context.Context, s S, sql string, vars map[string]any) iter.Seq2[StreamFrame[T], error] {
	return func(yield func(StreamFrame[T], error) bool) {
		con, req, err := streamRequest(s, sql, vars)
		if err != nil {
			yield(StreamFrame[T]{}, err)
			return
		}

		stream, err := openStream(ctx, con, req)
		if errors.Is(err, constants.ErrMethodNotAvailable) {
			bufferedFrames(ctx, s, sql, vars, yield)
			return
		}
		if err != nil {
			yield(StreamFrame[T]{}, err)
			return
		}

		r := newStreamReader[T](con.GetUnmarshaler().Unmarshal)
		ended := false
		defer func() {
			if ended {
				stream.Close()
				return
			}
			abandon(con, req, stream)
		}()

		for first := true; ; first = false {
			res, err := nextResponse(ctx, stream, first)
			if err != nil {
				yield(StreamFrame[T]{}, err)
				return
			}

			if res.Error != nil {
				ended = true
				if !r.framed && refusesStreaming(res.Error) {
					bufferedFrames(ctx, s, sql, vars, yield)
					return
				}
				yield(StreamFrame[T]{}, res.Error)
				return
			}

			// Only the server's end frame ends the query; after an error found
			// here, the deferred abandon cancels it on the server.
			frames, end, err := r.read(res.Result)
			if end {
				ended = true
			}
			for _, f := range frames {
				if !yield(f, nil) {
					return
				}
			}
			if err != nil {
				yield(StreamFrame[T]{}, err)
				return
			}
			if end {
				return
			}
		}
	}
}

// nextResponse waits for the stream's next response. The wait for the first
// one is also bounded by the connection's timeout, as the server answers with
// a begin frame at once, so that a server which stopped answering is caught.
// Later waits are bounded by ctx only, as a statement sends nothing until it
// has rows or finishes.
func nextResponse(ctx context.Context, stream *connection.ResponseStream, first bool) (connection.RPCResponse[cbor.RawMessage], error) {
	if first && stream.FirstResponseTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, stream.FirstResponseTimeout)
		defer cancel()
	}
	return stream.Next(ctx)
}

// openStream sends a streaming request, or fails with
// constants.ErrMethodNotAvailable when the connection cannot stream, such as
// over HTTP or through a wrapper whose current connection cannot.
func openStream(ctx context.Context, con connection.Connection, req *connection.RPCRequest) (*connection.ResponseStream, error) {
	streamer, ok := con.(connection.Streamer)
	if !ok {
		return nil, constants.ErrMethodNotAvailable
	}
	return streamer.CallStream(ctx, req)
}

// streamRequest returns the connection and the query_stream request for s.
func streamRequest[S sendable](s S, sql string, vars map[string]any) (connection.Connection, *connection.RPCRequest, error) {
	req := &connection.RPCRequest{Method: methodQueryStream, Params: []any{sql, vars}}

	switch v := any(s).(type) {
	case *DB:
		return v.con, req, nil
	case *Session:
		if v.isClosed() {
			return nil, nil, constants.ErrSessionClosed
		}
		req.Session = v.id
		return v.db.con, req, nil
	case *Transaction:
		if v.IsClosed() {
			return nil, nil, constants.ErrTransactionClosed
		}
		req.Session = v.sessionID
		req.Txn = v.id
		return v.db.con, req, nil
	}
	panic("unreachable")
}

// refusesStreaming reports whether an error answering query_stream, before any
// frame, means the server does not stream: it does not know the method, or it
// does not allow it. The query has not run then, so it can be sent again.
//
// SurrealDB sends -32602 only for a method it does not allow, before running
// it, with the message "Method not allowed"; invalid params are -32603.
func refusesStreaming(err *connection.RPCError) bool { //nolint:staticcheck // the RPC response carries an RPCError
	return err.Code == codeMethodNotFound || err.Code == codeMethodNotAllowed
}

// abandon cancels a stream whose reader left before its end frame, and drains
// it until the end frame arrives, so that frames still in flight are dropped
// rather than misrouted. A transaction's stream is drained before abandon
// returns, so that a Commit or Cancel which follows cannot overtake the
// canceled query; any other stream is drained in the background.
func abandon(con connection.Connection, req *connection.RPCRequest, stream *connection.ResponseStream) {
	drain := func() {
		ctx, cancel := context.WithTimeout(context.Background(), abandonTimeout)
		defer cancel()
		defer stream.Close()

		cancelReq := &connection.RPCRequest{Method: methodQueryCancel, Params: []any{stream.ID}, Session: req.Session}
		go func() { _, _ = con.Call(ctx, cancelReq) }()

		for {
			res, err := stream.Next(ctx)
			if err != nil || res.Error != nil {
				return
			}
			var f struct {
				Stream string `json:"stream"`
			}
			if res.Result == nil || con.GetUnmarshaler().Unmarshal(*res.Result, &f) != nil || f.Stream == "end" {
				return
			}
		}
	}

	if req.Txn != nil {
		drain()
		return
	}
	go drain()
}

// wireFrame is a frame of the streaming query protocol.
type wireFrame struct {
	Stream  string               `json:"stream"`
	Version *int                 `json:"version,omitempty"`
	Index   *int                 `json:"index,omitempty"`
	Values  cbor.RawMessage      `json:"values,omitempty"`
	Value   cbor.RawMessage      `json:"value,omitempty"`
	Time    string               `json:"time,omitempty"`
	Single  bool                 `json:"single,omitempty"`
	Results *int                 `json:"results,omitempty"`
	Error   *connection.RPCError `json:"error,omitempty"` //nolint:staticcheck // the frame's error is an RPC error object
}

// streamReader turns the frames of one stream into StreamFrames, holding the
// protocol to its contract.
type streamReader[T any] struct {
	unmarshal func([]byte, any) error
	framed    bool
	// singles holds the value of each statement which sent one, until its
	// finished frame says it is single.
	singles map[int]cbor.RawMessage
	// rowsSent holds the statements which sent rows, which can then send no
	// single value.
	rowsSent map[int]bool
	finished map[int]bool
}

func newStreamReader[T any](unmarshal func([]byte, any) error) *streamReader[T] {
	return &streamReader[T]{
		unmarshal: unmarshal,
		singles:   map[int]cbor.RawMessage{},
		rowsSent:  map[int]bool{},
		finished:  map[int]bool{},
	}
}

// read reads one frame. It returns the frames to yield, whether it was the
// server's end frame, and an error which ends the stream: the server's, or a
// break of the protocol found here, after which the query is still running.
func (r *streamReader[T]) read(raw *cbor.RawMessage) (frames []StreamFrame[T], end bool, err error) {
	if raw == nil {
		return nil, false, fmt.Errorf("%w: an empty response", ErrStreamProtocol)
	}
	var f wireFrame
	if err := r.unmarshal(*raw, &f); err != nil {
		return nil, false, fmt.Errorf("%w: %w", ErrStreamProtocol, err)
	}
	if f.Stream == "" {
		return nil, false, fmt.Errorf("%w: a response which is not a frame", ErrStreamProtocol)
	}
	r.framed = true

	switch f.Stream {
	case "begin":
		if f.Version != nil && *f.Version > QueryStreamVersion {
			return nil, false, fmt.Errorf("%w: the server speaks revision %d, this SDK reads revision %d",
				ErrStreamProtocol, *f.Version, QueryStreamVersion)
		}
		return nil, false, nil

	case "rows", "value", "finished":
		frames, err := r.readStatement(&f)
		return frames, false, err

	case "end":
		if f.Error != nil {
			return nil, true, f.Error
		}
		if f.Results != nil && *f.Results != len(r.finished) {
			return nil, true, fmt.Errorf("%w: the stream ended reporting %d statements, but %d were delivered",
				ErrStreamProtocol, *f.Results, len(r.finished))
		}
		return nil, true, nil
	}

	// A frame of a newer revision of the protocol, which the end frame still bounds.
	return nil, false, nil
}

// readStatement reads a frame which belongs to one statement.
func (r *streamReader[T]) readStatement(f *wireFrame) ([]StreamFrame[T], error) {
	if f.Index == nil || *f.Index < 0 {
		return nil, fmt.Errorf("%w: a %s frame without a statement index", ErrStreamProtocol, f.Stream)
	}
	if r.finished[*f.Index] {
		// Nothing follows a statement's finished frame.
		return nil, nil
	}

	switch f.Stream {
	case "rows":
		if _, ok := r.singles[*f.Index]; ok {
			return nil, fmt.Errorf("%w: statement %d sent rows after a single value", ErrStreamProtocol, *f.Index)
		}
		r.rowsSent[*f.Index] = true
		var rows []T
		if len(f.Values) > 0 {
			if err := r.unmarshal(f.Values, &rows); err != nil {
				return nil, fmt.Errorf("failed to unmarshal rows: %w", err)
			}
		}
		return []StreamFrame[T]{{Statement: *f.Index, Rows: rows}}, nil
	case "value":
		if r.rowsSent[*f.Index] {
			return nil, fmt.Errorf("%w: statement %d sent a single value after rows", ErrStreamProtocol, *f.Index)
		}
		r.singles[*f.Index] = f.Value
		return nil, nil
	default:
		return r.finish(f)
	}
}

func (r *streamReader[T]) finish(f *wireFrame) ([]StreamFrame[T], error) {
	index := *f.Index
	value, sentValue := r.singles[index]
	delete(r.singles, index)
	delete(r.rowsSent, index)
	r.finished[index] = true

	done := StreamFrame[T]{Statement: index, Done: true, Time: f.Time}

	if f.Error != nil {
		done.Err = &QueryError{Message: f.Error.Message}
		return []StreamFrame[T]{done}, nil
	}
	if f.Single && !sentValue {
		return nil, fmt.Errorf("%w: statement %d finished as a single value, but no value was sent",
			ErrStreamProtocol, index)
	}
	if !f.Single && sentValue {
		return nil, fmt.Errorf("%w: statement %d sent a single value, but finished as a list of rows",
			ErrStreamProtocol, index)
	}

	if f.Single {
		done.Single = true
		if len(value) > 0 {
			if err := r.unmarshal(value, &done.Value); err != nil {
				return nil, fmt.Errorf("failed to unmarshal value: %w", err)
			}
		}
	}
	return []StreamFrame[T]{done}, nil
}

// bufferedFrames runs the query with the query method and yields its results
// as frames.
func bufferedFrames[T any, S sendable](
	ctx context.Context, s S, sql string, vars map[string]any, yield func(StreamFrame[T], error) bool,
) {
	results, err := Query[cbor.RawMessage](ctx, s, sql, vars)
	if results == nil {
		yield(StreamFrame[T]{}, err)
		return
	}

	unmarshal := unmarshalerOf(s)
	for i, res := range *results {
		done := StreamFrame[T]{Statement: i, Done: true, Time: res.Time}

		switch {
		case res.Error != nil:
			done.Err = res.Error
		case isCBORArray(res.Result):
			// A statement whose result is an array streams it as rows, even
			// RETURN [1, 2] (checked against SurrealDB 3.3.4), so the
			// buffered answer gives an array as rows too.
			var rows []T
			if err := unmarshal(res.Result, &rows); err != nil {
				yield(StreamFrame[T]{}, fmt.Errorf("failed to unmarshal rows: %w", err))
				return
			}
			if len(rows) > 0 && !yield(StreamFrame[T]{Statement: i, Rows: rows}, nil) {
				return
			}
		default:
			done.Single = true
			if len(res.Result) > 0 {
				if err := unmarshal(res.Result, &done.Value); err != nil {
					yield(StreamFrame[T]{}, fmt.Errorf("failed to unmarshal value: %w", err))
					return
				}
			}
		}

		if !yield(done, nil) {
			return
		}
	}
}

func unmarshalerOf[S sendable](s S) func([]byte, any) error {
	switch v := any(s).(type) {
	case *DB:
		return v.con.GetUnmarshaler().Unmarshal
	case *Session:
		return v.db.con.GetUnmarshaler().Unmarshal
	case *Transaction:
		return v.db.con.GetUnmarshaler().Unmarshal
	}
	panic("unreachable")
}

// isCBORArray reports whether raw encodes an array.
func isCBORArray(raw cbor.RawMessage) bool {
	const majorTypeArray = 4
	return len(raw) > 0 && raw[0]>>5 == majorTypeArray
}
