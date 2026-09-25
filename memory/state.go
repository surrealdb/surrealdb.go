package memory

import (
	"context"
	"iter"
	"net/http"
	"net/url"
)

// CategoryState is the resolved attributes, entities, and relations for one
// memory category.
type CategoryState struct {
	Attributes []AttributeDetail `json:"attributes"`
	Entities   []EntityDetail    `json:"entities"`
	Relations  []RelationDetail  `json:"relations"`
	Actions    []ActionDetail    `json:"actions"`
}

// StateResponse is the result of [Client.State]: the context's memory grouped
// by category, plus standing instructions and open unknowns.
type StateResponse struct {
	Identity     CategoryState        `json:"identity"`
	Knowledge    CategoryState        `json:"knowledge"`
	Context      CategoryState        `json:"context"`
	Instructions []InstructionSummary `json:"instructions"`
	Unknowns     []UncertaintySummary `json:"unknowns"`
	// Truncated says which underlying tables were bounded short. A true flag
	// means the complete set must be read through that table's own
	// collection endpoint — except Instructions, which has none, so raise
	// Limit instead.
	Truncated StateTruncation `json:"truncated"`
}

// StateTruncation reports which of the state read's underlying tables were
// bounded short.
type StateTruncation struct {
	Entities     bool `json:"entities"`
	Attributes   bool `json:"attributes"`
	Relations    bool `json:"relations"`
	Actions      bool `json:"actions"`
	Instructions bool `json:"instructions"`
	Unknowns     bool `json:"unknowns"`
}

// State returns the full resolved memory state for the context.
func (c *Client) State(ctx context.Context) (*StateResponse, error) {
	var out StateResponse
	if err := c.getJSON(ctx, c.base+"/state", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProfileEntry is a single key/value pair in a [ProfileResponse] section.
type ProfileEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	// Entity is the navigable ref of the subject, when the entry has one.
	Entity     string `json:"entity,omitempty"`
	EntityType string `json:"entityType,omitempty"`
}

// ProfileResponse is the result of [Client.Profile]: the caller-facing profile
// split into static, dynamic, and preference entries plus instructions.
type ProfileResponse struct {
	Static       []ProfileEntry       `json:"static"`
	Dynamic      []ProfileEntry       `json:"dynamic"`
	Preferences  []ProfileEntry       `json:"preferences"`
	Instructions []InstructionSummary `json:"instructions"`
	// SelfFacts are the entries about the context's own principal, kept
	// separate from what it knows about everyone else.
	SelfFacts []ProfileEntry `json:"selfFacts"`
}

// Profile returns the assembled profile for the context.
func (c *Client) Profile(ctx context.Context) (*ProfileResponse, error) {
	var out ProfileResponse
	if err := c.getJSON(ctx, c.base+"/profile", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AuditOptions filter a [Client.Audit] listing. All fields are optional.
type AuditOptions struct {
	Principal string
	Key       string
	Kind      string
	// Since and Until bound the createdAt window (RFC 3339 instants).
	Since string
	Until string

	// Limit and Cursor page the listing; see [CursorOptions], whose fields
	// these mirror. The audit listing offers no total, so there is no Count.
	Limit  int
	Cursor string
}

func (o *AuditOptions) values() url.Values {
	q := url.Values{}
	if o == nil {
		return q
	}
	if o.Principal != "" {
		q.Set("principal", o.Principal)
	}
	if o.Key != "" {
		q.Set("key", o.Key)
	}
	if o.Kind != "" {
		q.Set("kind", o.Kind)
	}
	if o.Since != "" {
		q.Set("since", o.Since)
	}
	if o.Until != "" {
		q.Set("until", o.Until)
	}
	CursorOptions{Limit: o.Limit, Cursor: o.Cursor}.apply(q)
	return q
}

// AuditRow is a single audited operation.
type AuditRow struct {
	TraceID     string    `json:"traceId"`
	Kind        TraceKind `json:"kind"`
	Principal   string    `json:"principal,omitempty"`
	Model       string    `json:"model,omitempty"`
	Cost        float64   `json:"cost"`
	LatencyMs   int       `json:"latencyMs"`
	RowsTouched int       `json:"rowsTouched"`
	CreatedAt   string    `json:"createdAt"`
}

// AuditResponse is a page of audited operations from [Client.Audit].
type AuditResponse struct {
	Rows []AuditRow `json:"rows"`
	Page PageMeta   `json:"page"`
}

// Audit returns one page of audited operations for the context, newest first.
// A nil opts applies no filters and lets the server pick the page size.
func (c *Client) Audit(ctx context.Context, opts *AuditOptions) (*AuditResponse, error) {
	var out AuditResponse
	if err := c.getJSON(ctx, c.base+"/audit", opts.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AllAudit walks every page of the audit listing, newest first.
func (c *Client) AllAudit(ctx context.Context, opts *AuditOptions) iter.Seq2[AuditRow, error] {
	local := AuditOptions{}
	if opts != nil {
		local = *opts
	}
	return walkPages(ctx, local.Cursor, func(ctx context.Context, cursor string) ([]AuditRow, PageMeta, error) {
		local.Cursor = cursor
		page, err := c.Audit(ctx, &local)
		if err != nil {
			return nil, PageMeta{}, err
		}
		return page.Rows, page.Page, nil
	})
}

// LifecycleResponse reports how many rows a lifecycle pass affected.
type LifecycleResponse struct {
	Affected int `json:"affected"`
}

// DecayImportance runs the importance-decay lifecycle pass over the context.
func (c *Client) DecayImportance(ctx context.Context) (*LifecycleResponse, error) {
	var out LifecycleResponse
	if err := c.doJSON(ctx, http.MethodPost, c.base+"/lifecycle/decay", nil, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

// ExpireContext runs the expiry lifecycle pass over the context.
func (c *Client) ExpireContext(ctx context.Context) (*LifecycleResponse, error) {
	var out LifecycleResponse
	if err := c.doJSON(ctx, http.MethodPost, c.base+"/lifecycle/expire", nil, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}
