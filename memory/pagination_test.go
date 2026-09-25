package memory

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// pagedServer serves a scope listing split into the given pages. Each element
// is the rows for one page; the cursor is the index of the next page.
func pagedServer(t *testing.T, pages [][]string, queries *[]url.Values) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if queries != nil {
			*queries = append(*queries, r.URL.Query())
		}
		idx := 0
		if c := r.URL.Query().Get("cursor"); c != "" {
			if _, err := fmt.Sscanf(c, "p%d", &idx); err != nil {
				t.Errorf("unparseable cursor %q", c)
			}
		}
		if idx >= len(pages) {
			t.Errorf("cursor %d past the end", idx)
			idx = len(pages) - 1
		}
		rows := make([]string, 0, len(pages[idx]))
		for _, p := range pages[idx] {
			rows = append(rows, fmt.Sprintf(`{"path":%q,"createdAt":"now"}`, p))
		}
		meta := `"page":{"hasMore":false}`
		if idx+1 < len(pages) {
			meta = fmt.Sprintf(`"page":{"hasMore":true,"nextCursor":"p%d"}`, idx+1)
		}
		fmt.Fprintf(w, `{"scopes":[%s],%s}`, strings.Join(rows, ","), meta)
	}))
}

func TestWalkDoesNotStopOnAShortPage(t *testing.T) {
	// The decisive rule. /scopes is bounded in the database and then filtered
	// for visibility, so a page can be shorter than the limit while further
	// pages remain. Stopping on a short page silently loses rows.
	var queries []url.Values
	srv := pagedServer(t, [][]string{
		{"a", "b", "c"},
		{"d"},      // short, but not the last page
		{"e", "f"}, // still more after the short one
	}, &queries)
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := Collect(c.Scopes().All(context.Background(), CursorOptions{Limit: 3}))
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	var paths []string
	for _, n := range got {
		paths = append(paths, n.Path)
	}
	want := []string{"a", "b", "c", "d", "e", "f"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Errorf("paths = %v, want %v", paths, want)
	}
	if len(queries) != 3 {
		t.Fatalf("fetched %d pages, want 3", len(queries))
	}
	// The limit rides along on every page; the cursor advances.
	for i, q := range queries {
		if q.Get("limit") != "3" {
			t.Errorf("page %d limit = %q", i, q.Get("limit"))
		}
	}
	if queries[0].Get("cursor") != "" || queries[1].Get("cursor") != "p1" || queries[2].Get("cursor") != "p2" {
		t.Errorf("cursors = %q, %q, %q", queries[0].Get("cursor"), queries[1].Get("cursor"), queries[2].Get("cursor"))
	}
}

func TestWalkStopsWhenCursorIsAbsent(t *testing.T) {
	// A full page with no nextCursor is the last page.
	var queries []url.Values
	srv := pagedServer(t, [][]string{{"a", "b"}}, &queries)
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := Collect(c.Scopes().All(context.Background(), CursorOptions{Limit: 2}))
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(got) != 2 || len(queries) != 1 {
		t.Errorf("rows = %d over %d fetches, want 2 over 1", len(got), len(queries))
	}
}

func TestWalkErrorsOnRepeatedCursor(t *testing.T) {
	// A server that keeps handing back the same cursor would loop forever.
	// Saying so beats spinning, and beats silently truncating.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"scopes":[{"path":"a","createdAt":"now"}],"page":{"hasMore":true,"nextCursor":"stuck"}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := Collect(c.Scopes().All(context.Background(), CursorOptions{}))
	if err == nil {
		t.Fatal("expected an error on the repeated cursor")
	}
	if !strings.Contains(err.Error(), "cursor repeated") {
		t.Errorf("err = %v", err)
	}
}

func TestWalkStopsEarlyOnBreak(t *testing.T) {
	var queries []url.Values
	srv := pagedServer(t, [][]string{{"a", "b"}, {"c", "d"}, {"e"}}, &queries)
	defer srv.Close()

	c := newTestClient(t, srv)
	var seen int
	for _, err := range c.Scopes().All(context.Background(), CursorOptions{}) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		seen++
		if seen == 3 {
			break
		}
	}
	if seen != 3 {
		t.Errorf("seen = %d, want 3", seen)
	}
	// Breaking mid-page 2 must not fetch page 3.
	if len(queries) != 2 {
		t.Errorf("fetched %d pages, want 2", len(queries))
	}
}

func TestWalkHonoursContextCancellation(t *testing.T) {
	srv := pagedServer(t, [][]string{{"a"}, {"b"}, {"c"}}, nil)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newTestClient(t, srv)
	var err error
	for _, e := range c.Scopes().All(ctx, CursorOptions{}) {
		if e != nil {
			err = e
			break
		}
		cancel()
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestCollectReturnsRowsGatheredBeforeAnError(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		if hits == 1 {
			fmt.Fprint(w, `{"scopes":[{"path":"a","createdAt":"now"}],"page":{"hasMore":true,"nextCursor":"p1"}}`)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"message":"boom"}`)
	}))
	defer srv.Close()

	// No retries: the 500 should fail the walk immediately rather than
	// spending the backoff schedule on it.
	c, err := New("ctx-1", srv.URL, "sk-test", WithMaxRetries(0))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	got, err := Collect(c.Scopes().All(context.Background(), CursorOptions{}))
	if err == nil {
		t.Fatal("expected an error")
	}
	if len(got) != 1 || got[0].Path != "a" {
		t.Errorf("rows before the error = %+v, want the first page", got)
	}
}

func TestPageOptionsSendCountOnlyWhenAsked(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"principals":[],"page":{"hasMore":false}}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, err := c.Principals().List(context.Background(), PageOptions{Limit: 10}); err != nil {
		t.Fatalf("List: %v", err)
	}
	assertQuery(t, cs.query, map[string]string{"limit": "10"})

	if _, err := c.Principals().List(context.Background(), PageOptions{Limit: 10, Count: true}); err != nil {
		t.Fatalf("List: %v", err)
	}
	assertQuery(t, cs.query, map[string]string{"limit": "10", "count": "true"})
}

func TestTotalSizeDistinguishesAbsentFromZero(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"principals":[],"page":{"hasMore":false}}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	page, err := c.Principals().List(context.Background(), PageOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Page.TotalSize != nil {
		t.Errorf("TotalSize = %v, want nil when the server omits it", *page.Page.TotalSize)
	}
	srv.Close()

	var cs2 capture
	srv2 := captureServer(t, `{"principals":[],"page":{"hasMore":false,"totalSize":0}}`, &cs2)
	defer srv2.Close()
	page, err = newTestClient(t, srv2).Principals().List(context.Background(), PageOptions{Count: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Page.TotalSize == nil || *page.Page.TotalSize != 0 {
		t.Errorf("TotalSize = %v, want a present zero", page.Page.TotalSize)
	}
}

func TestMixingCursorAndOffsetModesIsRejected(t *testing.T) {
	c := newTestClient(t, httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request should never leave the process")
	})))
	_, err := c.Documents().List(context.Background(), ListDocumentsOptions{Cursor: "c1", Page: 2})
	if err == nil {
		t.Fatal("expected an error when mixing pagination modes")
	}
	if !strings.Contains(err.Error(), "cannot mix cursor pagination") {
		t.Errorf("err = %v", err)
	}
}
