package spectron

import (
	"context"
	"net/http"
	"net/url"
)

// ConsolidateRequest is the input to [Client.Consolidate].
type ConsolidateRequest struct {
	// DryRun reports what would change without writing.
	DryRun bool `json:"dryRun,omitempty"`
	// FactLimit caps the number of facts considered.
	FactLimit int `json:"factLimit,omitempty"`
	// ObservationLimit caps the number of observations considered.
	ObservationLimit int `json:"observationLimit,omitempty"`
}

// ConsolidateOutcome is one entity/key the consolidation pass acted on.
type ConsolidateOutcome struct {
	EntityName    string       `json:"entityName"`
	Key           string       `json:"key"`
	Value         string       `json:"value"`
	Kind          DecisionKind `json:"kind"`
	ProofCount    int          `json:"proofCount"`
	ObservationID string       `json:"observationId,omitempty"`
	Rationale     string       `json:"rationale,omitempty"`
}

// ConsolidateResponse is the result of [Client.Consolidate].
type ConsolidateResponse struct {
	Created    int                  `json:"created"`
	Updated    int                  `json:"updated"`
	Superseded int                  `json:"superseded"`
	DryRun     bool                 `json:"dryRun"`
	Outcomes   []ConsolidateOutcome `json:"outcomes"`
	TraceID    string               `json:"traceId"`
}

// Consolidate folds repeated observations into durable attributes for the
// context.
func (c *Client) Consolidate(ctx context.Context, req ConsolidateRequest) (*ConsolidateResponse, error) {
	var out ConsolidateResponse
	if err := c.doJSON(ctx, http.MethodPost, c.base+"/consolidate", req, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

// ReflectRequest is the input to [Client.Reflect].
type ReflectRequest struct {
	Query string `json:"query"`
	// Persist writes the reflection's derived attributes back to the context.
	Persist bool `json:"persist,omitempty"`
}

// ReflectResponse is the result of [Client.Reflect].
type ReflectResponse struct {
	Reflection          string             `json:"reflection"`
	Evidence            []string           `json:"evidence"`
	PersistedAttributes []AttributeSummary `json:"persistedAttributes"`
	TraceID             string             `json:"traceId"`
}

// Reflect runs an LLM reflection over the context's memory and optionally
// persists what it derives.
func (c *Client) Reflect(ctx context.Context, req ReflectRequest) (*ReflectResponse, error) {
	var out ReflectResponse
	if err := c.doJSON(ctx, http.MethodPost, c.base+"/reflect", req, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

// ElaborateRequest is the input to [Client.Elaborate].
type ElaborateRequest struct {
	// EntityRef targets a single entity; leave empty with Sweep to scan many.
	EntityRef string `json:"entityRef,omitempty"`
	// Sweep elaborates across entities rather than a single EntityRef.
	Sweep bool `json:"sweep,omitempty"`
	// Budget caps how much work the pass performs.
	Budget int `json:"budget,omitempty"`
	// DryRun reports proposed relations without emitting them.
	DryRun bool `json:"dryRun,omitempty"`
}

// ElaborateProposedRelation is a relation the elaboration pass proposes.
type ElaborateProposedRelation struct {
	Subject string `json:"subject"`
	Label   string `json:"label"`
	Object  string `json:"object"`
}

// ElaborateOutcome is the elaboration result for a single entity.
type ElaborateOutcome struct {
	EntityName        string                      `json:"entityName"`
	EntityType        string                      `json:"entityType"`
	ProposedRelations []ElaborateProposedRelation `json:"proposedRelations"`
	RelationsEmitted  int                         `json:"relationsEmitted"`
	DryRun            bool                        `json:"dryRun"`
	TraceID           string                      `json:"traceId"`
}

// ElaborateResponse is the result of [Client.Elaborate].
type ElaborateResponse struct {
	Outcomes         []ElaborateOutcome `json:"outcomes"`
	RelationsEmitted int                `json:"relationsEmitted"`
}

// Elaborate proposes (and optionally emits) new relations by reasoning over an
// entity's existing neighborhood.
func (c *Client) Elaborate(ctx context.Context, req ElaborateRequest) (*ElaborateResponse, error) {
	var out ElaborateResponse
	if err := c.doJSON(ctx, http.MethodPost, c.base+"/elaborate", req, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

// InspectOptions narrow a [Client.Inspect] call. All fields are optional.
type InspectOptions struct {
	// Ref addresses a specific row or entity to inspect.
	Ref string
	// AsOf walks the supersession chain to the row current at this instant.
	AsOf string
	// AtInstant reads substrate state at this system-time instant.
	AtInstant string
	// ValidFrom and ValidUntil bound valid-time (world-time).
	ValidFrom  string
	ValidUntil string
}

func (o InspectOptions) values() url.Values {
	q := url.Values{}
	if o.Ref != "" {
		q.Set("ref", o.Ref)
	}
	if o.AsOf != "" {
		q.Set("asOf", o.AsOf)
	}
	if o.AtInstant != "" {
		q.Set("atInstant", o.AtInstant)
	}
	if o.ValidFrom != "" {
		q.Set("validFrom", o.ValidFrom)
	}
	if o.ValidUntil != "" {
		q.Set("validUntil", o.ValidUntil)
	}
	return q
}

// Inspect returns a low-level diagnostic view of the substrate. The shape is
// deliberately untyped in the spec, so the raw JSON object is returned for the
// caller to decode.
func (c *Client) Inspect(ctx context.Context, opts InspectOptions) (rawObject, error) {
	var out rawObject
	if err := c.getJSON(ctx, c.base+"/inspect", opts.values(), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FsckRequest is the input to [Client.Fsck].
type FsckRequest struct {
	// Check selects which integrity checks to run; empty runs all of them.
	Check string `json:"check,omitempty"`
	// DuplicateThreshold is the similarity cutoff for the duplicate check.
	DuplicateThreshold float64 `json:"duplicateThreshold,omitempty"`
	// MaxResults caps the findings returned per category.
	MaxResults int `json:"maxResults,omitempty"`
}

// ContradictionFinding reports an entity/key holding conflicting values.
type ContradictionFinding struct {
	Entity string   `json:"entity"`
	Key    string   `json:"key"`
	Values []string `json:"values"`
}

// DuplicateFinding reports two entities that look like duplicates.
type DuplicateFinding struct {
	EntityA    string  `json:"entityA"`
	EntityB    string  `json:"entityB"`
	Similarity float64 `json:"similarity"`
}

// InjectionFinding reports a row that looks like a prompt-injection attempt.
type InjectionFinding struct {
	Kind    InjectionKind `json:"kind"`
	RowID   string        `json:"rowId"`
	Snippet string        `json:"snippet"`
}

// FsckReport is the result of [Client.Fsck].
type FsckReport struct {
	Contradictions []ContradictionFinding `json:"contradictions"`
	Duplicates     []DuplicateFinding     `json:"duplicates"`
	Injection      []InjectionFinding     `json:"injection"`
	Total          int                    `json:"total"`
}

// Fsck runs integrity checks over the context's memory: contradictions,
// duplicate entities, and suspected prompt injection.
func (c *Client) Fsck(ctx context.Context, req FsckRequest) (*FsckReport, error) {
	var out FsckReport
	if err := c.doJSON(ctx, http.MethodPost, c.base+"/fsck", req, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

// ContextQueryRequest is the input to [Client.QueryContext].
type ContextQueryRequest struct {
	Query     string    `json:"query"`
	K         int       `json:"k,omitempty"`
	Labels    []string  `json:"labels,omitempty"`
	Lens      ScopeSets `json:"lens,omitempty"`
	ScopeView string    `json:"scopeView,omitempty"`
}

// ContextQueryResponse is the result of [Client.QueryContext]: a single fused
// context string suitable for prompt assembly.
type ContextQueryResponse struct {
	Context string `json:"context"`
	QueryMS int    `json:"queryMs"`
	Tier    string `json:"tier"`
}

// QueryContext returns a single composed context string for the query, ready to
// splice into a prompt.
func (c *Client) QueryContext(ctx context.Context, req ContextQueryRequest) (*ContextQueryResponse, error) {
	var out ContextQueryResponse
	if err := c.doJSON(ctx, http.MethodPost, c.base+"/context", req, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}
