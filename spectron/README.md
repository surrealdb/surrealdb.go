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

The types and wire formats in this package track the Spectron OpenAPI spec
(`doc/spec/enduser.swagger.json` in `surrealdb/spectron`). A copy is vendored at
[`openapi/enduser.swagger.json`](openapi/enduser.swagger.json) so a future sync
is a diff against a known baseline.

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

One `Client` serves both blocking and concurrent use; every method takes a
`context.Context` for cancellation, and `*Client` is safe for concurrent use.

## Surface

### Memory verbs

| Method | Endpoint |
| --- | --- |
| `Remember(ctx, req)` | `POST /api/v1/{context}/facts` |
| `RememberMany(ctx, req)` | `POST /api/v1/{context}/facts/batch` |
| `Recall(ctx, req)` | `POST /api/v1/{context}/query` |
| `Forget(ctx, query, opts...)` | `POST /api/v1/{context}/forget` |
| `Chat(ctx, req)` / `ChatStream(ctx, req)` | `POST /api/v1/{context}/chat` |
| `QueryContext(ctx, req)` | `POST /api/v1/{context}/context` |
| `Consolidate(ctx, req)` | `POST /api/v1/{context}/consolidate` |
| `Reflect(ctx, req)` | `POST /api/v1/{context}/reflect` |
| `Elaborate(ctx, req)` | `POST /api/v1/{context}/elaborate` |
| `Inspect(ctx, opts)` | `GET /api/v1/{context}/inspect` |
| `Fsck(ctx, req)` | `POST /api/v1/{context}/fsck` |
| `State(ctx)` / `Profile(ctx)` | `GET /api/v1/{context}/state` and `/profile` |
| `Whoami(ctx)` | `GET /api/v1/{context}/me` |
| `Audit(ctx, opts)` | `GET /api/v1/{context}/audit` |
| `DecayImportance(ctx)` / `ExpireContext(ctx)` | `POST /api/v1/{context}/lifecycle/...` |
| `Health(ctx)` | `GET /api/v1/health` |

### Sub-clients

| Accessor | Methods | Endpoints |
| --- | --- | --- |
| `Documents()` | `Upload`, `Reprocess`, `List`, `Get`, `Delete`, `Chunks`, `FetchRaw`, `Query`, `RecomputeLinks`, `ListKeywords`, `SearchKeywords`, `Keyword`, `KeywordsFor` | `/api/v1/{context}/documents...` |
| `Entities()` | `List`, `Get`, `Delete`, `History` | `/api/v1/{context}/entities...` |
| `Scopes()` | `List`, `Register`, `Delete`, `Forget`, `Grants` | `/api/v1/{context}/scopes...` |
| `Sessions()` | `Create`, `Delete`, `Context`, `Turns` | `/api/v1/{context}/sessions...` |
| `Principals()` | `List`, `Get`, `Effective`, `Grant`, `Revoke` | `/api/v1/{context}/principals...` |
| `Keys()` | `Create`, `List`, `Delete`, `Rotate` | `/api/v1/{context}/keys...` |
| `Traces()` | `List`, `Get`, `Stats` | `/api/v1/{context}/traces...` |

### Remember

```go
client.Remember(ctx, spectron.RememberRequest{Text: "I work at Acme as CTO"})

client.Remember(ctx, spectron.RememberRequest{
    Text:      "Acme acquired Beta",
    SessionID: "sess:abc",
    Scope:     spectron.Scope{"team/acme"},
    Infer:     spectron.InferFull,
})

client.RememberMany(ctx, spectron.RememberManyRequest{
    Messages: []spectron.BatchMessage{
        {Role: spectron.RoleUser, Content: "I just got promoted to CTO"},
        {Role: spectron.RoleAssistant, Content: "Congratulations!"},
    },
    Extract: spectron.ExtractWholeConversation,
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
    Mode:  spectron.MemoryModeHybrid,
    Lens:  []string{"team/acme"},
})
for _, hit := range res.Hits {
    fmt.Println(hit.Score, hit.Source, hit.Text)
}
fmt.Println(res.Tier, res.ClassificationKind)
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

page, _ := client.Documents().List(ctx, spectron.ListDocumentsOptions{Status: spectron.DocReady})
for _, d := range page.Documents {
    fmt.Println(d.ID, d.Title, d.Status)
}
```

`Upload` accepts any `io.Reader`: `*os.File`, `bytes.Reader`, or anything else.

## Typed enums

String-valued spec enums are defined types with constants, so the compiler
checks the names while the wire form stays a plain string: `InferMode`,
`MemoryCategory`, `TurnRole`, `BatchExtractionMode`, `DocumentStatus`,
`QueryKind`, `Tier`, `ResultKind`, `QueryMode`, `MemoryQueryMode`, `GraphEdgeKind`,
`TraceKind`, `DecisionKind`, and `InjectionKind`. Unknown values still decode
without error.

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
- Non-idempotent writes never retry; the error is surfaced to the caller.
- Default timeout is 30s, applied per request. Override with
  `WithTimeout(d)` on `New`. Use `context.WithTimeout` for a tighter
  per-call deadline.

## Scope

Scope is a `spectron.Scope` (a `[]string`): an ordered, de-duplicated set of
hierarchical scope paths in canonical slash form, e.g. `"team/eng"` or
`"org/apple/product/ipad"`. A key/value pair is written as the two segments
`"key/value"`. Empty represents the caller's default write region.

It is sent on the wire as a plain JSON array. Marshaling drops empty entries and
de-duplicates while preserving first-seen order, so equivalent inputs produce the
same body and the `Idempotency-Key` stays stable across retries.

```go
client.Remember(ctx, spectron.RememberRequest{
    Text:  "...",
    Scope: spectron.Scope{"team/acme"},
})

client.Documents().Upload(ctx, body,
    spectron.WithScope(spectron.Scope{"team/acme", "user/tobie"}),
)
```

Note that labels (the `Labels` fields) keep the `key=value` form, e.g.
`"tier=gold"`; only scope paths use slash form.

## Delegation

`client.OnBehalfOf(principalID)` returns a derived client that attributes every
request to another principal via the `X-Spectron-On-Behalf-Of` header. The
server still enforces the calling token's own grants on top, so delegation can
only narrow access, never widen it. The derived client shares the underlying
HTTP transport, and sub-clients delegate too.

```go
agent := client.OnBehalfOf("user:bob")
hits, _ := agent.Recall(ctx, spectron.RecallRequest{Query: "what's my role?"})
_, _ = agent.Documents().List(ctx, spectron.ListDocumentsOptions{})

// Confirm the resolved identity, including the delegation in effect.
me, _ := agent.Whoami(ctx)
fmt.Println(me.PrincipalID, me.DelegatedPrincipalID, me.EffectiveGrants)
```

## Keys

`client.Keys()` manages scoped self-service bearer keys. The full secret is
returned only once, on `Create` and `Rotate`.

```go
minted, _ := client.Keys().Create(ctx, spectron.CreateKeyRequest{
    Name:       "ci-indexer",
    Grants:     map[string][]string{"memory:read": {"team/acme"}},
    TTLSeconds: 3600,
})
fmt.Println(minted.Key) // sp-{id}-{secret}; store it now

keys, _ := client.Keys().List(ctx)
rotated, _ := client.Keys().Rotate(ctx, "ci-indexer", 3600)
_ = client.Keys().Delete(ctx, "ci-indexer")
_ = keys
_ = rotated
```

## Authentication

Every request carries `Authorization: Bearer <apiKey>` and no other
auth-related header. There is no env-var fallback, no URL default, and no
alternative scheme; pass the key explicitly to `New`.
