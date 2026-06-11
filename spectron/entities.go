package spectron

import (
	"context"
	"net/http"
	"net/url"
)

// Entities is the entity sub-client returned by [Client.Entities].
type Entities struct {
	client *Client
}

// EntityListResponse is the result of [Entities.List].
type EntityListResponse struct {
	Entities []EntityDetail `json:"entities"`
}

// List returns the entities in the context. When entityType is non-empty the
// listing is filtered to that type.
func (e *Entities) List(ctx context.Context, entityType string) (*EntityListResponse, error) {
	q := url.Values{}
	if entityType != "" {
		q.Set("type", entityType)
	}
	var out EntityListResponse
	if err := e.client.getJSON(ctx, e.client.base+"/entities", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EntityResponse is the result of [Entities.Get]: the entity row together with
// its attributes and relation edges.
type EntityResponse struct {
	Entity     EntityDetail      `json:"entity"`
	Attributes []AttributeDetail `json:"attributes"`
	Relations  []RelationDetail  `json:"relations"`
}

// Get fetches a single entity addressed by its type and name.
func (e *Entities) Get(ctx context.Context, entityType, name string) (*EntityResponse, error) {
	path := e.client.base + "/entities/" + url.PathEscape(entityType) + "/" + url.PathEscape(name)
	var out EntityResponse
	if err := e.client.getJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes an entity addressed by its type and name.
func (e *Entities) Delete(ctx context.Context, entityType, name string) error {
	path := e.client.base + "/entities/" + url.PathEscape(entityType) + "/" + url.PathEscape(name)
	return e.client.doJSON(ctx, http.MethodDelete, path, nil, nil, false)
}

// EntityHistoryResponse is the result of [Entities.History]: the supersession
// chain for one attribute key on the entity, newest first.
type EntityHistoryResponse struct {
	History []AttributeDetail `json:"history"`
}

// History returns the value history for a single attribute key on an entity.
func (e *Entities) History(ctx context.Context, entityType, name, key string) (*EntityHistoryResponse, error) {
	path := e.client.base + "/entities/" + url.PathEscape(entityType) + "/" + url.PathEscape(name) + "/history/" + url.PathEscape(key)
	var out EntityHistoryResponse
	if err := e.client.getJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
