# Spectron

Go client for [Spectron](https://surrealdb.com/platform/spectron), bundled with
`surrealdb.go`.

```go
import "github.com/surrealdb/surrealdb.go/spectron"

client, err := spectron.New("acme-prod", "https://api.spectron.example", "sk-spec-...")
if err != nil {
    return err
}
defer client.Close()

ctx := context.Background()
if _, err := client.Remember(ctx, spectron.RememberRequest{Text: "I work at Acme as CTO"}); err != nil {
    return err
}
hits, err := client.Recall(ctx, spectron.RecallRequest{Query: "what do I do at Acme"})
if err != nil {
    return err
}
for _, h := range hits.Hits {
    fmt.Println(h.Score, h.Source, h.Text)
}
```

## Install

```
go get github.com/surrealdb/surrealdb.go/spectron
```

## Client

A `*spectron.Client` is pinned to a single context and hits `/api/v1/{context}/...`.

```go
client, err := spectron.New(
    "acme-prod",                       // context id
    "https://api.spectron.example",    // endpoint
    "sk-...",                          // api key
    spectron.WithTimeout(30*time.Second),
    spectron.WithMaxRetries(3),
)
```

| Argument | Required | Notes |
| --- | --- | --- |
| `contextID` | yes | Context id, e.g. `"acme-prod"`. |
| `endpoint`  | yes | Full URL of the Spectron host. Trailing slashes are trimmed. |
| `apiKey`    | yes | Bearer token, sent as `Authorization: Bearer <key>`. The SDK never reads environment variables. |

The Go client uses one `Client` for both blocking and concurrent use — every
method takes a `context.Context` for cancellation. `*Client` is safe for
concurrent use.

## Surface

| Method | Endpoint |
| --- | --- |
| `Remember(ctx, req)` | `POST /api/v1/{context}/facts` |
| `RememberMany(ctx, req)` | `POST /api/v1/{context}/facts/batch` |
| `Recall(ctx, req)` | `POST /api/v1/{context}/query` |
| `Forget(ctx, query, opts...)` | `POST /api/v1/{context}/forget` |
| `Chat(ctx, req)` | `POST /api/v1/{context}/chat` |
| `ChatStream(ctx, req)` | `POST /api/v1/{context}/chat` (SSE) |
| `Documents().Upload(ctx, body, opts...)` | `POST /api/v1/{context}/documents` (multipart) |

### Remember

```go
client.Remember(ctx, spectron.RememberRequest{Text: "I work at Acme as CTO"})

client.Remember(ctx, spectron.RememberRequest{
    Text:      "Acme acquired Beta",
    SessionID: "sess:abc",
    Scope:     spectron.Scope{"org": "acme"},
})

client.RememberMany(ctx, spectron.RememberManyRequest{
    Messages: []map[string]any{
        {"role": "user", "content": "I just got promoted to CTO"},
        {"role": "assistant", "content": "Congratulations!"},
    },
})
```

`Remember` and `RememberMany` send an `Idempotency-Key` header derived from
`sha256(METHOD | path | body | 30s-bucket)`, so a retry inside the bucket
collapses onto the previous attempt server-side.

### Recall

```go
res, err := client.Recall(ctx, spectron.RecallRequest{
    Query: "what role do I have at Acme",
    K:     10,
    Mode:  "hybrid",
})
for _, hit := range res.Hits {
    fmt.Println(hit.Score, hit.Source, hit.Text)
}
```

### Forget

```go
client.Forget(ctx, "anything about my old job")
client.Forget(ctx, "draft notes", spectron.WithPurge())
```

### Chat

```go
reply, _ := client.Chat(ctx, spectron.ChatRequest{Message: "what's my role?"})
fmt.Println(reply.Reply)

// Streaming via Go 1.23 range-over-func.
for chunk, err := range client.ChatStream(ctx, spectron.ChatRequest{Message: "what's my role?"}) {
    if err != nil {
        return err
    }
    fmt.Print(chunk.Delta)
    if chunk.Done {
        fmt.Println("\n[trace]", chunk.TraceID)
        break
    }
}
```

### Documents

```go
f, _ := os.Open("returns.pdf")
defer f.Close()

res, err := client.Documents().Upload(ctx, f,
    spectron.WithFilename("returns.pdf"),
    spectron.WithContentType("application/pdf"),
)
fmt.Println(res.ID, res.Status)
```

`Upload` accepts any `io.Reader` — `*os.File`, `bytes.Reader`, or anything
else.

## Errors

```go
_, err := client.Recall(ctx, spectron.RecallRequest{Query: "..."})
switch {
case errors.Is(err, spectron.ErrNotFound):
    var api *spectron.APIError
    errors.As(err, &api)
    fmt.Println(api.StatusCode, api.Message, api.TraceID)
case errors.Is(err, spectron.ErrAuth):
    // re-auth
}
```

| Sentinel | HTTP |
| --- | --- |
| `ErrAuth` | 401 |
| `ErrScope` | 403 |
| `ErrNotFound` | 404 |

Every non-2xx response also yields an `*APIError` carrying `StatusCode`,
`Message`, `TraceID`, and the decoded `Body`. Pull it out with `errors.As`.

## Retries and timeouts

- `GET` and idempotent writes (`Remember`, `RememberMany`) retry on
  connection errors and 5xx, with a 250ms / 500ms / 1s backoff. Capped at
  `WithMaxRetries(n)` (default 3).
- Non-idempotent writes never retry. You handle it.
- Default timeout is 30s, applied per request. Override with
  `WithTimeout(d)` on `New`. Use `context.WithTimeout` for a tighter
  per-call deadline.

## Scope

Scope is a plain `spectron.Scope` (`map[string]string`). The SDK serialises
it on the wire as the list-of-pairs shape the server expects, sorted by key
for stable hashing.

```go
client.Remember(ctx, spectron.RememberRequest{
    Text:  "...",
    Scope: spectron.Scope{"org": "acme"},
})

client.Documents().Upload(ctx, body,
    spectron.WithScope(spectron.Scope{"org": "acme", "user": "tobie"}),
)
```

## Authentication

Every request carries `Authorization: Bearer <apiKey>` and no other
auth-related header. There is no env-var fallback, no URL default, and no
alternative scheme — pass the key explicitly to `New`.
