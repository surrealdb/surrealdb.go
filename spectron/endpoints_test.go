package spectron

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	v, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatalf("parse query %q: %v", raw, err)
	}
	return v
}

// captureServer records the method, path, raw query, and decoded JSON body of
// the request it receives, then replies with the supplied JSON.
type capture struct {
	method string
	path   string
	query  string
	body   map[string]any
}

func captureServer(t *testing.T, reply string, cs *capture) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cs.method = r.Method
		cs.path = r.URL.Path
		cs.query = r.URL.RawQuery
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&cs.body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, reply)
	}))
}

func TestScopeMarshalNormalizes(t *testing.T) {
	// Order-preserving de-dup, dropping empties (surrealdb.py #264 / ScopeSet).
	b, err := json.Marshal(Scope{"team/acme", "", "org/acme", "team/acme"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != `["team/acme","org/acme"]` {
		t.Errorf("normalized scope wire = %s", b)
	}
	// Empty scope marshals to an empty array (the field is omitted upstream via
	// omitempty when the slice is empty).
	if b, _ := json.Marshal(Scope{}); string(b) != `[]` {
		t.Errorf("empty scope wire = %s", b)
	}
}

func TestScopeDedupOnWire(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"mode":"full","sessionId":"s"}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, err := c.Remember(context.Background(), RememberRequest{
		Text:  "x",
		Scope: Scope{"team/acme", "", "team/acme", "org/acme"},
	}); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	raw, ok := cs.body["scope"].([]any)
	if !ok {
		t.Fatalf("scope wire = %#v", cs.body["scope"])
	}
	got := make([]string, len(raw))
	for i, v := range raw {
		got[i], _ = v.(string)
	}
	if len(got) != 2 || got[0] != "team/acme" || got[1] != "org/acme" {
		t.Errorf("deduped scope = %v", got)
	}
}

func TestRememberManyWireIsSnakeCase(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"sessionId":"s","turnIds":["t1"],"extractions":[]}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	resp, err := c.RememberMany(context.Background(), RememberManyRequest{
		Messages:  []BatchMessage{{Role: RoleUser, Content: "hi"}},
		SessionID: "s",
		Extract:   ExtractWholeConversation,
		Scope:     Scope{scopeAcme},
	})
	if err != nil {
		t.Fatalf("RememberMany: %v", err)
	}
	if cs.path != "/api/v1/ctx-1/facts/batch" {
		t.Errorf("path = %q", cs.path)
	}
	if cs.body["session_id"] != "s" {
		t.Errorf("expected snake_case session_id, body = %#v", cs.body)
	}
	if cs.body["extract"] != "whole_conversation" {
		t.Errorf("extract = %v", cs.body["extract"])
	}
	msgs, ok := cs.body["messages"].([]any)
	if !ok || len(msgs) != 1 {
		t.Fatalf("messages = %#v", cs.body["messages"])
	}
	if resp.SessionID != "s" || len(resp.TurnIDs) != 1 {
		t.Errorf("resp = %+v", resp)
	}
}

func TestRecallWireIsCamelCase(t *testing.T) {
	var cs capture
	reply := `{"classificationKind":"hybrid","tier":"hybrid","queryMs":3,` +
		`"seedEntities":["acme"],"hits":[{"id":"x","score":0.5,"source":"attribute","text":"t"}],` +
		`"trace":{"traceId":"tr","resolutionTier":"hybrid","tierReason":"r","retrievedCount":1,"latencyMs":2,"topScores":[0.5]}}`
	srv := captureServer(t, reply, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	resp, err := c.Recall(context.Background(), RecallRequest{
		Query:  "q",
		Mode:   MemoryModeHybrid,
		Lens:   []string{scopeAcme},
		Labels: []string{"tier=gold"},
	})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if cs.body["mode"] != "hybrid" {
		t.Errorf("mode wire = %v", cs.body["mode"])
	}
	lens, ok := cs.body["lens"].([]any)
	if !ok || len(lens) != 1 || lens[0] != scopeAcme {
		t.Errorf("lens wire = %#v", cs.body["lens"])
	}
	if _, snake := cs.body["session_id"]; snake {
		t.Errorf("query body must stay camelCase, found session_id")
	}
	if resp.ClassificationKind != QueryHybrid || resp.Tier != TierHybrid {
		t.Errorf("typed enums = %q / %q", resp.ClassificationKind, resp.Tier)
	}
	if len(resp.Hits) != 1 || resp.Hits[0].Source != ResultAttribute {
		t.Errorf("hit source = %v", resp.Hits)
	}
	if resp.Trace.TraceID != "tr" || len(resp.Trace.TopScores) != 1 {
		t.Errorf("trace = %+v", resp.Trace)
	}
}

func TestDocumentsListQueryParams(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"documents":[{"id":"d1","status":"ready"}],"page":1,"pageSize":20,"total":1}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	page, err := c.Documents().List(context.Background(), ListDocumentsOptions{
		Status:   DocReady,
		MimeType: "application/pdf",
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if cs.method != http.MethodGet || cs.path != docsPath {
		t.Errorf("method/path = %s %s", cs.method, cs.path)
	}
	q := mustQuery(t, cs.query)
	if q.Get("status") != "ready" || q.Get("mime_type") != "application/pdf" || q.Get("page_size") != "20" {
		t.Errorf("query = %q", cs.query)
	}
	if len(page.Documents) != 1 || page.Documents[0].Status != DocReady {
		t.Errorf("page = %+v", page)
	}
}

func TestDocumentsQueryDecodesHit(t *testing.T) {
	var cs capture
	reply := `{"queryMs":4,"results":[{"score":0.8,` +
		`"chunk":{"id":"c1","document":"d1","position":0,"charStart":0,"charEnd":3,"text":"abc"},` +
		`"document":{"id":"d1","title":"T","source":"s"},` +
		`"graphEvidence":[{"edgeKind":"document_link","neighbourLabel":"n","weight":0.5}]}]}`
	srv := captureServer(t, reply, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	resp, err := c.Documents().Query(context.Background(), DocumentQueryRequest{
		Query: "q",
		Mode:  ModeHybrid,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if cs.path != "/api/v1/ctx-1/documents/query" || cs.body["mode"] != "hybrid" {
		t.Errorf("path/mode = %s / %v", cs.path, cs.body["mode"])
	}
	if len(resp.Results) != 1 {
		t.Fatalf("results = %+v", resp.Results)
	}
	hit := resp.Results[0]
	if hit.Chunk.ID != "c1" || hit.Document.Title != "T" {
		t.Errorf("hit = %+v", hit)
	}
	if len(hit.GraphEvidence) != 1 || hit.GraphEvidence[0].EdgeKind != EdgeDocumentLink {
		t.Errorf("graph evidence = %+v", hit.GraphEvidence)
	}
}

func TestSessionsCreateScopeWire(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"id":"sess1","scope":["team/acme"],"createdAt":"now"}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	sess, err := c.Sessions().Create(context.Background(), CreateSessionRequest{
		Scope:    Scope{scopeAcme},
		Metadata: json.RawMessage(`{"source":"test"}`),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cs.path != "/api/v1/ctx-1/sessions" {
		t.Errorf("path = %q", cs.path)
	}
	scope, ok := cs.body["scope"].([]any)
	if !ok || len(scope) != 1 || scope[0] != scopeAcme {
		t.Errorf("scope wire = %#v", cs.body["scope"])
	}
	if sess.ID != "sess1" || len(sess.Scope) != 1 {
		t.Errorf("session = %+v", sess)
	}
}

func TestScopesListBareArray(t *testing.T) {
	var cs capture
	srv := captureServer(t, `[{"path":"team/acme","createdAt":"now"}]`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	nodes, err := c.Scopes().List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if cs.method != http.MethodGet || cs.path != "/api/v1/ctx-1/scopes" {
		t.Errorf("method/path = %s %s", cs.method, cs.path)
	}
	if len(nodes) != 1 || nodes[0].Path != scopeAcme {
		t.Errorf("nodes = %+v", nodes)
	}
}

func TestPrincipalsGrantBody(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"id":"p1","kind":"user","displayName":"P","grants":{"team/acme":["read"]}}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	p, err := c.Principals().Grant(context.Background(), "p1", GrantRequest{
		Path:  scopeAcme,
		Verbs: []string{"read", "write"},
	})
	if err != nil {
		t.Fatalf("Grant: %v", err)
	}
	if cs.method != http.MethodPost || cs.path != "/api/v1/ctx-1/principals/p1/grants" {
		t.Errorf("method/path = %s %s", cs.method, cs.path)
	}
	if cs.body["path"] != scopeAcme {
		t.Errorf("path body = %v", cs.body["path"])
	}
	if p.ID != "p1" || len(p.Grants[scopeAcme]) != 1 {
		t.Errorf("principal = %+v", p)
	}
}

func TestHealthHitsNonContextPath(t *testing.T) {
	var cs capture
	srv := captureServer(t, ``, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.Health(context.Background()); err != nil {
		t.Fatalf("Health: %v", err)
	}
	if cs.path != "/api/v1/health" {
		t.Errorf("path = %q (should not be context-scoped)", cs.path)
	}
}

func TestDocumentDeleteNoContent(t *testing.T) {
	var cs capture
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cs.method = r.Method
		cs.path = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if err := c.Documents().Delete(context.Background(), "d1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if cs.method != http.MethodDelete || cs.path != "/api/v1/ctx-1/documents/d1" {
		t.Errorf("method/path = %s %s", cs.method, cs.path)
	}
}

func TestTracesStatsDecodesNested(t *testing.T) {
	var cs capture
	reply := `{"windowHours":24,"totalQueries":10,"avgLatencyMs":5.5,"cacheHits":3,"cacheHitRate":0.3,` +
		`"responseTracesTotal":10,"responseTracesCached":3,` +
		`"tierCounts":{"direct":1,"hybrid":2,"fullContext":3},` +
		`"sourceKindDistribution":[{"kind":"chunk","count":4}],` +
		`"retrieval":{"traces":10,"avgCandidateSet":2.5,"maxCandidateSet":5},` +
		`"supersession":{"supersessionEvents":1,"entitiesChurned":1,"churnPerEntity":1.0},` +
		`"contradiction":{"contradictions":0,"reconciliations":0,"contradictionRate":0.0}}`
	srv := captureServer(t, reply, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	stats, err := c.Traces().Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if cs.path != "/api/v1/ctx-1/traces/stats" {
		t.Errorf("path = %q", cs.path)
	}
	if stats.TierCounts.FullContext != 3 || len(stats.SourceKindDist) != 1 {
		t.Errorf("stats = %+v", stats)
	}
}

// TestEnumJSONRoundTrip checks that a typed enum field marshals to its wire
// string and decodes back to the typed value.
func TestEnumJSONRoundTrip(t *testing.T) {
	type holder struct {
		Infer InferMode      `json:"infer"`
		Cat   MemoryCategory `json:"memory_category"`
	}
	in := holder{Infer: InferTriples, Cat: MemoryIdentity}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != `{"infer":"triples","memory_category":"identity"}` {
		t.Errorf("wire = %s", b)
	}
	var out holder
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Infer != InferTriples || out.Cat != MemoryIdentity {
		t.Errorf("round-trip = %+v", out)
	}
}
