# Agent Memory

Go client for [Agent Memory](https://surrealdb.com/agent-memory), bundled with
`surrealdb.go`.

```go
import "github.com/surrealdb/surrealdb.go/memory"

client, err := memory.New("acme-prod", "https://api.memory.example", "sk-spec-...")
if err != nil {
    return err
}
defer client.Close()

ctx := context.Background()
if _, err := client.Remember(ctx, &memory.RememberRequest{Text: "I work at Acme as CTO"}); err != nil {
    return err
}
hits, err := client.Recall(ctx, &memory.RecallRequest{Query: "what do I do at Acme"})
if err != nil {
    return err
}
for _, h := range hits.Hits {
    fmt.Println(h.Score, h.Source, h.Text)
}
```

The types and wire formats in this package track the Agent Memory OpenAPI spec
(`doc/spec/enduser.swagger.json` in `surrealdb/spectron`). A copy is vendored at
[`openapi/enduser.swagger.json`](openapi/enduser.swagger.json) so a future sync
is a diff against a known baseline.

## Install

```
go get github.com/surrealdb/surrealdb.go/memory
```

### Migrating from `spectron`

This package was called `spectron` up to and including v1.6.0. The old import
path is **gone** as of v1.7.0 — there is no compatibility shim. Every exported
name is unchanged apart from the package qualifier, so the migration is a
find-and-replace:

```diff
-import "github.com/surrealdb/surrealdb.go/spectron"
+import "github.com/surrealdb/surrealdb.go/memory"

-client, err := spectron.New(ctx, endpoint, key)
+client, err := memory.New(ctx, endpoint, key)
```

Pin `v1.6.0` if you need the old path while you migrate.

## Client

A `*memory.Client` is pinned to a single context and hits `/api/v1/{context}/...`.

```go
client, err := memory.New(
    "acme-prod",                       // context id
    "https://api.memory.example",    // endpoint
    "sk-...",                          // api key
    memory.WithTimeout(30*time.Second),
    memory.WithMaxRetries(3),
)
```

| Argument | Required | Notes |
| --- | --- | --- |
| `contextID` | yes | Context id, e.g. `"acme-prod"`. |
| `endpoint`  | yes | Full URL of the Agent Memory host. Trailing slashes are trimmed. |
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
client.Remember(ctx, &memory.RememberRequest{Text: "I work at Acme as CTO"})

client.Remember(ctx, &memory.RememberRequest{
    Text:      "Acme acquired Beta",
    SessionID: "sess:abc",
    Scopes:    memory.ScopeSets{{"team/acme"}},
    Infer:     memory.InferFull,
})

client.RememberMany(ctx, &memory.RememberManyRequest{
    Messages: []memory.BatchMessage{
        {Role: memory.RoleUser, Content: "I just got promoted to CTO"},
        {Role: memory.RoleAssistant, Content: "Congratulations!"},
    },
    Extract: memory.ExtractWholeConversation,
})
```

`Remember` and `RememberMany` send an `Idempotency-Key` header derived from
`sha256(METHOD | path | body | 30s-bucket)`, so a retry inside the bucket
collapses onto the previous attempt server-side.

### Recall

```go
res, err := client.Recall(ctx, &memory.RecallRequest{
    Query: "what role do I have at Acme",
    K:     10,
    Mode:  memory.MemoryModeHybrid,
    Lens:  memory.ScopeSets{{"team/acme"}},
})
for _, hit := range res.Hits {
    fmt.Println(hit.Score, hit.Source, hit.Text)
}
fmt.Println(res.Tier, res.ClassificationKind)
```

### Forget

```go
client.Forget(ctx, "anything about my old job")
client.Forget(ctx, "draft notes", memory.WithPurge())
```

### Chat

```go
reply, _ := client.Chat(ctx, &memory.ChatRequest{Message: "what's my role?"})
fmt.Println(reply.Reply)

// Streaming via Go 1.23 range-over-func.
for chunk, err := range client.ChatStream(ctx, &memory.ChatRequest{Message: "what's my role?"}) {
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
    memory.WithFilename("returns.pdf"),
    memory.WithContentType("application/pdf"),
)
fmt.Println(res.ID, res.Status)

page, _ := client.Documents().List(ctx, memory.ListDocumentsOptions{Status: memory.DocReady})
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
_, err := client.Recall(ctx, &memory.RecallRequest{Query: "..."})
switch {
case errors.Is(err, memory.ErrNotFound):
    var api *memory.APIError
    errors.As(err, &api)
    fmt.Println(api.StatusCode, api.Message, api.TraceID)
case errors.Is(err, memory.ErrAuth):
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

Scope is a `memory.ScopeSets` (a `[][]string`): a DNF (OR-of-ANDs) selector.
The outer slice is an OR of clauses; each inner clause is an AND of hierarchical
scope paths in canonical slash form, e.g. `"team/eng"` or
`"org/apple/product/ipad"`. A key/value pair is written as the two segments
`"key/value"`. Empty represents the caller's default write region.

A single clause holding one path is the common case. To express co-ownership
(OR) use multiple clauses; to require several paths together (AND) put them in
one clause:

- `memory.ScopeSets{{"team/acme"}}` — a single scope.
- `memory.ScopeSets{{"team/a"}, {"team/b"}}` — `team/a OR team/b`.
- `memory.ScopeSets{{"team/b", "clearance/secret"}}` — `team/b AND clearance/secret`.

It is sent on the wire as a JSON array of arrays. Marshaling drops empty paths,
de-duplicates paths within each clause (preserving first-seen order), and drops
empty clauses, so equivalent inputs produce the same body and the
`Idempotency-Key` stays stable across retries.

```go
client.Remember(ctx, &memory.RememberRequest{
    Text:   "...",
    Scopes: memory.ScopeSets{{"team/acme"}},
})

client.Documents().Upload(ctx, body,
    memory.WithScopes(memory.ScopeSets{{"team/acme", "user/tobie"}}),
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
hits, _ := agent.Recall(ctx, &memory.RecallRequest{Query: "what's my role?"})
_, _ = agent.Documents().List(ctx, memory.ListDocumentsOptions{})

// Confirm the resolved identity, including the delegation in effect.
me, _ := agent.Whoami(ctx)
fmt.Println(me.PrincipalID, me.DelegatedPrincipalID, me.EffectiveGrants)
```

## Keys

`client.Keys()` manages scoped self-service bearer keys. The full secret is
returned only once, on `Create` and `Rotate`.

```go
minted, _ := client.Keys().Create(ctx, memory.CreateKeyRequest{
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
