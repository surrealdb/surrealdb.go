package memory

import (
	"context"
	"strings"
	"testing"
)

const uncertaintyRow = `{"id":"u1","about":"hq","reason":"two sources disagree",` +
	`"entity":"entity:company/acme","key":"hq","labels":["tier=gold"],` +
	`"resolvable":true,"resolved":false,"scope":[["team/acme"]],"createdAt":"now"}`

func TestUncertaintyListQueryParams(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"unknowns":[`+uncertaintyRow+`],"page":{"hasMore":false}}`, &cs)
	defer srv.Close()

	resolved := false
	c := newTestClient(t, srv)
	page, err := c.Uncertainty().List(context.Background(), UncertaintyListOptions{
		Entity:   "company/acme",
		Resolved: &resolved,
		Limit:    20,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if cs.path != "/api/v1/ctx-1/uncertainty" {
		t.Errorf("path = %s", cs.path)
	}
	assertQuery(t, cs.query, map[string]string{"entity": "company/acme", "resolved": "false", "limit": "20"})
	if len(page.Unknowns) != 1 || page.Unknowns[0].Key != "hq" || !page.Unknowns[0].Resolvable {
		t.Errorf("unknowns = %+v", page.Unknowns)
	}
}

func TestUncertaintyResolvedFilterIsTristate(t *testing.T) {
	// A nil Resolved must not send resolved=false: "both" and "unsettled" are
	// different questions.
	var cs capture
	srv := captureServer(t, `{"unknowns":[],"page":{"hasMore":false}}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, err := c.Uncertainty().List(context.Background(), UncertaintyListOptions{}); err != nil {
		t.Fatalf("List: %v", err)
	}
	assertQuery(t, cs.query, map[string]string{})

	resolved := true
	if _, err := c.Uncertainty().List(context.Background(), UncertaintyListOptions{Resolved: &resolved}); err != nil {
		t.Fatalf("List: %v", err)
	}
	assertQuery(t, cs.query, map[string]string{"resolved": "true"})
}

func TestUncertaintyResolveBodyAndUnwrap(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"uncertainty":`+strings.Replace(uncertaintyRow, `"resolved":false`, `"resolved":true,"resolvedAt":"then"`, 1)+`}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.Uncertainty().Resolve(context.Background(), "u1", "Leeds", WithResolveNote("confirmed on the call"))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cs.path != "/api/v1/ctx-1/uncertainty/u1/resolve" {
		t.Errorf("path = %s", cs.path)
	}
	if cs.body["acceptedValue"] != "Leeds" || cs.body["note"] != "confirmed on the call" {
		t.Errorf("body = %+v", cs.body)
	}
	if !got.Resolved || got.ResolvedAt != "then" {
		t.Errorf("resolved flag = %+v", got)
	}
}

func TestUncertaintyResolveOmitsAbsentNote(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"uncertainty":`+uncertaintyRow+`}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, err := c.Uncertainty().Resolve(context.Background(), "u1", "Leeds"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if _, present := cs.body["note"]; present {
		t.Errorf("note should be omitted when unset, body = %+v", cs.body)
	}
}

func TestUncertaintyCountUsesTotalSize(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"unknowns":[`+uncertaintyRow+`],"page":{"hasMore":true,"nextCursor":"c1","totalSize":42}}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	n, err := c.Uncertainty().Count(context.Background(), UncertaintyListOptions{Entity: "company/acme"})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 42 {
		t.Errorf("count = %d, want 42", n)
	}
	// One row, count on, and any inherited cursor dropped — a count must not
	// resume someone else's walk.
	assertQuery(t, cs.query, map[string]string{"entity": "company/acme", "limit": "1", "count": "true"})
}

func TestUncertaintyCountFallsBackToRowsWhenTotalAbsent(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"unknowns":[`+uncertaintyRow+`],"page":{"hasMore":false}}`, &cs)
	defer srv.Close()

	n, err := newTestClient(t, srv).Uncertainty().Count(context.Background(), UncertaintyListOptions{})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 1 {
		t.Errorf("count = %d, want the rows actually returned", n)
	}
}
