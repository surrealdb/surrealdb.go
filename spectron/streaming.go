package spectron

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
)

// ChatChunk is a single Server-Sent Event frame from a streaming chat call.
type ChatChunk struct {
	// Delta is the incremental text token. Empty on non-delta frames.
	Delta string
	// TraceID is the server-assigned trace id, if echoed by the frame.
	TraceID string
	// SessionID is the chat session id, if echoed by the frame.
	SessionID string
	// Done is true on the terminal frame (either an explicit `event: done`
	// or a `data: [DONE]` sentinel).
	Done bool
	// Raw is the parsed JSON payload for frames whose data was JSON.
	Raw map[string]any
}

// frame converts a fully-buffered SSE event (its event name and joined data
// payload) into a ChatChunk.
func frame(eventName, payload string) ChatChunk {
	if payload == "[DONE]" {
		return ChatChunk{Done: true}
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		// Not JSON; treat the whole payload as a raw delta.
		return ChatChunk{Delta: payload}
	}
	chunk := ChatChunk{Raw: data}
	if v, ok := data["traceId"].(string); ok {
		chunk.TraceID = v
	} else if v, ok := data["trace_id"].(string); ok {
		chunk.TraceID = v
	}
	if v, ok := data["sessionId"].(string); ok {
		chunk.SessionID = v
	} else if v, ok := data["session_id"].(string); ok {
		chunk.SessionID = v
	}
	if eventName == "done" {
		chunk.Done = true
		return chunk
	}
	if d, ok := data["done"].(bool); ok && d {
		chunk.Done = true
		return chunk
	}
	if v, ok := data["delta"].(string); ok {
		chunk.Delta = v
	} else if v, ok := data["token"].(string); ok {
		chunk.Delta = v
	}
	return chunk
}

// iterateSSE consumes an SSE stream from r, yielding each frame as a
// ChatChunk via yield. It honours ctx cancellation: when ctx is done, the
// iterator stops without yielding a further error (the response body close
// will already surface the cancellation to the caller of the outer call,
// if needed).
//
// The SSE grammar implemented here matches Python's _streaming.py: lines
// beginning with `event:` set the current event name; lines beginning with
// `data:` (with optional single leading space) are appended to the buffer;
// a blank line completes a frame; lines beginning with `:` are comments.
func iterateSSE(ctx context.Context, r io.Reader, yield func(ChatChunk, error) bool) {
	scanner := bufio.NewScanner(r)
	// Allow up to 1 MiB per SSE line. The default 64 KiB is too tight for
	// JSON-heavy payloads.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var (
		eventName string
		dataLines []string
	)

	emit := func() bool {
		if len(dataLines) == 0 {
			eventName = ""
			return true
		}
		payload := strings.Join(dataLines, "\n")
		chunk := frame(eventName, payload)
		eventName = ""
		dataLines = dataLines[:0]
		if !yield(chunk, nil) {
			return false
		}
		return !chunk.Done
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			if !emit() {
				return
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(line[len("event:"):])
			continue
		}
		if strings.HasPrefix(line, "data:") {
			rest := line[len("data:"):]
			// Per SSE, a single leading space after the colon is stripped.
			rest = strings.TrimPrefix(rest, " ")
			dataLines = append(dataLines, rest)
			continue
		}
		// Other field names (id:, retry:, ...) are ignored.
	}
	if err := scanner.Err(); err != nil {
		yield(ChatChunk{}, &APIError{Message: "stream read: " + err.Error()})
		return
	}
	// Flush any buffered final frame that wasn't terminated by a blank line.
	if len(dataLines) > 0 {
		_ = emit()
	}
}
