package memory_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/surrealdb/surrealdb.go/memory"
)

// ExampleClient_Remember demonstrates the canonical remember-then-recall
// flow against an in-process fake.
func ExampleClient_Remember() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/acme/facts":
			_, _ = io.WriteString(w, `{"mode":"fact","sessionId":"s","turnId":"t"}`)
		case "/api/v1/acme/query":
			_, _ = io.WriteString(w, `{"hits":[{"id":"1","score":0.9,"source":"f","text":"CTO at Acme"}]}`)
		}
	}))
	defer srv.Close()

	client, err := memory.New("acme", srv.URL, "sk-demo")
	if err != nil {
		panic(err)
	}
	defer client.Close()

	ctx := context.Background()
	if _, err = client.Remember(ctx, &memory.RememberRequest{Text: "I work at Acme as CTO"}); err != nil {
		panic(err)
	}
	hits, err := client.Recall(ctx, &memory.RecallRequest{Query: "role at Acme"})
	if err != nil {
		panic(err)
	}
	fmt.Println(hits.Hits[0].Text)
	// Output: CTO at Acme
}

// ExampleClient_Recall_errorHandling shows how to discriminate Agent Memory errors.
func ExampleClient_Recall_errorHandling() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Trace-Id", "abc")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"message":"missing"}`)
	}))
	defer srv.Close()

	client, _ := memory.New("acme", srv.URL, "sk-demo")
	defer client.Close()

	_, err := client.Recall(context.Background(), &memory.RecallRequest{Query: "x"})
	switch {
	case errors.Is(err, memory.ErrNotFound):
		var api *memory.APIError
		if errors.As(err, &api) {
			fmt.Printf("not found (trace=%s)\n", api.TraceID)
		}
	default:
		fmt.Println("other:", err)
	}
	// Output: not found (trace=abc)
}
