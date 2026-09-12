package memory

import (
	"context"
	"net/http"
)

// LookupSection names one section of a [LookupResponse].
type LookupSection string

// Known values for [LookupSection]. Note that "entities" is not selectable:
// it is filled by resolution, not requested.
const (
	SectionFacts       LookupSection = "facts"
	SectionRelations   LookupSection = "relations"
	SectionEvents      LookupSection = "events"
	SectionPassages    LookupSection = "passages"
	SectionUncertainty LookupSection = "uncertainty"
)

// LookupRequest is the input to [Client.Lookup].
type LookupRequest struct {
	// Query is what the reader asked about.
	Query string `json:"query"`
	// Subject skips resolution and answers about this subject directly, as
	// <type>/<name>. Use it to walk a trail cheaply once you already know
	// which entity you mean.
	Subject string `json:"subject,omitempty"`
	// EntityType restricts resolution to one entity type.
	EntityType string `json:"entityType,omitempty"`
	// AmbiguityMargin is how far the top candidate must beat the runner-up
	// for the answer to be one entity rather than a list. Zero means the
	// server default (0.15).
	AmbiguityMargin float64 `json:"ambiguityMargin,omitempty"`
	// Include selects which sections to fill. Empty means everything except
	// [SectionPassages], which costs a vector search.
	Include []LookupSection `json:"include,omitempty"`

	// Per-section caps. Zero means the server default.
	FactLimit        int `json:"factLimit,omitempty"`
	RelationLimit    int `json:"relationLimit,omitempty"`
	EventLimit       int `json:"eventLimit,omitempty"`
	PassageLimit     int `json:"passageLimit,omitempty"`
	UncertaintyLimit int `json:"uncertaintyLimit,omitempty"`
}

// ResolutionKind discriminates the arms of a [Resolution].
type ResolutionKind string

// The four ways a lookup query can resolve.
const (
	// ResolutionEntity means one subject, confidently.
	ResolutionEntity ResolutionKind = "entity"
	// ResolutionAmbiguous means several candidates within the ambiguity
	// margin of each other.
	ResolutionAmbiguous ResolutionKind = "ambiguous"
	// ResolutionTopic means the query named no single thing; the answer is
	// the cluster in the Entities and Passages sections.
	ResolutionTopic ResolutionKind = "topic"
	// ResolutionEmpty means nothing on record.
	ResolutionEmpty ResolutionKind = "empty"
)

// Resolution is what a lookup query resolved to: a tagged union over Kind.
//
// Branch on Kind. Do not infer the arm from which slices are populated — an
// entity answer with no neighbours and an empty answer with no near misses
// both leave every slice nil.
type Resolution struct {
	Kind ResolutionKind `json:"kind"`

	// Subject and Confidence are set when Kind is [ResolutionEntity].
	// Confidence is the winning candidate's score; 1.0 is an exact identity
	// match.
	Subject    *EntityMatch `json:"subject,omitempty"`
	Confidence float64      `json:"confidence,omitempty"`

	// Candidates is set when Kind is [ResolutionAmbiguous]. Each carries a
	// Distinguisher, because four rows of "product, 3 days ago" would not let
	// anyone choose.
	Candidates []EntityMatch `json:"candidates,omitempty"`

	// Nearest is set when Kind is [ResolutionEmpty]: what the name search did
	// turn up, so the reader can tell "not stored" from "stored under another
	// name".
	Nearest []EntityMatch `json:"nearest,omitempty"`
}

// Section is a bounded slice of one lookup section, with the flag that says
// whether it was cut.
//
// Truncated comes from a probe row, never a count: the read fetches one row
// past the limit and drops it, so "there is more" costs nothing. It is the
// signal to follow the section's own collection endpoint, not to re-request
// the lookup with a bigger limit.
type Section[T any] struct {
	Items []T `json:"items"`
	// Truncated reports that the section has more rows than it carries.
	Truncated bool `json:"truncated"`
}

// Passage is a document passage behind a lookup answer.
type Passage struct {
	Text       string  `json:"text"`
	Score      float64 `json:"score"`
	DocumentID string  `json:"documentId,omitempty"`
	Position   *int64  `json:"position,omitempty"`
	OccurredAt string  `json:"occurredAt,omitempty"`
}

// Coverage says where an answer's facts came from.
type Coverage struct {
	// SourceKinds counts how many of the facts in this answer came from each
	// source kind.
	SourceKinds map[string]int64 `json:"sourceKinds"`
}

// LookupResponse is the result of [Client.Lookup].
//
// Every section is bounded and reports its own Truncated flag; none of them
// page. A truncated section is a genuine prefix of its collection's walk, in
// the same newest-first order — with one exception: Facts is ranked by
// importance while /attributes pages in write order, so the ranked head is a
// different question, not the walk's first page.
//
// The walk for each section:
//
//	Facts        [Facts.AllAttributes] filtered by entity
//	Relations    [Facts.AllEdgesOf] — one direction alone reproduces half
//	Events       [Facts.AllActions] filtered by actor
//	Passages     [Client.Recall]
//	Uncertainty  [Uncertainty.All] filtered by entity
//
// A section that was not requested comes back empty with Truncated false: it
// was declined, not cut, and points at no walk.
type LookupResponse struct {
	Resolution Resolution `json:"resolution"`
	// Entities holds the entities a topic query spans. Empty for an entity
	// answer, where the subject is on Resolution instead.
	Entities    Section[EntityMatch]     `json:"entities"`
	Facts       Section[AttributeDetail] `json:"facts"`
	Relations   Section[RelationDetail]  `json:"relations"`
	Events      Section[ActionDetail]    `json:"events"`
	Passages    Section[Passage]         `json:"passages"`
	Uncertainty Section[UnknownFact]     `json:"uncertainty"`
	Coverage    *Coverage                `json:"coverage,omitempty"`
}

// Lookup answers "what does this context know about X" in one round trip.
//
// It is deterministic and composite: every row is stored, nothing is
// summarised by a model, and the same query returns the same answer. That is
// what separates it from [Client.Recall], which ranks by relevance, and from
// [Client.Chat], which composes a reply.
//
// Lookup is a read behind a POST, so it carries an Idempotency-Key and is
// retried on transport failures and 5xx.
func (c *Client) Lookup(ctx context.Context, req *LookupRequest) (*LookupResponse, error) {
	var out LookupResponse
	if err := c.doJSON(ctx, http.MethodPost, c.base+"/lookup", req, &out, true); err != nil {
		return nil, err
	}
	return &out, nil
}
