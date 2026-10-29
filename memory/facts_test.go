package memory

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestFactsAttributesQueryParams(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"attributes":[{"id":"a1","entity":"entity:person/alice","key":"role","value":"CTO"}],"page":{"hasMore":false}}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	page, err := c.Facts().Attributes(context.Background(), AttributeListOptions{
		Entity: "person/alice",
		Key:    "role",
		Limit:  25,
	})
	if err != nil {
		t.Fatalf("Attributes: %v", err)
	}
	if cs.path != "/api/v1/ctx-1/attributes" {
		t.Errorf("path = %s", cs.path)
	}
	// Filters take the plain form; the refs that come back are prefixed.
	assertQuery(t, cs.query, map[string]string{"entity": "person/alice", "key": "role", "limit": "25"})
	if len(page.Attributes) != 1 || page.Attributes[0].Entity != "entity:person/alice" {
		t.Errorf("attributes = %+v", page.Attributes)
	}
}

func TestFactsRelationsQueryParams(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"relations":[],"page":{"hasMore":false}}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Facts().Relations(context.Background(), RelationListOptions{
		Src:   "person/alice",
		Dst:   "company/acme",
		Label: "works_at",
	})
	if err != nil {
		t.Fatalf("Relations: %v", err)
	}
	assertQuery(t, cs.query, map[string]string{"src": "person/alice", "dst": "company/acme", "label": "works_at"})
}

func TestFactsActionsQueryParams(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"actions":[{"id":"ac1","actor":"entity:person/alice","verb":"shipped","summary":"s","confidence":0.9,"memoryCategory":"knowledge","createdAt":"now"}],"page":{"hasMore":false}}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	page, err := c.Facts().Actions(context.Background(), ActionListOptions{
		Actor: "person/alice",
		Verb:  "shipped",
		Since: "2026-01-01T00:00:00Z",
		Until: "2026-12-31T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("Actions: %v", err)
	}
	assertQuery(t, cs.query, map[string]string{
		"actor": "person/alice", "verb": "shipped",
		"since": "2026-01-01T00:00:00Z", "until": "2026-12-31T00:00:00Z",
	})
	if len(page.Actions) != 1 || page.Actions[0].Verb != "shipped" {
		t.Errorf("actions = %+v", page.Actions)
	}
}

func TestAllEdgesOfWalksBothDirections(t *testing.T) {
	// src and dst are separate filters, so one alone reproduces only half the
	// neighbourhood. Outbound edges come first.
	var queries []url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		queries = append(queries, q)
		id := "out"
		if q.Get("dst") != "" {
			id = "in"
		}
		fmt.Fprintf(w, `{"relations":[{"id":%q,"subject":"s","label":"l","object":"o"}],"page":{"hasMore":false}}`, id)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := Collect(c.Facts().AllEdgesOf(context.Background(), "person/alice", RelationListOptions{Label: "works_at"}))
	if err != nil {
		t.Fatalf("AllEdgesOf: %v", err)
	}
	if len(got) != 2 || got[0].ID != "out" || got[1].ID != "in" {
		t.Fatalf("rows = %+v, want outbound then inbound", got)
	}
	if len(queries) != 2 {
		t.Fatalf("made %d requests, want 2", len(queries))
	}
	if queries[0].Get("src") != "person/alice" || queries[0].Get("dst") != "" {
		t.Errorf("first leg = %v, want src only", queries[0])
	}
	if queries[1].Get("dst") != "person/alice" || queries[1].Get("src") != "" {
		t.Errorf("second leg = %v, want dst only", queries[1])
	}
	// Other filters ride along on both legs.
	for i, q := range queries {
		if q.Get("label") != "works_at" {
			t.Errorf("leg %d dropped the label filter: %v", i, q)
		}
	}
}

func TestAllEdgesOfReplacesAnyPresetDirection(t *testing.T) {
	var queries []url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query())
		fmt.Fprint(w, `{"relations":[],"page":{"hasMore":false}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	opts := RelationListOptions{Src: "person/bob", Dst: "person/carol"}
	if _, err := Collect(c.Facts().AllEdgesOf(context.Background(), "person/alice", opts)); err != nil {
		t.Fatalf("AllEdgesOf: %v", err)
	}
	if queries[0].Get("src") != "person/alice" || queries[0].Get("dst") != "" {
		t.Errorf("first leg = %v", queries[0])
	}
	if queries[1].Get("dst") != "person/alice" || queries[1].Get("src") != "" {
		t.Errorf("second leg = %v", queries[1])
	}
}
