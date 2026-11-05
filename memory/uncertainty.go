package memory

import (
	"context"
	"iter"
	"net/http"
	"net/url"
	"strconv"
)

// Uncertainty is the uncertainty sub-client returned by [Client.Uncertainty].
// It lists the things a context knows it is unsure about, and settles them.
type Uncertainty struct {
	client *Client
}

// UnknownFact is one thing the context is unsure about.
//
// Entity and Key are set only on the two reconciler-raised kinds — a
// contradiction between sources, and a value that changed without a stated
// reason. Flags raised from a turn carry neither.
type UnknownFact struct {
	ID string `json:"id"`
	// About names the thing in question.
	About string `json:"about"`
	// Reason says why it was flagged.
	Reason string `json:"reason"`
	// Entity is the navigable ref of the subject, entity:<type>/<name>, when
	// there is one.
	Entity string `json:"entity,omitempty"`
	// Key is the attribute key in question, when there is one.
	Key    string   `json:"key,omitempty"`
	Labels []string `json:"labels"`
	// Resolvable reports whether [Uncertainty.Resolve] would settle this flag
	// for the calling key. It is evaluated per caller: a flag someone else can
	// settle still reads false here.
	Resolvable bool      `json:"resolvable"`
	Resolved   bool      `json:"resolved"`
	ResolvedAt string    `json:"resolvedAt,omitempty"`
	Scope      ScopeSets `json:"scope"`
	CreatedAt  string    `json:"createdAt"`
}

// UncertaintyListOptions filters and paginates an [Uncertainty.List] call.
type UncertaintyListOptions struct {
	// Entity restricts the listing to one subject, as <type>/<name>.
	Entity string
	// Resolved filters on settled state. Leave nil for both.
	Resolved *bool

	// Limit, Cursor and Count are the cursor pagination inputs; see
	// [PageOptions], whose fields these mirror.
	Limit  int
	Cursor string
	Count  bool
}

func (o UncertaintyListOptions) values() url.Values {
	q := url.Values{}
	if o.Entity != "" {
		q.Set("entity", o.Entity)
	}
	if o.Resolved != nil {
		q.Set("resolved", strconv.FormatBool(*o.Resolved))
	}
	PageOptions{Limit: o.Limit, Cursor: o.Cursor, Count: o.Count}.apply(q)
	return q
}

// UncertaintyListResponse is a page of flags from [Uncertainty.List].
type UncertaintyListResponse struct {
	Unknowns []UnknownFact `json:"unknowns"`
	Page     PageMeta      `json:"page"`
}

// List returns one page of the context's uncertainty flags.
func (u *Uncertainty) List(ctx context.Context, opts UncertaintyListOptions) (*UncertaintyListResponse, error) {
	var out UncertaintyListResponse
	if err := u.client.getJSON(ctx, u.client.base+"/uncertainty", opts.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// All walks every page of the uncertainty listing.
func (u *Uncertainty) All(ctx context.Context, opts UncertaintyListOptions) iter.Seq2[UnknownFact, error] {
	opts.Count = false
	return walkPages(ctx, opts.Cursor, func(ctx context.Context, cursor string) ([]UnknownFact, PageMeta, error) {
		opts.Cursor = cursor
		page, err := u.List(ctx, opts)
		if err != nil {
			return nil, PageMeta{}, err
		}
		return page.Unknowns, page.Page, nil
	})
}

// Count returns how many flags match the filters, without fetching them.
//
// It costs a full count of the filtered set, which is the unbounded read
// pagination exists to avoid — so call it when you want the number, not as a
// prelude to a walk.
func (u *Uncertainty) Count(ctx context.Context, opts UncertaintyListOptions) (int64, error) {
	opts.Limit, opts.Cursor, opts.Count = 1, "", true
	page, err := u.List(ctx, opts)
	if err != nil {
		return 0, err
	}
	if page.Page.TotalSize != nil {
		return *page.Page.TotalSize, nil
	}
	// The server may omit the total; the rows we did get are then the answer.
	return int64(len(page.Unknowns)), nil
}

// ResolveOption tweaks an [Uncertainty.Resolve] call.
type ResolveOption func(*resolveRequest)

type resolveRequest struct {
	AcceptedValue string `json:"acceptedValue"`
	Note          string `json:"note,omitempty"`
}

// WithResolveNote records why the value was chosen. It is persisted as the new
// row's source clause, so the resulting fact can explain itself later.
func WithResolveNote(note string) ResolveOption {
	return func(r *resolveRequest) { r.Note = note }
}

// Resolve settles one uncertainty flag by accepting a value.
//
// The accepted value is written as a fresh assertion at the upsert trust
// prior. A flag whose [UnknownFact.Resolvable] is false for the calling key is
// rejected with a 422.
func (u *Uncertainty) Resolve(ctx context.Context, id, acceptedValue string, opts ...ResolveOption) (*UnknownFact, error) {
	req := resolveRequest{AcceptedValue: acceptedValue}
	for _, opt := range opts {
		opt(&req)
	}
	path := u.client.base + "/uncertainty/" + url.PathEscape(id) + "/resolve"
	var out struct {
		Uncertainty UnknownFact `json:"uncertainty"`
	}
	if err := u.client.doJSON(ctx, http.MethodPost, path, req, &out, false); err != nil {
		return nil, err
	}
	return &out.Uncertainty, nil
}
