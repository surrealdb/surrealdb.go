package memory

import (
	"context"
	"iter"
	"net/http"
	"net/url"
)

// Scopes is the scope sub-client returned by [Client.Scopes].
type Scopes struct {
	client *Client
}

// ScopeNode is a registered scope path in the context's scope tree.
type ScopeNode struct {
	Path         string `json:"path"`
	CreatedAt    string `json:"createdAt"`
	TombstonedAt string `json:"tombstonedAt,omitempty"`
}

// ScopePage is a page of scope nodes from [Scopes.List].
type ScopePage struct {
	Scopes []ScopeNode `json:"scopes"`
	Page   PageMeta    `json:"page"`
}

// List returns one page of the registered scope nodes for the context.
//
// The listing is bounded in the database and then filtered for the caller's
// visibility, so a page can come back shorter than Limit while further pages
// remain. Follow [PageMeta.NextCursor], or use [Scopes.All].
func (s *Scopes) List(ctx context.Context, opts CursorOptions) (*ScopePage, error) {
	q := url.Values{}
	opts.apply(q)
	var out ScopePage
	if err := s.client.getJSON(ctx, s.client.base+"/scopes", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// All walks every page of the scope listing. opts.Cursor seeds the walk and
// opts.Limit sets the page size.
func (s *Scopes) All(ctx context.Context, opts CursorOptions) iter.Seq2[ScopeNode, error] {
	return walkPages(ctx, opts.Cursor, func(ctx context.Context, cursor string) ([]ScopeNode, PageMeta, error) {
		opts.Cursor = cursor
		page, err := s.List(ctx, opts)
		if err != nil {
			return nil, PageMeta{}, err
		}
		return page.Scopes, page.Page, nil
	})
}

// RegisterScopeRequest is the input to [Scopes.Register].
type RegisterScopeRequest struct {
	Path        string `json:"path"`
	DisplayName string `json:"displayName,omitempty"`
	Description string `json:"description,omitempty"`
}

// Register creates a scope node at the given path.
func (s *Scopes) Register(ctx context.Context, req RegisterScopeRequest) (*ScopeNode, error) {
	var out ScopeNode
	if err := s.client.doJSON(ctx, http.MethodPost, s.client.base+"/scopes", req, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete tombstones the scope node at path.
func (s *Scopes) Delete(ctx context.Context, path string) error {
	q := url.Values{}
	q.Set("path", path)
	return s.client.doJSON(ctx, http.MethodDelete, s.client.base+"/scopes?"+q.Encode(), nil, nil, false)
}

// ForgetScopeResponse is the result of [Scopes.Forget].
type ForgetScopeResponse struct {
	Forgotten int `json:"forgotten"`
}

// Forget removes all rows under a scope path. An empty path forgets the
// caller's default region.
func (s *Scopes) Forget(ctx context.Context, path string) (*ForgetScopeResponse, error) {
	req := struct {
		Path string `json:"path,omitempty"`
	}{Path: path}
	var out ForgetScopeResponse
	if err := s.client.doJSON(ctx, http.MethodPost, s.client.base+"/scopes/forget", req, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

// ScopeGrantRequest is the input to [Scopes.Grants].
type ScopeGrantRequest struct {
	Subject string              `json:"subject,omitempty"`
	Grants  map[string][]string `json:"grants,omitempty"`
}

// Grants is a stub on the server today and always returns a 501 *APIError. It
// is included so the SDK tracks the full spec surface; call it only once the
// endpoint ships.
func (s *Scopes) Grants(ctx context.Context, req ScopeGrantRequest) error {
	return s.client.doJSON(ctx, http.MethodPost, s.client.base+"/scope-grants", req, nil, false)
}
