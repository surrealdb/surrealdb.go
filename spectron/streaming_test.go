package spectron

import (
	"context"
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
