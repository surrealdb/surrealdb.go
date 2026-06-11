package spectron

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// CategoryState is the resolved attributes, entities, and relations for one
// memory category.
type CategoryState struct {
	Attributes []AttributeDetail `json:"attributes"`
	Entities   []EntityDetail    `json:"entities"`
	Relations  []RelationDetail  `json:"relations"`
}

// StateResponse is the result of [Client.State]: the context's memory grouped
// by category, plus standing instructions and open unknowns.
type StateResponse struct {
	Identity     CategoryState        `json:"identity"`
	Knowledge    CategoryState        `json:"knowledge"`
	Context      CategoryState        `json:"context"`
	Instructions []InstructionSummary `json:"instructions"`
	Unknowns     []UncertaintySummary `json:"unknowns"`
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
}

// ProfileResponse is the result of [Client.Profile]: the caller-facing profile
// split into static, dynamic, and preference entries plus instructions.
type ProfileResponse struct {
	Static       []ProfileEntry       `json:"static"`
	Dynamic      []ProfileEntry       `json:"dynamic"`
	Preferences  []ProfileEntry       `json:"preferences"`
	Instructions []InstructionSummary `json:"instructions"`
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
	// Limit caps the number of rows returned.
	Limit int
}

func (o AuditOptions) values() url.Values {
	q := url.Values{}
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
	if o.Limit > 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
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

// AuditResponse is the result of [Client.Audit].
type AuditResponse struct {
	Rows []AuditRow `json:"rows"`
}

// Audit lists audited operations for the context, newest first.
func (c *Client) Audit(ctx context.Context, opts AuditOptions) (*AuditResponse, error) {
	var out AuditResponse
	if err := c.getJSON(ctx, c.base+"/audit", opts.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
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
