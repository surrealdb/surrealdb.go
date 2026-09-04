package memory

import "encoding/json"

// ScopeSets is a DNF (OR-of-ANDs) scope selector. The outer slice is an OR of
// clauses; each inner clause is an AND of hierarchical scope paths in canonical
// slash form (e.g. "team/eng", "org/apple/product/ipad"). A key/value pair is
// written as the two segments "key/value". A single clause holding one path is
// the common case. Empty represents the caller's default write region.
//
// As an example, ScopeSets{{"team/a"}, {"team/b", "clearance/secret"}} means
// "team/a OR (team/b AND clearance/secret)".
//
// On the wire it is a JSON array of arrays of strings, matching the server's
// ScopeSets contract. [ScopeSets.MarshalJSON] drops empty paths, de-duplicates
// paths within each clause while preserving first-seen order, and drops any
// clause that ends up empty (the server rejects empty inner clauses); the
// order-stable output keeps the Idempotency-Key stable across retries.
type ScopeSets [][]string

// MarshalJSON encodes the selector as a JSON array of arrays of strings,
// dropping empty paths, de-duplicating paths within each clause, and dropping
// empty clauses.
func (s ScopeSets) MarshalJSON() ([]byte, error) {
	out := make([][]string, 0, len(s))
	for _, clause := range s {
		paths := make([]string, 0, len(clause))
		seen := make(map[string]struct{}, len(clause))
		for _, path := range clause {
			if path == "" {
				continue
			}
			if _, dup := seen[path]; dup {
				continue
			}
			seen[path] = struct{}{}
			paths = append(paths, path)
		}
		if len(paths) == 0 {
			continue
		}
		out = append(out, paths)
	}
	return json.Marshal(out)
}

// Triple is a structured fact supplied directly by the caller, bypassing LLM
// extraction. key + value create an attribute on entity; when value is empty
// and target is set, the triple is a relation edge from entity to target
// labeled by key. Consumed only when the write uses InferTriples.
type Triple struct {
	Entity         TripleEntity    `json:"entity"`
	Key            string          `json:"key"`
	Value          *string         `json:"value,omitempty"`
	Target         *TripleEntity   `json:"target,omitempty"`
	MemoryCategory *MemoryCategory `json:"memory_category,omitempty"`
}

// TripleEntity names an entity on a [Triple] by type and surface form.
type TripleEntity struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// GeoFilter narrows a query to a geographic region. Near and Within are
// mutually exclusive.
type GeoFilter struct {
	// Near matches rows within RadiusKm of a point.
	Near *GeoNear `json:"near,omitempty"`
	// Within matches rows inside a WKT polygon.
	Within string `json:"within,omitempty"`
}

// GeoNear is a point-and-radius geo predicate.
type GeoNear struct {
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	RadiusKm float64 `json:"radiusKm"`
}

// ExtractionResult summarizes the memory extraction work the server performed
// for a remember or chat turn.
type ExtractionResult struct {
	TurnID        string               `json:"turnId"`
	Entities      []EntitySummary      `json:"entities,omitempty"`
	Attributes    []AttributeSummary   `json:"attributes,omitempty"`
	Relations     []RelationSummary    `json:"relations,omitempty"`
	Instructions  []InstructionSummary `json:"instructions,omitempty"`
	Uncertainties []UncertaintySummary `json:"uncertainties,omitempty"`
	Corrections   []CorrectionSummary  `json:"corrections,omitempty"`
}

// EntitySummary is an entity touched by an extraction.
type EntitySummary struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	EntityType     string         `json:"entityType"`
	MemoryCategory MemoryCategory `json:"memoryCategory"`
	IsNew          bool           `json:"isNew"`
}

// AttributeSummary is an attribute written by an extraction.
type AttributeSummary struct {
	ID             string         `json:"id"`
	EntityID       string         `json:"entityId"`
	Key            string         `json:"key"`
	Value          string         `json:"value"`
	MemoryCategory MemoryCategory `json:"memoryCategory"`
}

// RelationSummary is a relation edge written by an extraction.
type RelationSummary struct {
	Subject        string         `json:"subject"`
	Label          string         `json:"label"`
	Object         string         `json:"object"`
	MemoryCategory MemoryCategory `json:"memoryCategory"`
}

// InstructionSummary is a standing instruction surfaced by an extraction.
type InstructionSummary struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// UncertaintySummary records something the extractor could not resolve.
type UncertaintySummary struct {
	About  string `json:"about"`
	Reason string `json:"reason"`
}

// CorrectionSummary records a value the extraction superseded.
type CorrectionSummary struct {
	EntityID string `json:"entityId"`
	Key      string `json:"key"`
	OldValue string `json:"oldValue"`
	NewValue string `json:"newValue"`
}

// AttributeDetail is the full row for an attribute, including temporal bounds
// and the supersession chain.
type AttributeDetail struct {
	ID             string         `json:"id"`
	Entity         string         `json:"entity"`
	Key            string         `json:"key"`
	Value          string         `json:"value"`
	MemoryCategory MemoryCategory `json:"memoryCategory"`
	Importance     float64        `json:"importance"`
	CreatedAt      string         `json:"createdAt"`
	ValidFrom      string         `json:"validFrom,omitempty"`
	ValidUntil     string         `json:"validUntil,omitempty"`
	Supersedes     string         `json:"supersedes,omitempty"`
	SupersededBy   string         `json:"supersededBy,omitempty"`
}

// EntityDetail is the full row for an entity.
type EntityDetail struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	EntityType     string         `json:"entityType"`
	MemoryCategory MemoryCategory `json:"memoryCategory"`
	Importance     float64        `json:"importance"`
	CreatedAt      string         `json:"createdAt"`
	UpdatedAt      string         `json:"updatedAt"`
}

// RelationDetail is the full row for a relation edge.
type RelationDetail struct {
	ID             string         `json:"id"`
	Subject        string         `json:"subject"`
	Label          string         `json:"label"`
	Object         string         `json:"object"`
	MemoryCategory MemoryCategory `json:"memoryCategory"`
	CreatedAt      string         `json:"createdAt"`
	ValidFrom      string         `json:"validFrom,omitempty"`
	ValidUntil     string         `json:"validUntil,omitempty"`
}

// RememberRequest is the input to [Client.Remember]. It maps to the spec's
// FactsRequest, whose request body is snake_case (session_id, memory_category).
type RememberRequest struct {
	Text           string          `json:"text,omitempty"`
	Infer          InferMode       `json:"infer,omitempty"`
	SessionID      string          `json:"session_id,omitempty"`
	Scopes         ScopeSets       `json:"scopes,omitempty"`
	Role           *TurnRole       `json:"role,omitempty"`
	MemoryCategory *MemoryCategory `json:"memory_category,omitempty"`
	Labels         []string        `json:"labels,omitempty"`
	Triples        []Triple        `json:"triples,omitempty"`
}

// RememberResponse is the result of [Client.Remember].
type RememberResponse struct {
	Mode       InferMode         `json:"mode"`
	SessionID  string            `json:"sessionId"`
	ChunkID    string            `json:"chunkId,omitempty"`
	Extraction *ExtractionResult `json:"extraction,omitempty"`
	Preview    bool              `json:"preview,omitempty"`
	TurnID     string            `json:"turnId,omitempty"`
}

// BatchMessage is one message in a [Client.RememberMany] batch.
type BatchMessage struct {
	Role    TurnRole `json:"role"`
	Content string   `json:"content"`
	// Timestamp is an optional RFC 3339 instant for the message.
	Timestamp string `json:"ts,omitempty"`
}

// RememberManyRequest is the input to [Client.RememberMany]. It maps to the
// spec's FactsBatchRequest, whose request body is snake_case (session_id).
type RememberManyRequest struct {
	Messages  []BatchMessage      `json:"messages"`
	Extract   BatchExtractionMode `json:"extract,omitempty"`
	Infer     InferMode           `json:"infer,omitempty"`
	SessionID string              `json:"session_id,omitempty"`
	Scopes    ScopeSets           `json:"scopes,omitempty"`
	Labels    []string            `json:"labels,omitempty"`
}

// RememberBatchResponse is the result of [Client.RememberMany].
type RememberBatchResponse struct {
	SessionID   string             `json:"sessionId"`
	TurnIDs     []string           `json:"turnIds,omitempty"`
	Extractions []ExtractionResult `json:"extractions,omitempty"`
}

// RecallRequest is the input to [Client.Recall]. It maps to the spec's
// QueryMemoryRequestJson (camelCase body).
type RecallRequest struct {
	Query      string          `json:"query"`
	K          int             `json:"k,omitempty"`
	Mode       MemoryQueryMode `json:"mode,omitempty"`
	SessionID  string          `json:"sessionId,omitempty"`
	Include    []string        `json:"include,omitempty"`
	Labels     []string        `json:"labels,omitempty"`
	Lens       ScopeSets       `json:"lens,omitempty"`
	ScopeView  string          `json:"scopeView,omitempty"`
	Source     string          `json:"source,omitempty"`
	Location   *GeoFilter      `json:"location,omitempty"`
	AsOf       string          `json:"asOf,omitempty"`
	AtInstant  string          `json:"atInstant,omitempty"`
	ValidFrom  string          `json:"validFrom,omitempty"`
	ValidUntil string          `json:"validUntil,omitempty"`
}

// RecallHit is a single match returned by [Client.Recall].
type RecallHit struct {
	ID     string     `json:"id"`
	Score  float64    `json:"score"`
	Source ResultKind `json:"source"`
	Text   string     `json:"text"`
}

// QueryTrace is the short trace summary returned inline on a recall.
type QueryTrace struct {
	TraceID        string    `json:"traceId"`
	ResolutionTier string    `json:"resolutionTier"`
	TierReason     string    `json:"tierReason"`
	RetrievedCount int       `json:"retrievedCount"`
	LatencyMs      int       `json:"latencyMs"`
	TopScores      []float64 `json:"topScores"`
}

// RecallResponse is the result of [Client.Recall].
type RecallResponse struct {
	ClassificationKind QueryKind   `json:"classificationKind"`
	Hits               []RecallHit `json:"hits"`
	QueryMS            int         `json:"queryMs"`
	SeedEntities       []string    `json:"seedEntities"`
	Tier               Tier        `json:"tier"`
	Trace              QueryTrace  `json:"trace"`
}

// ForgetResponse is the result of [Client.Forget].
type ForgetResponse struct {
	Deleted int `json:"deleted"`
}

// ChatRequest is the input to [Client.Chat] and [Client.ChatStream]. It maps to
// the spec's ChatRequestJson (camelCase body).
type ChatRequest struct {
	Message     string    `json:"message"`
	SessionID   string    `json:"sessionId,omitempty"`
	Scopes      ScopeSets `json:"scopes,omitempty"`
	Model       string    `json:"model,omitempty"`
	Labels      []string  `json:"labels,omitempty"`
	BypassCache bool      `json:"bypassCache,omitempty"`
}

// ChatResponse is the (non-streaming) result of [Client.Chat].
type ChatResponse struct {
	Reply         string            `json:"reply"`
	SessionID     string            `json:"sessionId"`
	TraceID       string            `json:"traceId"`
	MemoryUpdates *ExtractionResult `json:"memoryUpdates,omitempty"`
}

// UploadResponse is the result of [Documents.Upload] and [Documents.Reprocess].
type UploadResponse struct {
	ContentHash  string         `json:"contentHash"`
	Deduplicated bool           `json:"deduplicated"`
	ID           string         `json:"id"`
	Status       DocumentStatus `json:"status"`
}

// rawObject is a free-form JSON object the spec leaves untyped. Kept as
// json.RawMessage so callers can decode it into a concrete shape themselves.
type rawObject = json.RawMessage
