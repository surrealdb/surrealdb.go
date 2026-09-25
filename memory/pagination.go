package memory

import (
	"context"
	"iter"
	"net/url"
	"strconv"
)

// Page-size bounds applied by the service.
const (
	// DefaultPageLimit is the page size the server uses when Limit is zero.
	DefaultPageLimit = 100
	// MaxPageLimit is the largest page the server will return. A larger Limit
	// is clamped server-side rather than rejected.
	MaxPageLimit = 500
)

// PageMeta is the pagination block returned beside a page's rows.
//
// Walk a listing by following NextCursor until it is empty. Do not stop on a
// short page: a listing that is bounded in the database and then filtered for
// visibility — /scopes and /keys, for example — can return fewer rows than
// Limit while further pages remain.
type PageMeta struct {
	// HasMore reports whether a further page exists.
	HasMore bool `json:"hasMore"`
	// NextCursor addresses the next page, and is empty on the last page.
	NextCursor string `json:"nextCursor,omitempty"`
	// TotalSize counts every row matching the filters, and is set only when
	// the request asked for it with Count. It counts the whole listing, not
	// the rows remaining after the cursor.
	TotalSize *int64 `json:"totalSize,omitempty"`
}

// PageOptions are the pagination inputs shared by most listings.
//
// Paging is keyset rather than offset: a cursor names the last returned row's
// position in the listing's sort order, so every page costs the same as the
// first and the walk stays stable under concurrent writes. A cursor carries a
// fingerprint of the request's filters, so reusing one under changed filters
// is rejected rather than silently resuming inside a different result set.
type PageOptions struct {
	// Limit caps the rows per page. Zero means DefaultPageLimit; anything
	// above MaxPageLimit is clamped by the server.
	Limit int
	// Cursor resumes a walk from a previous page's [PageMeta.NextCursor].
	Cursor string
	// Count asks the server to populate [PageMeta.TotalSize]. It costs a full
	// count of the filtered set — the unbounded read that pagination exists to
	// avoid — so leave it off for ordinary page fetches.
	Count bool
}

func (o PageOptions) apply(q url.Values) {
	if o.Limit > 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.Cursor != "" {
		q.Set("cursor", o.Cursor)
	}
	if o.Count {
		q.Set("count", "true")
	}
}

// CursorOptions is [PageOptions] without Count, for the listings that do not
// offer a total: /scopes and /audit.
type CursorOptions struct {
	// Limit caps the rows per page. Zero means DefaultPageLimit.
	Limit int
	// Cursor resumes a walk from a previous page's [PageMeta.NextCursor].
	Cursor string
}

func (o CursorOptions) apply(q url.Values) {
	if o.Limit > 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.Cursor != "" {
		q.Set("cursor", o.Cursor)
	}
}

// fetchPage retrieves one page of a listing: the rows, and the pagination
// block that says where the next page starts.
type fetchPage[T any] func(ctx context.Context, cursor string) ([]T, PageMeta, error)

// walkPages yields every row of a listing, following the cursor from page to
// page. The walk ends when the server stops handing back a cursor; it does not
// end on a short page.
//
// The first error ends the iteration, so a caller that breaks early simply
// stops fetching. Walkers clear Count before calling: a walk visits every row,
// so asking each page for a total is pure cost.
func walkPages[T any](ctx context.Context, seed string, fetch fetchPage[T]) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T
		seen := make(map[string]struct{})
		cursor := seed
		for {
			if err := ctx.Err(); err != nil {
				yield(zero, err)
				return
			}
			rows, page, err := fetch(ctx, cursor)
			if err != nil {
				yield(zero, err)
				return
			}
			for _, row := range rows {
				if !yield(row, nil) {
					return
				}
			}
			next := page.NextCursor
			if next == "" {
				return
			}
			// A cursor that repeats means the walk cannot advance. Looping
			// forever, or silently stopping on a page we have already seen,
			// would both be worse than saying so.
			if _, dup := seen[next]; dup {
				yield(zero, &APIError{
					Message: "pagination cursor repeated; the page walk cannot advance",
				})
				return
			}
			seen[next] = struct{}{}
			cursor = next
		}
	}
}

// Collect drains an iterator into a slice, stopping at the first error and
// returning the rows gathered up to that point alongside it.
//
// It is the eager counterpart to the All* walkers:
//
//	scopes, err := memory.Collect(client.Scopes().All(ctx, memory.CursorOptions{}))
//
// Prefer ranging over the iterator directly when the listing may be large or
// when you can stop early.
func Collect[T any](seq iter.Seq2[T, error]) ([]T, error) {
	var out []T
	for row, err := range seq {
		if err != nil {
			return out, err
		}
		out = append(out, row)
	}
	return out, nil
}

// OffsetOptions selects the deprecated offset pagination mode, still accepted
// on the three document listings (/documents, /documents/{id}/chunks and
// /documents/keywords) and nowhere else.
//
// Prefer [PageOptions]. Offset paging costs more on every page after the first
// and shifts under concurrent writes, so a walk can miss or repeat rows.
type OffsetOptions struct {
	// Page is the 1-based page number.
	Page int
	// PageSize caps the rows per page.
	PageSize int
}

func (o OffsetOptions) apply(q url.Values) {
	if o.Page > 0 {
		q.Set("page", strconv.Itoa(o.Page))
	}
	if o.PageSize > 0 {
		q.Set("pageSize", strconv.Itoa(o.PageSize))
	}
}

func (o OffsetOptions) set() bool { return o.Page > 0 || o.PageSize > 0 }

func (o PageOptions) set() bool { return o.Limit > 0 || o.Cursor != "" || o.Count }

// checkPageMode rejects mixing cursor and offset pagination before the request
// leaves the process. The server answers the combination with a 400, so
// catching it here just turns a confusing round-trip into a clear error.
func checkPageMode(page PageOptions, offset OffsetOptions) error {
	if page.set() && offset.set() {
		return &APIError{
			Message: "cannot mix cursor pagination (Limit/Cursor/Count) with the " +
				"deprecated offset mode (Page/PageSize); pick one",
		}
	}
	return nil
}
