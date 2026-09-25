package memory

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
)

// The chat stream's SSE vocabulary.
const (
	doneSentinel = "[DONE]"
	eventDone    = "done"
	eventError   = "error"
)

// ChatChunk is a single Server-Sent Event frame from a streaming chat call.
type ChatChunk struct {
	// Event is the SSE event label, when the frame carried one. The chat
	// stream uses "meta", "chunk", "done" and "error".
	Event string
	// Delta is the incremental text token. Empty on non-delta frames.
	Delta string
	// TraceID is the server-assigned trace id, if echoed by the frame.
	TraceID string
	// SessionID is the chat session id, if echoed by the frame.
	SessionID string
	// Reply is the complete, server-sanitised reply text. Only the terminal
	// frame carries it.
	Reply string
	// MemoryUpdates describes what the turn wrote back to memory. Only the
	// terminal frame carries it.
	MemoryUpdates *ExtractionResult
	// Citations carries one entry per inline marker in Reply. Only the
	// terminal frame carries them.
	Citations []Citation
	// Done is true on the terminal frame (an explicit `event: done`, a
	// `done: true` payload, or a `data: [DONE]` sentinel).
	Done bool
	// Raw is the parsed JSON payload for frames whose data was JSON.
	Raw map[string]any
}

// frame converts a fully-buffered SSE event (its event name and joined data
// payload) into a ChatChunk.
//
// A non-nil error means the server reported a failure inside the stream. The
// stream opens with a 200, so a mid-flight failure can only arrive as a frame;
// it has to surface where a request failure would, not as an ordinary chunk.
//
// The terminal fields (reply, memoryUpdates, citations) are read by shape
// rather than gated on the event name, because the server is free to attach
// them to whichever frame closes the stream.
func frame(eventName, payload string) (ChatChunk, error) {
	if payload == doneSentinel {
		return ChatChunk{Event: eventName, Done: true}, nil
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		// Not JSON. An error event still has to fail the stream; anything
		// else is treated as a raw delta.
		if eventName == eventError {
			return ChatChunk{}, streamError(payload, payload)
		}
		return ChatChunk{Event: eventName, Delta: payload}, nil
	}

	if eventName == eventError {
		return ChatChunk{}, streamError(stringField(data, "error", "message", "detail", "title"), data)
	}
	if msg, ok := data["error"].(string); ok && msg != "" {
		return ChatChunk{}, streamError(msg, data)
	}

	chunk := ChatChunk{Event: eventName, Raw: data}
	chunk.TraceID = stringField(data, "traceId", "trace_id")
	chunk.SessionID = stringField(data, "sessionId", "session_id")
	// "text" is the chat stream's own token key; "delta" and "token" are the
	// shapes the other streaming surfaces use.
	chunk.Delta = stringField(data, "delta", "token", "text")
	chunk.Reply = stringField(data, "reply")
	decodeField(data, "memoryUpdates", &chunk.MemoryUpdates)
	decodeField(data, "citations", &chunk.Citations)

	if eventName == eventDone {
		chunk.Done = true
	} else if d, ok := data["done"].(bool); ok && d {
		chunk.Done = true
	}
	return chunk, nil
}

// stringField returns the first of keys present in data as a non-empty string.
func stringField(data map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := data[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// decodeField re-marshals data[key] into dest. A missing, null, or
// structurally unexpected value leaves dest untouched: a malformed side-field
// must not fail a frame that is otherwise good.
func decodeField(data map[string]any, key string, dest any) {
	raw, ok := data[key]
	if !ok || raw == nil {
		return
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return
	}
	_ = json.Unmarshal(encoded, dest)
}

// iterateSSE consumes an SSE stream from r, yielding each frame as a
// ChatChunk via yield. It honors ctx cancellation: when ctx is done, the
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
		chunk, err := frame(eventName, payload)
		eventName = ""
		dataLines = dataLines[:0]
		if err != nil {
			yield(ChatChunk{}, err)
			return false
		}
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
