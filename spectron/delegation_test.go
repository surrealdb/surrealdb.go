package spectron

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// delegationProbe records the delegation header and path of each request and
// replies with the supplied JSON body.
func delegationProbe(t *testing.T, reply string, gotHeader, gotPath *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotHeader = r.Header.Get("X-Spectron-On-Behalf-Of")
		*gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, reply)
	}))
}

func TestOnBehalfOfSetsHeader(t *testing.T) {
	var gotHeader, gotPath string
	reply := `{"classificationKind":"hybrid","tier":"hybrid","queryMs":1,"seedEntities":[],` +
		`"hits":[],"trace":{"traceId":"t","resolutionTier":"hybrid","tierReason":"r",` +
		`"retrievedCount":0,"latencyMs":1,"topScores":[]}}`
	srv := delegationProbe(t, reply, &gotHeader, &gotPath)
	defer srv.Close()

	c := newTestClient(t, srv)

	// Delegated call carries the header.
	if _, err := c.OnBehalfOf(principalBob).Recall(context.Background(), RecallRequest{Query: "q"}); err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if gotHeader != principalBob {
		t.Errorf("delegation header = %q, want user:bob", gotHeader)
	}

	// Undelegated call from the parent client must NOT carry the header. This
	// also proves OnBehalfOf returns an independent clone.
	gotHeader = ""
	if _, err := c.Recall(context.Background(), RecallRequest{Query: "q"}); err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if gotHeader != "" {
		t.Errorf("parent client leaked delegation header: %q", gotHeader)
	}
}

func TestOnBehalfOfPropagatesToSubClients(t *testing.T) {
	var gotHeader, gotPath string
	srv := delegationProbe(t, `{"documents":[],"page":1,"pageSize":20,"total":0}`, &gotHeader, &gotPath)
	defer srv.Close()

	c := newTestClient(t, srv)
	del := c.OnBehalfOf("svc:indexer")
	if _, err := del.Documents().List(context.Background(), ListDocumentsOptions{}); err != nil {
		t.Fatalf("Documents.List: %v", err)
	}
	if gotHeader != "svc:indexer" {
		t.Errorf("sub-client delegation header = %q", gotHeader)
	}
	if gotPath != docsPath {
		t.Errorf("path = %q", gotPath)
	}
}

func TestWhoamiHitsMe(t *testing.T) {
	var gotHeader, gotPath string
	reply := `{"principalId":"user:bob","displayName":"Bob","kind":"user","enforce":true,` +
		`"grants":{"memory:read":["team/acme"]},"effectiveGrants":{"memory:read":["team/acme"]},` +
		`"delegatedPrincipalId":"svc:agent"}`
	srv := delegationProbe(t, reply, &gotHeader, &gotPath)
	defer srv.Close()

	c := newTestClient(t, srv)
	me, err := c.OnBehalfOf(principalBob).Whoami(context.Background())
	if err != nil {
		t.Fatalf("Whoami: %v", err)
	}
	if gotPath != "/api/v1/ctx-1/me" {
		t.Errorf("path = %q", gotPath)
	}
	if gotHeader != principalBob {
		t.Errorf("delegation header = %q", gotHeader)
	}
	if me.PrincipalID != principalBob || !me.Enforce || me.DelegatedPrincipalID != "svc:agent" {
		t.Errorf("whoami = %+v", me)
	}
	if _, ok := me.Grants["memory:read"]; !ok {
		t.Errorf("grants not decoded: %+v", me.Grants)
	}
}

func TestKeysCreateSendsBodyAndTTL(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"id":"k1","key":"sp-k1-secret","validUntil":"2026-12-31"}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	minted, err := c.Keys().Create(context.Background(), CreateKeyRequest{
		Name:       "ci",
		Grants:     map[string][]string{"memory:read": {"team/acme"}},
		TTLSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if cs.method != http.MethodPost || cs.path != "/api/v1/ctx-1/keys" {
		t.Errorf("method/path = %s %s", cs.method, cs.path)
	}
	if mustQuery(t, cs.query).Get("ttlSeconds") != "3600" {
		t.Errorf("ttlSeconds query = %q", cs.query)
	}
	if cs.body["name"] != "ci" {
		t.Errorf("body name = %v", cs.body["name"])
	}
	if _, hasTTL := cs.body["TTLSeconds"]; hasTTL {
		t.Errorf("TTLSeconds must not appear in the body: %#v", cs.body)
	}
	if minted.ID != "k1" || minted.Key != "sp-k1-secret" {
		t.Errorf("minted = %+v", minted)
	}
}

func TestKeysListRotateDelete(t *testing.T) {
	// List: bare array.
	var cs capture
	srv := captureServer(t, `[{"id":"k1","name":"ci","createdAt":"now"}]`, &cs)
	c := newTestClient(t, srv)
	keys, err := c.Keys().List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if cs.path != "/api/v1/ctx-1/keys" || len(keys) != 1 || keys[0].ID != "k1" {
		t.Errorf("list path/result = %s %+v", cs.path, keys)
	}
	srv.Close()

	// Rotate: POST /keys/{name}/rotate with ttlSeconds query.
	var cs2 capture
	srv2 := captureServer(t, `{"id":"k1","key":"sp-k1-new"}`, &cs2)
	c2 := newTestClient(t, srv2)
	minted, err := c2.Keys().Rotate(context.Background(), "ci", 60)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if cs2.path != "/api/v1/ctx-1/keys/ci/rotate" || mustQuery(t, cs2.query).Get("ttlSeconds") != "60" {
		t.Errorf("rotate path/query = %s ? %s", cs2.path, cs2.query)
	}
	if minted.Key != "sp-k1-new" {
		t.Errorf("rotated key = %q", minted.Key)
	}
	srv2.Close()

	// Delete: 204 No Content.
	var gotMethod, gotPath string
	srv3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv3.Close()
	c3 := newTestClient(t, srv3)
	if err := c3.Keys().Delete(context.Background(), "ci"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/api/v1/ctx-1/keys/ci" {
		t.Errorf("delete method/path = %s %s", gotMethod, gotPath)
	}
}
