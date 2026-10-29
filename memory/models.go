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
	Entity TripleEntity `json:"entity"`
	// Key is the attribute key. Leave it empty on a relation-only or
	// event-only triple.
	Key            string          `json:"key,omitempty"`
	Value          *string         `json:"value,omitempty"`
	Target         *TripleEntity   `json:"target,omitempty"`
	MemoryCategory *MemoryCategory `json:"memory_category,omitempty"`
	// Verb and Object describe an event rather than an attribute.
	Verb   *string `json:"verb,omitempty"`
	Object *string `json:"object,omitempty"`
	// Summary is a one-line rendering of the assertion.
	Summary    *string  `json:"summary,omitempty"`
	Confidence *float64 `json:"confidence,omitempty"`
	// OccurredAt is event time, distinct from assertion validity.
	OccurredAt *string `json:"occurred_at,omitempty"`
	ValidFrom  *string `json:"valid_from,omitempty"`
	ValidUntil *string `json:"valid_until,omitempty"`
	// TemporalHint is free text the extractor could not resolve to an
	// instant ("last spring"), kept so a later pass can.
	TemporalHint *string `json:"temporal_hint,omitempty"`
	// SourceClause is the span of the input this assertion came from.
	SourceClause *string `json:"source_clause,omitempty"`
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
	Actions       []ActionSummary      `json:"actions,omitempty"`
}

// ActionSummary is an event recorded by an extraction.
type ActionSummary struct {
	Actor          string         `json:"actor"`
	Verb           string         `json:"verb"`
	Object         string         `json:"object,omitempty"`
	Summary        string         `json:"summary"`
	MemoryCategory MemoryCategory `json:"memoryCategory"`
	OccurredAt     string         `json:"occurredAt,omitempty"`
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
	Confidence     float64        `json:"confidence"`
	Labels         []string       `json:"labels"`
	Scope          ScopeSets      `json:"scope"`
	Summary        string         `json:"summary,omitempty"`
	Source         *SourceRef     `json:"source,omitempty"`
}

// SourceRef is the compact provenance carried on attribute, relation and
// action rows.
type SourceRef struct {
	// Kind is the source kind: turn, document, upsert, reflect, elaboration
	// or consolidation.
	Kind string `json:"kind"`
	// Ref is a navigable ref — turn:<id>, doc:<id> or trace:<id> — when the
	// source carries one.
	Ref string `json:"ref,omitempty"`
	// SessionID is the session the source turn belongs to (turn kind only).
	SessionID string `json:"sessionId,omitempty"`
	// Title is the source document's title (document kind only).
	Title string `json:"title,omitempty"`
	// Trust is the source-level trust prior the row was written under.
	Trust *float64 `json:"trust,omitempty"`
}

// ActionDetail is the full row for a recorded event: who did what, to what,
// and when it happened.
type ActionDetail struct {
	ID string `json:"id"`
	// Actor is the navigable ref of the acting entity, entity:<type>/<name>.
	Actor string `json:"actor"`
	Verb  string `json:"verb"`
	// Object is the navigable ref of the acted-on entity, when it resolved to
	// one. Otherwise see ObjectText.
	Object string `json:"object,omitempty"`
	// ObjectText is the acted-on thing's verbatim name, when it did not
	// resolve to an entity.
	ObjectText     string         `json:"objectText,omitempty"`
	Summary        string         `json:"summary"`
	MemoryCategory MemoryCategory `json:"memoryCategory"`
	Confidence     float64        `json:"confidence"`
	// OccurredAt is the event time, distinct from assertion validity and from
	// learn time. Absent when the source did not carry one.
	OccurredAt string     `json:"occurredAt,omitempty"`
	ValidFrom  string     `json:"validFrom,omitempty"`
	ValidUntil string     `json:"validUntil,omitempty"`
	Source     *SourceRef `json:"source,omitempty"`
	CreatedAt  string     `json:"createdAt"`
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
	Confidence     float64        `json:"confidence"`
	Labels         []string       `json:"labels"`
	Scope          ScopeSets      `json:"scope"`
	Summary        string         `json:"summary,omitempty"`
	Source         *SourceRef     `json:"source,omitempty"`
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
	// Addressees names who the turn was directed at, so a fact stated to one
	// person is not read back as stated to everyone.
	Addressees []string `json:"addressees,omitempty"`
	// ObservedAt dates the assertion, for backfilling facts that were true
	// before they were recorded (RFC 3339).
	ObservedAt string `json:"observed_at,omitempty"`
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
	// Addressees names who this message was directed at.
	Addressees []string `json:"addressees,omitempty"`
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
	// IncludeDuplicates keeps near-identical hits that would otherwise be
	// collapsed. Nil leaves the server default.
	IncludeDuplicates *bool `json:"includeDuplicates,omitempty"`
}

// RecallHit is a single match returned by [Client.Recall].
type RecallHit struct {
	ID     string     `json:"id"`
	Score  float64    `json:"score"`
	Source ResultKind `json:"source"`
	Text   string     `json:"text"`
	// Resource addresses the row this hit came from, so the caller can
	// navigate to it rather than re-deriving it from Text.
	Resource *ResourceRef `json:"resource,omitempty"`
	// OccurredAt is the event time behind the hit, when it has one.
	OccurredAt string `json:"occurredAt,omitempty"`
	// DateNotes renders any temporal qualification the hit carries.
	DateNotes string `json:"dateNotes,omitempty"`
}

// ResourceRef addresses the substrate row behind a recall hit. Branch on Kind;
// each kind populates a different subset of the fields.
//
//	entity     EntityType, Name
//	attribute  EntityType, Name, Key
//	relation   SubjectType, SubjectName, Label, ObjectType, ObjectName
//	action     ActorType, ActorName, ObjectType, ObjectName
//	document   DocumentID, Position
//	session    SessionID, TurnID, Position
type ResourceRef struct {
	Kind string `json:"kind"`

	EntityType string `json:"entityType,omitempty"`
	Name       string `json:"name,omitempty"`
	Key        string `json:"key,omitempty"`

	SubjectType string `json:"subjectType,omitempty"`
	SubjectName string `json:"subjectName,omitempty"`
	Label       string `json:"label,omitempty"`
	ObjectType  string `json:"objectType,omitempty"`
	ObjectName  string `json:"objectName,omitempty"`

	ActorType string `json:"actorType,omitempty"`
	ActorName string `json:"actorName,omitempty"`

	DocumentID string `json:"documentId,omitempty"`
	SessionID  string `json:"sessionId,omitempty"`
	TurnID     string `json:"turnId,omitempty"`
	Position   *int64 `json:"position,omitempty"`
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
	// ContextHits are supporting rows retrieved alongside Hits — context for
	// the answer rather than answers themselves.
	ContextHits []RecallHit `json:"contextHits,omitempty"`
	// QueryWindow is the time range the query was understood to ask about,
	// when it carried one.
	QueryWindow *QueryWindow `json:"queryWindow,omitempty"`
}

// QueryWindow is the time range a query resolved to.
type QueryWindow struct {
	// Phrase is the wording the window came from ("last spring").
	Phrase string `json:"phrase"`
	Start  string `json:"start"`
	End    string `json:"end"`
	// Precision says how tightly the phrase pinned the range.
	Precision string `json:"precision"`
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
	// SuppressMarkers asks the server to omit the inline [S1]-style citation
	// markers from the reply text. The citations themselves are still
	// returned on [ChatResponse.Citations].
	SuppressMarkers bool `json:"suppressMarkers,omitempty"`
}

// ChatResponse is the (non-streaming) result of [Client.Chat].
type ChatResponse struct {
	Reply         string            `json:"reply"`
	SessionID     string            `json:"sessionId"`
	TraceID       string            `json:"traceId"`
	MemoryUpdates *ExtractionResult `json:"memoryUpdates,omitempty"`
	// Citations carries one entry per inline [S1]-style marker in Reply.
	Citations []Citation `json:"citations,omitempty"`
}

// Citation is one source backing a chat reply, addressed by the inline marker
// that appears in the reply text.
type Citation struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Marker string `json:"marker"`
	// Snippet is the quoted span from the source.
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
	// DocumentTitle is set when the source is a document passage.
	DocumentTitle string `json:"documentTitle,omitempty"`
	// PositionPercent locates the snippet within its document, 0-100.
	PositionPercent *int   `json:"positionPercent,omitempty"`
	OccurredAt      string `json:"occurredAt,omitempty"`
	Role            string `json:"role,omitempty"`
}

// UploadResponse is the result of [Documents.Upload] and [Documents.Reprocess].
type UploadResponse struct {
	ContentHash  string         `json:"contentHash"`
	Deduplicated bool           `json:"deduplicated"`
	ID           string         `json:"id"`
	Status       DocumentStatus `json:"status"`
	// ObservedAt is the ingest-time assertion instant the document's facts
	// are dated from.
	ObservedAt string `json:"observedAt,omitempty"`
}

// rawObject is a free-form JSON object the spec leaves untyped. Kept as
// json.RawMessage so callers can decode it into a concrete shape themselves.
type rawObject = json.RawMessage
