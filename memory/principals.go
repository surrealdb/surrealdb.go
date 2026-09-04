package memory

import (
	"context"
	"net/http"
	"net/url"
)

// Principals is the principal sub-client returned by [Client.Principals].
type Principals struct {
	client *Client
}

// Principal is an actor known to the context, with its scope grants. Grants
// maps a scope pattern to the verbs permitted on it.
type Principal struct {
	ID          string              `json:"id"`
	Kind        string              `json:"kind"`
	DisplayName string              `json:"displayName"`
	Grants      map[string][]string `json:"grants"`
}

// List returns the principals known to the context.
func (p *Principals) List(ctx context.Context) ([]Principal, error) {
	var out []Principal
	if err := p.client.getJSON(ctx, p.client.base+"/principals", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get fetches a single principal by id.
func (p *Principals) Get(ctx context.Context, principalID string) (*Principal, error) {
	path := p.client.base + "/principals/" + url.PathEscape(principalID)
	var out Principal
	if err := p.client.getJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EffectiveGrants is the resolved grant for a principal at a scope path.
type EffectiveGrants struct {
	Path  string   `json:"path"`
	Verbs []string `json:"verbs"`
	AsOf  string   `json:"asOf,omitempty"`
}

// Effective resolves the verbs a principal effectively holds at a scope path,
// optionally as of a past instant.
func (p *Principals) Effective(ctx context.Context, principalID, path, asOf string) (*EffectiveGrants, error) {
	q := url.Values{}
	if path != "" {
		q.Set("path", path)
	}
	if asOf != "" {
		q.Set("asOf", asOf)
	}
	reqPath := p.client.base + "/principals/" + url.PathEscape(principalID) + "/effective"
	var out EffectiveGrants
	if err := p.client.getJSON(ctx, reqPath, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GrantRequest is the input to [Principals.Grant] and [Principals.Revoke]. Path
// is a scope pattern (a path or a /* subtree) and Verbs are the permissions to
// grant or revoke on it.
type GrantRequest struct {
	Path  string   `json:"path"`
	Verbs []string `json:"verbs"`
}

// Grant adds the requested verbs to a principal at a scope pattern and returns
// the updated principal.
func (p *Principals) Grant(ctx context.Context, principalID string, req GrantRequest) (*Principal, error) {
	path := p.client.base + "/principals/" + url.PathEscape(principalID) + "/grants"
	var out Principal
	if err := p.client.doJSON(ctx, http.MethodPost, path, req, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

// Revoke removes the requested verbs from a principal at a scope pattern and
// returns the updated principal. The verbs travel in the request body, which
// the spec allows on this DELETE.
func (p *Principals) Revoke(ctx context.Context, principalID string, req GrantRequest) (*Principal, error) {
	path := p.client.base + "/principals/" + url.PathEscape(principalID) + "/grants"
	var out Principal
	if err := p.client.doJSON(ctx, http.MethodDelete, path, req, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}
