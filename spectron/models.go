package spectron

import (
	"encoding/json"
	"sort"
)

// Scope is a free-form principal scope. It is sent on the wire as a list
// of {"key":..., "value":...} objects, sorted by key for deterministic
// hashing (which matters for the Idempotency-Key calculation).
type Scope map[string]string

// MarshalJSON encodes Scope as [{"key":k, "value":v}, ...] sorted by key.
// A nil or empty Scope encodes as the JSON null literal so callers can use
// `omitempty` on Scope-typed fields to drop them entirely.
func (s Scope) MarshalJSON() ([]byte, error) {
	if len(s) == 0 {
		return []byte("null"), nil
	}
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	type pair struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	pairs := make([]pair, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, pair{Key: k, Value: s[k]})
	}
	return json.Marshal(pairs)
}

// ExtractionResult summarises the memory extraction work the server
// performed for a remember/chat turn. Nested summary lists are kept as
// untyped maps because their shapes are still evolving server-side.
type ExtractionResult struct {
	TurnID        string           `json:"turnId"`
	Entities      []map[string]any `json:"entities,omitempty"`
	Attributes    []map[string]any `json:"attributes,omitempty"`
	Relations     []map[string]any `json:"relations,omitempty"`
	Instructions  []map[string]any `json:"instructions,omitempty"`
	Uncertainties []map[string]any `json:"uncertainties,omitempty"`
	Corrections   []map[string]any `json:"corrections,omitempty"`
}

// RememberRequest is the input to [Client.Remember].
type RememberRequest struct {
	Text           string           `json:"text,omitempty"`
	Infer          string           `json:"infer,omitempty"`
	SessionID      string           `json:"sessionId,omitempty"`
	Scope          Scope            `json:"scope,omitempty"`
	Role           string           `json:"role,omitempty"`
	MemoryCategory string           `json:"memoryCategory,omitempty"`
	Triples        []map[string]any `json:"triples,omitempty"`
}

// RememberResponse is the result of [Client.Remember].
type RememberResponse struct {
	Mode       string            `json:"mode"`
	SessionID  string            `json:"sessionId"`
	ChunkID    string            `json:"chunkId,omitempty"`
	Extraction *ExtractionResult `json:"extraction,omitempty"`
	Preview    *bool             `json:"preview,omitempty"`
	TurnID     string            `json:"turnId,omitempty"`
}

// RememberManyRequest is the input to [Client.RememberMany].
type RememberManyRequest struct {
	Messages  []map[string]any `json:"messages"`
	SessionID string           `json:"sessionId,omitempty"`
	Scope     Scope            `json:"scope,omitempty"`
}

// RememberBatchResponse is the result of [Client.RememberMany].
type RememberBatchResponse struct {
	SessionID   string             `json:"sessionId"`
	TurnIDs     []string           `json:"turnIds,omitempty"`
	Extractions []ExtractionResult `json:"extractions,omitempty"`
}

// RecallRequest is the input to [Client.Recall].
type RecallRequest struct {
	Query     string   `json:"query"`
	K         int      `json:"k,omitempty"`
	Mode      string   `json:"mode,omitempty"`
	SessionID string   `json:"sessionId,omitempty"`
	Include   []string `json:"include,omitempty"`
	AsOf      string   `json:"asOf,omitempty"`
	AtInstant string   `json:"atInstant,omitempty"`
}

// RecallHit is a single match returned by [Client.Recall].
type RecallHit struct {
	ID     string  `json:"id"`
	Score  float64 `json:"score"`
	Source string  `json:"source"`
	Text   string  `json:"text"`
}

// RecallResponse is the result of [Client.Recall].
type RecallResponse struct {
	ClassificationKind string         `json:"classificationKind,omitempty"`
	Hits               []RecallHit    `json:"hits,omitempty"`
	QueryMS            int            `json:"queryMs,omitempty"`
	SeedEntities       []string       `json:"seedEntities,omitempty"`
	Tier               string         `json:"tier,omitempty"`
	Trace              map[string]any `json:"trace,omitempty"`
}

// ForgetResponse is the result of [Client.Forget].
type ForgetResponse struct {
	Deleted int `json:"deleted"`
}

// ChatRequest is the input to [Client.Chat] and [Client.ChatStream].
type ChatRequest struct {
	Message     string `json:"message"`
	SessionID   string `json:"sessionId,omitempty"`
	Scope       Scope  `json:"scope,omitempty"`
	Model       string `json:"model,omitempty"`
	BypassCache bool   `json:"bypassCache,omitempty"`
}

// ChatResponse is the (non-streaming) result of [Client.Chat].
type ChatResponse struct {
	Reply         string            `json:"reply"`
	SessionID     string            `json:"sessionId"`
	TraceID       string            `json:"traceId"`
	MemoryUpdates *ExtractionResult `json:"memoryUpdates,omitempty"`
}

// UploadResponse is the result of [Documents.Upload].
type UploadResponse struct {
	ContentHash  string `json:"contentHash"`
	Deduplicated bool   `json:"deduplicated"`
	ID           string `json:"id"`
	Status       string `json:"status"`
}
