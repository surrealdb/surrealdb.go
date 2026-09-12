package memory

import (
	"context"
	"iter"
	"net/http"
	"net/url"
	"strconv"
)

// Entities is the entity sub-client returned by [Client.Entities].
type Entities struct {
	client *Client
}

// ListEntitiesOptions filters and paginates an [Entities.List] call.
type ListEntitiesOptions struct {
	// Type restricts the listing to one entity type.
	Type string

	// Limit, Cursor and Count are the cursor pagination inputs; see
	// [PageOptions], whose fields these mirror.
	Limit  int
	Cursor string
	Count  bool
}

func (o ListEntitiesOptions) values() url.Values {
	q := url.Values{}
	if o.Type != "" {
		q.Set("type", o.Type)
	}
	PageOptions{Limit: o.Limit, Cursor: o.Cursor, Count: o.Count}.apply(q)
	return q
}

// EntityListResponse is a page of entities from [Entities.List].
type EntityListResponse struct {
	Entities []EntityDetail `json:"entities"`
	Page     PageMeta       `json:"page"`
}

// List returns one page of the entities in the context.
func (e *Entities) List(ctx context.Context, opts ListEntitiesOptions) (*EntityListResponse, error) {
	var out EntityListResponse
	if err := e.client.getJSON(ctx, e.client.base+"/entities", opts.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// All walks every page of the entity listing.
func (e *Entities) All(ctx context.Context, opts ListEntitiesOptions) iter.Seq2[EntityDetail, error] {
	opts.Count = false
	return walkPages(ctx, opts.Cursor, func(ctx context.Context, cursor string) ([]EntityDetail, PageMeta, error) {
		opts.Cursor = cursor
		page, err := e.List(ctx, opts)
		if err != nil {
			return nil, PageMeta{}, err
		}
		return page.Entities, page.Page, nil
	})
}

// EntityMatch is an entity with the coverage figures a reader needs to decide
// whether it is the one they meant, and whether walking to it is worth it.
type EntityMatch struct {
	Entity EntityDetail `json:"entity"`
	// Score is match quality in [0, 1]. 1.0 is an exact match on the
	// normalised identity name.
	Score float64 `json:"score"`
	// FactCount is attributes + events + outbound relations — the number that
	// says whether walking here will return anything.
	FactCount int `json:"factCount"`
	// Distinguisher is one sentence assembled from the highest-importance
	// fact summaries, so two same-named candidates can be told apart. Absent
	// when the entity has no summarised facts.
	Distinguisher string `json:"distinguisher,omitempty"`
	// LastLearnedAt is the newest known-time across the entity's own facts.
	LastLearnedAt string `json:"lastLearnedAt,omitempty"`
	// MatchedAlias is the alias that matched, when the query named one.
	MatchedAlias string `json:"matchedAlias,omitempty"`
}

// EntityRanking selects how [Entities.Top] orders its result.
type EntityRanking string

// Known values for [EntityRanking].
const (
	// RankByCoverage ranks by fact count. The default.
	RankByCoverage EntityRanking = "coverage"
	// RankByImportance ranks by the entity's importance score.
	RankByImportance EntityRanking = "importance"
	// RankByRecency ranks by how recently the entity was learned about.
	RankByRecency EntityRanking = "recency"
)

// EntitySearchOptions tunes an [Entities.Search] call.
type EntitySearchOptions struct {
	// Type restricts the search to one entity type.
	Type string
	// Limit caps the matches returned. Zero lets the server choose (10).
	Limit int
}

// Search finds entities by name, ranked by match quality.
//
// This is a ranked head, not a collection: it returns the best matches and
// does not page. Raise Limit to see more.
func (e *Entities) Search(ctx context.Context, query string, opts EntitySearchOptions) ([]EntityMatch, error) {
	q := url.Values{}
	q.Set("q", query)
	if opts.Type != "" {
		q.Set("type", opts.Type)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	var out struct {
		Matches []EntityMatch `json:"matches"`
	}
	if err := e.client.getJSON(ctx, e.client.base+"/entities/search", q, &out); err != nil {
		return nil, err
	}
	return out.Matches, nil
}

// TopEntitiesOptions tunes an [Entities.Top] call.
type TopEntitiesOptions struct {
	// By selects the ranking. Empty means [RankByCoverage].
	By EntityRanking
	// Type restricts the result to one entity type.
	Type string
	// Limit caps the entities returned. Zero lets the server choose (10).
	Limit int
}

// Top returns the context's most prominent entities.
//
// Like [Entities.Search] this is a ranked head rather than a collection: it
// does not page. To enumerate every entity, use [Entities.All].
func (e *Entities) Top(ctx context.Context, opts TopEntitiesOptions) ([]EntityMatch, error) {
	q := url.Values{}
	if opts.By != "" {
		q.Set("by", string(opts.By))
	}
	if opts.Type != "" {
		q.Set("type", opts.Type)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	var out struct {
		Entities []EntityMatch `json:"entities"`
	}
	if err := e.client.getJSON(ctx, e.client.base+"/entities/top", q, &out); err != nil {
		return nil, err
	}
	return out.Entities, nil
}

// EntityResponse is the result of [Entities.Get]: the entity row together with
// its attributes and relation edges.
type EntityResponse struct {
	Entity     EntityDetail      `json:"entity"`
	Attributes []AttributeDetail `json:"attributes"`
	Relations  []RelationDetail  `json:"relations"`
	// Truncated says which sections were cut short. Both come back newest
	// first — the order those collections page in — so a truncated section is
	// a genuine prefix of that walk, and [Facts.AllAttributes] or
	// [Facts.AllEdgesOf] reads the rest.
	Truncated EntityTruncation `json:"truncated"`
}

// EntityTruncation reports which of an entity read's two fact sections were
// bounded short.
type EntityTruncation struct {
	Attributes bool `json:"attributes"`
	Relations  bool `json:"relations"`
}

// EntityGetOptions tunes an [Entities.Get] read.
type EntityGetOptions struct {
	// Limit caps the rows in each fact section. Zero lets the server choose
	// (500, which is also the cap).
	Limit int
	// AsOf reads the context as it was known at this instant (RFC 3339).
	AsOf string
	// AtInstant reads the facts valid at this instant, whenever they were
	// learned.
	AtInstant string
	// ValidFrom and ValidUntil bound assertion validity.
	ValidFrom  string
	ValidUntil string
}

func (o EntityGetOptions) values() url.Values {
	q := url.Values{}
	if o.Limit > 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
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

// Get fetches a single entity addressed by its type and name, with its
// attributes and relation edges.
//
// Both fact sections are bounded; check [EntityResponse.Truncated] and walk
// the matching collection when a section was cut.
func (e *Entities) Get(ctx context.Context, entityType, name string, opts EntityGetOptions) (*EntityResponse, error) {
	path := e.client.base + "/entities/" + url.PathEscape(entityType) + "/" + url.PathEscape(name)
	var out EntityResponse
	if err := e.client.getJSON(ctx, path, opts.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Neighbour is one edge of an entity's neighbourhood, together with the entity
// at the far end.
type Neighbour struct {
	// Far is the entity at the other end of the edge.
	Far        EntityMatch `json:"far"`
	Label      string      `json:"label"`
	RelationID string      `json:"relationId"`
	// Outbound is true when the edge reads subject -> far, false when it
	// reads far -> subject.
	Outbound   bool   `json:"outbound"`
	ValidUntil string `json:"validUntil,omitempty"`
}

// NeighbourhoodOptions filters and paginates an [Entities.Neighbours] call.
type NeighbourhoodOptions struct {
	// MinFacts drops neighbours with fewer facts than this. It cannot be
	// combined with Count.
	MinFacts int

	// Limit, Cursor and Count are the cursor pagination inputs; see
	// [PageOptions]. Count returns the subject's visible edge total.
	Limit  int
	Cursor string
	Count  bool
}

func (o NeighbourhoodOptions) values() url.Values {
	q := url.Values{}
	if o.MinFacts > 0 {
		q.Set("minFacts", strconv.Itoa(o.MinFacts))
	}
	PageOptions{Limit: o.Limit, Cursor: o.Cursor, Count: o.Count}.apply(q)
	return q
}

func (o NeighbourhoodOptions) validate() error {
	if o.MinFacts > 0 && o.Count {
		return &APIError{
			Message: "MinFacts cannot be combined with Count: the total counts the " +
				"subject's visible edges, which the filter would contradict",
		}
	}
	return nil
}

// NeighbourhoodResponse is a page of neighbours from [Entities.Neighbours].
type NeighbourhoodResponse struct {
	Neighbours []Neighbour `json:"neighbours"`
	Page       PageMeta    `json:"page"`
}

// Neighbours returns one page of the entities directly connected to this one,
// in either direction.
func (e *Entities) Neighbours(ctx context.Context, entityType, name string, opts NeighbourhoodOptions) (*NeighbourhoodResponse, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	path := e.client.base + "/entities/" + url.PathEscape(entityType) + "/" + url.PathEscape(name) + "/neighbourhood"
	var out NeighbourhoodResponse
	if err := e.client.getJSON(ctx, path, opts.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AllNeighbours walks every page of an entity's neighbourhood.
func (e *Entities) AllNeighbours(ctx context.Context, entityType, name string, opts NeighbourhoodOptions) iter.Seq2[Neighbour, error] {
	opts.Count = false
	return walkPages(ctx, opts.Cursor, func(ctx context.Context, cursor string) ([]Neighbour, PageMeta, error) {
		opts.Cursor = cursor
		page, err := e.Neighbours(ctx, entityType, name, opts)
		if err != nil {
			return nil, PageMeta{}, err
		}
		return page.Neighbours, page.Page, nil
	})
}

// Delete removes an entity addressed by its type and name.
func (e *Entities) Delete(ctx context.Context, entityType, name string) error {
	path := e.client.base + "/entities/" + url.PathEscape(entityType) + "/" + url.PathEscape(name)
	return e.client.doJSON(ctx, http.MethodDelete, path, nil, nil, false)
}

// EntityHistoryAllResponse is a page of an entity's attribute changes from
// [Entities.Changes].
type EntityHistoryAllResponse struct {
	History []AttributeDetail `json:"history"`
	Page    PageMeta          `json:"page"`
}

// Changes returns one page of every attribute change on an entity, newest
// first, across all keys.
//
// For the supersession chain of a single key, use [Entities.History].
func (e *Entities) Changes(ctx context.Context, entityType, name string, opts PageOptions) (*EntityHistoryAllResponse, error) {
	path := e.client.base + "/entities/" + url.PathEscape(entityType) + "/" + url.PathEscape(name) + "/history"
	q := url.Values{}
	opts.apply(q)
	var out EntityHistoryAllResponse
	if err := e.client.getJSON(ctx, path, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AllChanges walks every page of an entity's attribute changes.
func (e *Entities) AllChanges(ctx context.Context, entityType, name string, opts PageOptions) iter.Seq2[AttributeDetail, error] {
	opts.Count = false
	return walkPages(ctx, opts.Cursor, func(ctx context.Context, cursor string) ([]AttributeDetail, PageMeta, error) {
		opts.Cursor = cursor
		page, err := e.Changes(ctx, entityType, name, opts)
		if err != nil {
			return nil, PageMeta{}, err
		}
		return page.History, page.Page, nil
	})
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
