package memory

import (
	"context"
	"strings"
	"testing"
)

const entityMatchRow = `{"entity":{"id":"e1","name":"Acme","entityType":"company"},` +
	`"score":0.91,"factCount":12,"distinguisher":"the one in Leeds","matchedAlias":"ACME Ltd"}`

func TestEntitiesSearchUnwrapsMatches(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"matches":[`+entityMatchRow+`]}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.Entities().Search(context.Background(), "acme", EntitySearchOptions{Type: "company", Limit: 5})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if cs.path != "/api/v1/ctx-1/entities/search" {
		t.Errorf("path = %s", cs.path)
	}
	assertQuery(t, cs.query, map[string]string{"q": "acme", "type": "company", "limit": "5"})
	if len(got) != 1 || got[0].MatchedAlias != "ACME Ltd" || got[0].FactCount != 12 {
		t.Errorf("matches = %+v", got)
	}
}

func TestEntitiesTopRanking(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"entities":[`+entityMatchRow+`]}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.Entities().Top(context.Background(), TopEntitiesOptions{By: RankByImportance, Limit: 3})
	if err != nil {
		t.Fatalf("Top: %v", err)
	}
	if cs.path != "/api/v1/ctx-1/entities/top" {
		t.Errorf("path = %s", cs.path)
	}
	assertQuery(t, cs.query, map[string]string{"by": "importance", "limit": "3"})
	if len(got) != 1 || got[0].Entity.Name != "Acme" {
		t.Errorf("entities = %+v", got)
	}
}

func TestEntitiesTopDefaultsToServerRanking(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"entities":[]}`, &cs)
	defer srv.Close()

	if _, err := newTestClient(t, srv).Entities().Top(context.Background(), TopEntitiesOptions{}); err != nil {
		t.Fatalf("Top: %v", err)
	}
	assertQuery(t, cs.query, map[string]string{})
}

func TestEntitiesGetSendsTemporalOptionsAndDecodesTruncation(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"entity":{"id":"e1","name":"Acme","entityType":"company"},`+
		`"attributes":[],"relations":[],"truncated":{"attributes":true,"relations":false}}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.Entities().Get(context.Background(), "company", "acme", EntityGetOptions{
		Limit:     50,
		AsOf:      "2026-01-01T00:00:00Z",
		AtInstant: "2026-06-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	assertQuery(t, cs.query, map[string]string{
		"limit": "50", "asOf": "2026-01-01T00:00:00Z", "atInstant": "2026-06-01T00:00:00Z",
	})
	if !got.Truncated.Attributes || got.Truncated.Relations {
		t.Errorf("truncated = %+v", got.Truncated)
	}
}

func TestEntitiesNeighboursQueryParams(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"neighbours":[{"far":`+entityMatchRow+`,"label":"works_at","relationId":"r1","outbound":true}],`+
		`"page":{"hasMore":false}}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	page, err := c.Entities().Neighbours(context.Background(), "company", "acme", NeighbourhoodOptions{
		MinFacts: 2,
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("Neighbours: %v", err)
	}
	if cs.path != "/api/v1/ctx-1/entities/company/acme/neighbourhood" {
		t.Errorf("path = %s", cs.path)
	}
	assertQuery(t, cs.query, map[string]string{"minFacts": "2", "limit": "10"})
	if len(page.Neighbours) != 1 || !page.Neighbours[0].Outbound || page.Neighbours[0].Far.Entity.Name != "Acme" {
		t.Errorf("neighbours = %+v", page.Neighbours)
	}
}

func TestNeighboursRejectsMinFactsWithCount(t *testing.T) {
	// The total counts the subject's visible edges, which the filter would
	// contradict; the server rejects the pair.
	c := newTestClient(t, captureServer(t, `{}`, &capture{}))
	_, err := c.Entities().Neighbours(context.Background(), "company", "acme",
		NeighbourhoodOptions{MinFacts: 2, Count: true})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "MinFacts cannot be combined with Count") {
		t.Errorf("err = %v", err)
	}
}

func TestEntitiesChangesWalksAllKeys(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"history":[{"id":"a1","entity":"entity:company/acme","key":"hq","value":"Leeds"}],`+
		`"page":{"hasMore":false}}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	page, err := c.Entities().Changes(context.Background(), "company", "acme", PageOptions{Limit: 5})
	if err != nil {
		t.Fatalf("Changes: %v", err)
	}
	// The all-keys listing, distinct from History's single-key chain.
	if cs.path != "/api/v1/ctx-1/entities/company/acme/history" {
		t.Errorf("path = %s", cs.path)
	}
	assertQuery(t, cs.query, map[string]string{"limit": "5"})
	if len(page.History) != 1 || page.History[0].Key != "hq" {
		t.Errorf("history = %+v", page.History)
	}
}

func TestEntitiesHistoryStillTargetsOneKey(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"history":[]}`, &cs)
	defer srv.Close()

	if _, err := newTestClient(t, srv).Entities().History(context.Background(), "company", "acme", "hq"); err != nil {
		t.Fatalf("History: %v", err)
	}
	if cs.path != "/api/v1/ctx-1/entities/company/acme/history/hq" {
		t.Errorf("path = %s", cs.path)
	}
}
