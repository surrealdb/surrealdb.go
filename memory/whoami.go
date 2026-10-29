package memory

import "context"

// WhoamiResponse is the result of [Client.Whoami]: the resolved identity and
// grants for the calling token, including any delegation in effect. The grant
// maps are the spec's Grants shape, keying a grant verb (`memory:read`,
// `scope:create`, ...) to the scope patterns it permits.
type WhoamiResponse struct {
	PrincipalID          string              `json:"principalId"`
	DisplayName          string              `json:"displayName"`
	Kind                 string              `json:"kind"`
	Enforce              bool                `json:"enforce"`
	Grants               map[string][]string `json:"grants"`
	EffectiveGrants      map[string][]string `json:"effectiveGrants"`
	DelegatedPrincipalID string              `json:"delegatedPrincipalId,omitempty"`
	TokenGrants          map[string][]string `json:"tokenGrants,omitempty"`
}

// Whoami resolves the identity and effective grants for the calling token
// (GET /api/v1/{context}/me). When the Client was derived via
// [Client.OnBehalfOf], the response reflects the delegated principal and
// reports the delegating principal in DelegatedPrincipalID.
func (c *Client) Whoami(ctx context.Context) (*WhoamiResponse, error) {
	var out WhoamiResponse
	if err := c.getJSON(ctx, c.base+"/me", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
