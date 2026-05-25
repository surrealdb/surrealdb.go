// Package spectron is a Go client for the Spectron memory service
// (https://github.com/surrealdb/spectron).
//
// A Spectron Client is pinned to a single context and talks to
// /api/v1/{context}/... over HTTPS. Every request carries a bearer token;
// writes that the SDK considers idempotent additionally carry an
// Idempotency-Key derived from sha256(METHOD | path | body | 30s-bucket),
// so a retry inside the bucket collapses onto the previous attempt
// server-side.
//
// Example:
//
//	client, err := spectron.New(
//	    "acme-prod",
//	    "https://api.spectron.example",
//	    "sk-spec-...",
//	)
//	if err != nil {
//	    return err
//	}
//	defer client.Close()
//
//	_, err = client.Remember(ctx, spectron.RememberRequest{Text: "I work at Acme as CTO"})
//	if err != nil {
//	    return err
//	}
//
//	hits, err := client.Recall(ctx, spectron.RecallRequest{Query: "what do I do at Acme"})
//
// All methods take a context.Context as the first argument; cancellation is
// honoured by both regular requests and SSE streams.
//
// This package is bundled with the surrealdb.go module but does not share
// transport or codec plumbing with the core SurrealDB client.
package spectron
