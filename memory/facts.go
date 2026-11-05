package memory

import (
	"context"
	"iter"
	"net/url"
)

// Facts is the fact sub-client returned by [Client.Facts]. It walks the three
// collections a context's knowledge is stored in — attributes, relations and
// actions — as flat, filterable listings.
//
// These are the collections the bounded sections of [Client.Lookup] and
// [Entities.Get] are prefixes of: when a section comes back truncated, its
// matching listing here is how you read the rest.
//
// Entity filters take the plain <type>/<name> form ("person/alice"). The refs
// carried on the returned rows are navigable and so are prefixed
// ("entity:person/alice").
type Facts struct {
	client *Client
}

// AttributeListOptions filters and paginates a [Facts.Attributes] call.
type AttributeListOptions struct {
	// Entity restricts the listing to one subject, as <type>/<name>.
	Entity string
	// Key restricts the listing to one attribute key.
	Key string

	// Limit, Cursor and Count are the cursor pagination inputs; see
	// [PageOptions], whose fields these mirror.
	Limit  int
	Cursor string
	Count  bool
}

func (o AttributeListOptions) values() url.Values {
	q := url.Values{}
	if o.Entity != "" {
		q.Set("entity", o.Entity)
	}
	if o.Key != "" {
		q.Set("key", o.Key)
	}
	PageOptions{Limit: o.Limit, Cursor: o.Cursor, Count: o.Count}.apply(q)
	return q
}

// AttributeListResponse is a page of attributes from [Facts.Attributes].
type AttributeListResponse struct {
	Attributes []AttributeDetail `json:"attributes"`
	Page       PageMeta          `json:"page"`
}

// Attributes returns one page of the context's attributes, newest first.
func (f *Facts) Attributes(ctx context.Context, opts AttributeListOptions) (*AttributeListResponse, error) {
	var out AttributeListResponse
	if err := f.client.getJSON(ctx, f.client.base+"/attributes", opts.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AllAttributes walks every page of the attribute listing.
func (f *Facts) AllAttributes(ctx context.Context, opts AttributeListOptions) iter.Seq2[AttributeDetail, error] {
	opts.Count = false
	return walkPages(ctx, opts.Cursor, func(ctx context.Context, cursor string) ([]AttributeDetail, PageMeta, error) {
		opts.Cursor = cursor
		page, err := f.Attributes(ctx, opts)
		if err != nil {
			return nil, PageMeta{}, err
		}
		return page.Attributes, page.Page, nil
	})
}

// RelationListOptions filters and paginates a [Facts.Relations] call.
type RelationListOptions struct {
	// Src restricts the listing to edges leaving one entity, as <type>/<name>.
	Src string
	// Dst restricts the listing to edges arriving at one entity.
	Dst string
	// Label restricts the listing to one relation label.
	Label string

	// Limit, Cursor and Count are the cursor pagination inputs; see
	// [PageOptions].
	Limit  int
	Cursor string
	Count  bool
}

func (o RelationListOptions) values() url.Values {
	q := url.Values{}
	if o.Src != "" {
		q.Set("src", o.Src)
	}
	if o.Dst != "" {
		q.Set("dst", o.Dst)
	}
	if o.Label != "" {
		q.Set("label", o.Label)
	}
	PageOptions{Limit: o.Limit, Cursor: o.Cursor, Count: o.Count}.apply(q)
	return q
}

// RelationListResponse is a page of relations from [Facts.Relations].
type RelationListResponse struct {
	Relations []RelationDetail `json:"relations"`
	Page      PageMeta         `json:"page"`
}

// Relations returns one page of the context's relation edges.
func (f *Facts) Relations(ctx context.Context, opts RelationListOptions) (*RelationListResponse, error) {
	var out RelationListResponse
	if err := f.client.getJSON(ctx, f.client.base+"/relations", opts.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AllRelations walks every page of the relation listing.
func (f *Facts) AllRelations(ctx context.Context, opts RelationListOptions) iter.Seq2[RelationDetail, error] {
	opts.Count = false
	return walkPages(ctx, opts.Cursor, func(ctx context.Context, cursor string) ([]RelationDetail, PageMeta, error) {
		opts.Cursor = cursor
		page, err := f.Relations(ctx, opts)
		if err != nil {
			return nil, PageMeta{}, err
		}
		return page.Relations, page.Page, nil
	})
}

// AllEdgesOf walks every relation touching one entity, in either direction:
// the outbound edges first, then the inbound ones.
//
// Src and Dst are separate filters, so one of them alone reproduces only half
// the neighbourhood. This is the walk that matches a truncated `relations`
// section from [Client.Lookup] or [Entities.Get], both of which report edges
// in both directions.
//
// Any Src or Dst already on opts is replaced; every other filter is kept.
func (f *Facts) AllEdgesOf(ctx context.Context, entity string, opts RelationListOptions) iter.Seq2[RelationDetail, error] {
	return func(yield func(RelationDetail, error) bool) {
		for _, direction := range []func(RelationListOptions) RelationListOptions{
			func(o RelationListOptions) RelationListOptions { o.Src, o.Dst = entity, ""; return o },
			func(o RelationListOptions) RelationListOptions { o.Src, o.Dst = "", entity; return o },
		} {
			for row, err := range f.AllRelations(ctx, direction(opts)) {
				if !yield(row, err) {
					return
				}
				if err != nil {
					return
				}
			}
		}
	}
}

// ActionListOptions filters and paginates a [Facts.Actions] call.
type ActionListOptions struct {
	// Actor restricts the listing to one acting entity, as <type>/<name>.
	Actor string
	// Verb restricts the listing to one action verb.
	Verb string
	// Since and Until bound occurredAt (RFC 3339 instants), inclusive.
	Since string
	Until string

	// Limit, Cursor and Count are the cursor pagination inputs; see
	// [PageOptions].
	Limit  int
	Cursor string
	Count  bool
}

func (o ActionListOptions) values() url.Values {
	q := url.Values{}
	if o.Actor != "" {
		q.Set("actor", o.Actor)
	}
	if o.Verb != "" {
		q.Set("verb", o.Verb)
	}
	if o.Since != "" {
		q.Set("since", o.Since)
	}
	if o.Until != "" {
		q.Set("until", o.Until)
	}
	PageOptions{Limit: o.Limit, Cursor: o.Cursor, Count: o.Count}.apply(q)
	return q
}

// ActionListResponse is a page of actions from [Facts.Actions].
type ActionListResponse struct {
	Actions []ActionDetail `json:"actions"`
	Page    PageMeta       `json:"page"`
}

// Actions returns one page of the context's recorded events.
func (f *Facts) Actions(ctx context.Context, opts ActionListOptions) (*ActionListResponse, error) {
	var out ActionListResponse
	if err := f.client.getJSON(ctx, f.client.base+"/actions", opts.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AllActions walks every page of the action listing.
func (f *Facts) AllActions(ctx context.Context, opts ActionListOptions) iter.Seq2[ActionDetail, error] {
	opts.Count = false
	return walkPages(ctx, opts.Cursor, func(ctx context.Context, cursor string) ([]ActionDetail, PageMeta, error) {
		opts.Cursor = cursor
		page, err := f.Actions(ctx, opts)
		if err != nil {
			return nil, PageMeta{}, err
		}
		return page.Actions, page.Page, nil
	})
}
