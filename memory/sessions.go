package memory

import (
	"context"
	"iter"
	"net/http"
	"net/url"
)

// Sessions is the session sub-client returned by [Client.Sessions].
type Sessions struct {
	client *Client
}

// CreateSessionRequest is the input to [Sessions.Create].
type CreateSessionRequest struct {
	Scopes ScopeSets `json:"scopes,omitempty"`
	// Metadata is a free-form JSON object attached to the session. Encode it
	// yourself (for example with json.Marshal) and pass the result here.
	Metadata rawObject `json:"metadata,omitempty"`
}

// Session is a conversational session in the context.
type Session struct {
	ID        string    `json:"id"`
	Scopes    ScopeSets `json:"scopes"`
	CreatedAt string    `json:"createdAt"`
}

// Create opens a new session, optionally scoped and carrying metadata.
func (s *Sessions) Create(ctx context.Context, req CreateSessionRequest) (*Session, error) {
	var out Session
	if err := s.client.doJSON(ctx, http.MethodPost, s.client.base+"/sessions", req, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes a session and its turns.
func (s *Sessions) Delete(ctx context.Context, sessionID string) error {
	path := s.client.base + "/sessions/" + url.PathEscape(sessionID)
	return s.client.doJSON(ctx, http.MethodDelete, path, nil, nil, false)
}

// SessionContextResponse is the result of [Sessions.Context].
type SessionContextResponse struct {
	Context string `json:"context"`
}

// Context returns a composed context string scoped to a single session.
func (s *Sessions) Context(ctx context.Context, sessionID, query string) (*SessionContextResponse, error) {
	path := s.client.base + "/sessions/" + url.PathEscape(sessionID) + "/context"
	req := struct {
		Query string `json:"query"`
	}{Query: query}
	var out SessionContextResponse
	if err := s.client.doJSON(ctx, http.MethodPost, path, req, &out, true); err != nil {
		return nil, err
	}
	return &out, nil
}

// Turn is a single recorded turn within a session.
type Turn struct {
	ID        string   `json:"id"`
	Session   string   `json:"session"`
	Seq       int      `json:"seq"`
	Role      TurnRole `json:"role"`
	Content   string   `json:"content"`
	CreatedAt string   `json:"createdAt"`
}

// TurnListResponse is a page of turns from [Sessions.Turns].
type TurnListResponse struct {
	Turns []Turn   `json:"turns"`
	Page  PageMeta `json:"page"`
}

// Turns returns one page of the turns recorded in a session, oldest first.
func (s *Sessions) Turns(ctx context.Context, sessionID string, opts PageOptions) (*TurnListResponse, error) {
	path := s.client.base + "/sessions/" + url.PathEscape(sessionID) + "/turns"
	q := url.Values{}
	opts.apply(q)
	var out TurnListResponse
	if err := s.client.getJSON(ctx, path, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AllTurns walks every page of a session's turns, oldest first.
func (s *Sessions) AllTurns(ctx context.Context, sessionID string, opts PageOptions) iter.Seq2[Turn, error] {
	opts.Count = false
	return walkPages(ctx, opts.Cursor, func(ctx context.Context, cursor string) ([]Turn, PageMeta, error) {
		opts.Cursor = cursor
		page, err := s.Turns(ctx, sessionID, opts)
		if err != nil {
			return nil, PageMeta{}, err
		}
		return page.Turns, page.Page, nil
	})
}
