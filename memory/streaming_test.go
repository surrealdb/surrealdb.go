package memory

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func collect(t *testing.T, body string) []ChatChunk {
	t.Helper()
	var chunks []ChatChunk
	iterateSSE(context.Background(), strings.NewReader(body), func(c ChatChunk, err error) bool {
		if err != nil {
			t.Fatalf("unexpected SSE error: %v", err)
		}
		chunks = append(chunks, c)
		return true
	})
	return chunks
}

func TestSSEBasicDelta(t *testing.T) {
	body := "data: {\"delta\":\"hello\"}\n\n" +
		"data: {\"delta\":\" world\"}\n\n" +
		"data: [DONE]\n\n"
	chunks := collect(t, body)
	if len(chunks) != 3 {
		t.Fatalf("len = %d, want 3", len(chunks))
	}
	if chunks[0].Delta != "hello" || chunks[1].Delta != " world" {
		t.Errorf("deltas = %q, %q", chunks[0].Delta, chunks[1].Delta)
	}
	if !chunks[2].Done {
		t.Error("final [DONE] chunk should be Done")
	}
}

func TestSSEEventDone(t *testing.T) {
	body := "event: done\ndata: {\"sessionId\":\"s1\",\"traceId\":\"t1\"}\n\n"
	chunks := collect(t, body)
	if len(chunks) != 1 {
		t.Fatalf("len = %d, want 1", len(chunks))
	}
	c := chunks[0]
	if !c.Done {
		t.Error("expected Done=true on event:done frame")
	}
	if c.SessionID != "s1" || c.TraceID != "t1" {
		t.Errorf("got SessionID=%q TraceID=%q", c.SessionID, c.TraceID)
	}
}

func TestSSEMultiLineData(t *testing.T) {
	body := "data: line1\ndata: line2\n\n"
	chunks := collect(t, body)
	if len(chunks) != 1 {
		t.Fatalf("len = %d", len(chunks))
	}
	// Not JSON, so payload becomes the delta verbatim.
	if chunks[0].Delta != "line1\nline2" {
		t.Errorf("delta = %q", chunks[0].Delta)
	}
}

func TestSSEIgnoresCommentsAndUnknownFields(t *testing.T) {
	body := ": keepalive\nid: 42\nretry: 100\ndata: {\"delta\":\"ok\"}\n\n"
	chunks := collect(t, body)
	if len(chunks) != 1 || chunks[0].Delta != "ok" {
		t.Fatalf("got %#v", chunks)
	}
}

func TestSSEUnterminatedFinalFrame(t *testing.T) {
	// No trailing blank line.
	body := "data: {\"delta\":\"final\"}"
	chunks := collect(t, body)
	if len(chunks) != 1 || chunks[0].Delta != "final" {
		t.Fatalf("got %#v", chunks)
	}
}

func TestSSEStopsOnYieldFalse(t *testing.T) {
	body := "data: {\"delta\":\"a\"}\n\ndata: {\"delta\":\"b\"}\n\n"
	var seen []string
	iterateSSE(context.Background(), strings.NewReader(body), func(c ChatChunk, err error) bool {
		if err != nil {
			t.Fatal(err)
		}
		seen = append(seen, c.Delta)
		return false // stop after first
	})
	if len(seen) != 1 || seen[0] != "a" {
		t.Errorf("seen = %v", seen)
	}
}

func TestSSESnakeCaseTraceID(t *testing.T) {
	body := "data: {\"session_id\":\"sx\",\"trace_id\":\"tx\",\"delta\":\"d\"}\n\n"
	chunks := collect(t, body)
	if len(chunks) != 1 {
		t.Fatalf("len = %d", len(chunks))
	}
	if chunks[0].SessionID != "sx" || chunks[0].TraceID != "tx" {
		t.Errorf("got %+v", chunks[0])
	}
}

// collectErr drains a stream that is expected to fail, returning the chunks
// yielded before the failure and the error that ended it.
func collectErr(t *testing.T, body string) ([]ChatChunk, error) {
	t.Helper()
	var (
		chunks []ChatChunk
		failed error
	)
	iterateSSE(context.Background(), strings.NewReader(body), func(c ChatChunk, err error) bool {
		if err != nil {
			failed = err
			return false
		}
		chunks = append(chunks, c)
		return true
	})
	return chunks, failed
}

func TestSSEReadsTextTokenKey(t *testing.T) {
	// "text" is the key the chat stream actually emits. Reading only
	// delta/token yielded silently empty chunks.
	body := "event: chunk\ndata: {\"text\":\"hi\"}\n\n"
	chunks := collect(t, body)
	if len(chunks) != 1 {
		t.Fatalf("len = %d, want 1", len(chunks))
	}
	if chunks[0].Delta != "hi" {
		t.Errorf("Delta = %q, want %q", chunks[0].Delta, "hi")
	}
	if chunks[0].Event != "chunk" {
		t.Errorf("Event = %q, want %q", chunks[0].Event, "chunk")
	}
}

func TestSSEDeltaWinsOverText(t *testing.T) {
	body := "data: {\"delta\":\"a\",\"token\":\"b\",\"text\":\"c\"}\n\n"
	chunks := collect(t, body)
	if chunks[0].Delta != "a" {
		t.Errorf("Delta = %q, want %q", chunks[0].Delta, "a")
	}
}

func TestSSETerminalFrameCarriesReplyAndCitations(t *testing.T) {
	body := "event: done\n" +
		`data: {"done":true,"reply":"Acme is a company [S1].",` +
		`"citations":[{"id":"c1","kind":"passage","marker":"[S1]","snippet":"Acme","score":0.9,` +
		`"documentTitle":"Handbook","positionPercent":12}],` +
		`"memoryUpdates":{"turnId":"t1","entities":[{"id":"e1","name":"Acme",` +
		`"entityType":"org","memoryCategory":"knowledge","isNew":true}]}}` + "\n\n"
	chunks := collect(t, body)
	if len(chunks) != 1 {
		t.Fatalf("len = %d, want 1", len(chunks))
	}
	c := chunks[0]
	if !c.Done {
		t.Error("expected Done=true")
	}
	if c.Reply != "Acme is a company [S1]." {
		t.Errorf("Reply = %q", c.Reply)
	}
	if len(c.Citations) != 1 {
		t.Fatalf("Citations len = %d, want 1", len(c.Citations))
	}
	cit := c.Citations[0]
	if cit.Marker != "[S1]" || cit.DocumentTitle != "Handbook" {
		t.Errorf("citation = %+v", cit)
	}
	// A null position and position 0 are different places in a document.
	if cit.PositionPercent == nil || *cit.PositionPercent != 12 {
		t.Errorf("citation = %+v", cit)
	}
	if c.MemoryUpdates == nil {
		t.Fatal("MemoryUpdates = nil")
	}
	if c.MemoryUpdates.TurnID != "t1" || len(c.MemoryUpdates.Entities) != 1 {
		t.Errorf("MemoryUpdates = %+v", c.MemoryUpdates)
	}
}

func TestSSETerminalFieldsReadByShapeNotEventName(t *testing.T) {
	// The server may attach the terminal payload to whichever frame closes
	// the stream, so the fields must not be gated on `event: done`.
	body := `data: {"done":true,"reply":"final"}` + "\n\n"
	chunks := collect(t, body)
	if len(chunks) != 1 || chunks[0].Reply != "final" {
		t.Fatalf("chunks = %+v", chunks)
	}
}

func TestSSEErrorEventFailsTheStream(t *testing.T) {
	body := "event: chunk\ndata: {\"text\":\"partial\"}\n\n" +
		"event: error\ndata: {\"error\":\"model unavailable\"}\n\n"
	chunks, err := collectErr(t, body)
	if err == nil {
		t.Fatal("expected an error from the error frame")
	}
	if !errors.Is(err, ErrStream) {
		t.Errorf("errors.Is(err, ErrStream) = false; err = %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("errors.As(*APIError) = false; err = %v", err)
	}
	if apiErr.StatusCode != 0 {
		t.Errorf("StatusCode = %d, want 0 (the response itself succeeded)", apiErr.StatusCode)
	}
	if apiErr.Message != "model unavailable" {
		t.Errorf("Message = %q", apiErr.Message)
	}
	if len(chunks) != 1 || chunks[0].Delta != "partial" {
		t.Errorf("chunks before the failure = %+v", chunks)
	}
}

func TestSSEErrorKeyWithoutEventNameFailsTheStream(t *testing.T) {
	body := `data: {"error":"quota exceeded"}` + "\n\n"
	_, err := collectErr(t, body)
	if err == nil || !errors.Is(err, ErrStream) {
		t.Fatalf("err = %v, want ErrStream", err)
	}
}

func TestSSEMalformedSideFieldDoesNotFailTheFrame(t *testing.T) {
	// citations arriving as the wrong shape must not lose the reply.
	body := `data: {"done":true,"reply":"kept","citations":"oops"}` + "\n\n"
	chunks := collect(t, body)
	if len(chunks) != 1 {
		t.Fatalf("len = %d, want 1", len(chunks))
	}
	if chunks[0].Reply != "kept" {
		t.Errorf("Reply = %q, want %q", chunks[0].Reply, "kept")
	}
	if chunks[0].Citations != nil {
		t.Errorf("Citations = %+v, want nil", chunks[0].Citations)
	}
}
