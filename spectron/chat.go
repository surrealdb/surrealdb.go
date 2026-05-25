package spectron

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
)

// Chat sends a message and returns the model's reply. For incremental
// streaming output, use [Client.ChatStream] instead.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	// Wire format: include stream:true only when streaming. The non-stream
	// path explicitly sends false to be unambiguous.
	payload := chatWirePayload(req, false)
	var out ChatResponse
	if err := c.doJSON(ctx, http.MethodPost, c.base+"/chat", payload, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

// ChatStream sends a chat message and yields each Server-Sent Event chunk
// as it arrives. The iterator stops on a terminal "[DONE]" sentinel, on
// end-of-stream, on ctx cancellation, or when an error is reported. A
// non-nil error is yielded at most once and always with a zero-value
// ChatChunk.
//
// Usage:
//
//	for chunk, err := range client.ChatStream(ctx, req) {
//	    if err != nil { return err }
//	    fmt.Print(chunk.Delta)
//	    if chunk.Done { break }
//	}
func (c *Client) ChatStream(ctx context.Context, req ChatRequest) iter.Seq2[ChatChunk, error] {
	return func(yield func(ChatChunk, error) bool) {
		payload := chatWirePayload(req, true)
		body, err := json.Marshal(payload)
		if err != nil {
			yield(ChatChunk{}, &APIError{Message: fmt.Sprintf("marshal chat payload: %v", err)})
			return
		}
		resp, err := c.do(ctx, http.MethodPost, c.base+"/chat", body, "application/json", nil, false, true)
		if err != nil {
			yield(ChatChunk{}, err)
			return
		}
		defer resp.Body.Close()
		iterateSSE(ctx, resp.Body, yield)
	}
}

// chatWirePayload mirrors Python's _drop_none on the chat body and forces
// stream into the request when streaming.
func chatWirePayload(req ChatRequest, stream bool) map[string]any {
	out := map[string]any{"message": req.Message}
	if stream {
		out["stream"] = true
	}
	if req.SessionID != "" {
		out["sessionId"] = req.SessionID
	}
	if len(req.Scope) > 0 {
		out["scope"] = req.Scope
	}
	if req.Model != "" {
		out["model"] = req.Model
	}
	if req.BypassCache {
		out["bypassCache"] = true
	}
	return out
}
