package memory

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLookupRequestWire(t *testing.T) {
	var cs capture
	srv := captureServer(t, `{"resolution":{"kind":"topic"},"entities":{"items":[],"truncated":false},`+
		`"facts":{"items":[],"truncated":false},"relations":{"items":[],"truncated":false},`+
		`"events":{"items":[],"truncated":false},"passages":{"items":[],"truncated":false},`+
		`"uncertainty":{"items":[],"truncated":false}}`, &cs)
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Lookup(context.Background(), &LookupRequest{
		Query:     "acme",
		Subject:   "company/acme",
		Include:   []LookupSection{SectionFacts, SectionPassages},
		FactLimit: 5,
	})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if cs.path != "/api/v1/ctx-1/lookup" {
		t.Errorf("path = %s", cs.path)
	}
	if cs.body["query"] != "acme" || cs.body["subject"] != "company/acme" {
		t.Errorf("body = %+v", cs.body)
	}
	inc, _ := cs.body["include"].([]any)
	if len(inc) != 2 || inc[0] != "facts" || inc[1] != "passages" {
		t.Errorf("include = %#v", cs.body["include"])
	}
	if cs.body["factLimit"] != float64(5) {
		t.Errorf("factLimit = %#v", cs.body["factLimit"])
	}
	// Unset tuning knobs must not be sent as zeros.
	for _, k := range []string{"relationLimit", "eventLimit", "passageLimit", "uncertaintyLimit", "ambiguityMargin", "entityType"} {
		if _, present := cs.body[k]; present {
			t.Errorf("%s should be omitted when unset", k)
		}
	}
}

func TestLookupSendsIdempotencyKey(t *testing.T) {
	// Lookup is a read behind a POST, so it carries a key and gets the retry
	// budget.
	var gotIdem string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIdem = r.Header.Get("Idempotency-Key")
		_, _ = io.WriteString(w, `{"resolution":{"kind":"topic"},"entities":{"items":[],"truncated":false},`+
			`"facts":{"items":[],"truncated":false},"relations":{"items":[],"truncated":false},`+
			`"events":{"items":[],"truncated":false},"passages":{"items":[],"truncated":false},`+
			`"uncertainty":{"items":[],"truncated":false}}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, err := c.Lookup(context.Background(), &LookupRequest{Query: "q"}); err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if gotIdem == "" {
		t.Error("Lookup should send an Idempotency-Key")
	}
}

func TestLookupResolutionArms(t *testing.T) {
	sections := `"entities":{"items":[],"truncated":false},` +
		`"facts":{"items":[],"truncated":false},"relations":{"items":[],"truncated":false},` +
		`"events":{"items":[],"truncated":false},"passages":{"items":[],"truncated":false},` +
		`"uncertainty":{"items":[],"truncated":false}`
	entity := `{"entity":{"id":"e1","name":"Acme","entityType":"company"},"score":1,"factCount":7}`

	cases := []struct {
		name       string
		resolution string
		check      func(t *testing.T, r Resolution)
	}{
		{
			name:       "entity",
			resolution: `{"kind":"entity","confidence":1.0,"subject":` + entity + `}`,
			check: func(t *testing.T, r Resolution) {
				if r.Kind != ResolutionEntity {
					t.Fatalf("kind = %q", r.Kind)
				}
				if r.Subject == nil || r.Subject.Entity.Name != "Acme" || r.Subject.FactCount != 7 {
					t.Errorf("subject = %+v", r.Subject)
				}
				if r.Confidence != 1.0 {
					t.Errorf("confidence = %v", r.Confidence)
				}
			},
		},
		{
			name:       "ambiguous",
			resolution: `{"kind":"ambiguous","candidates":[` + entity + `]}`,
			check: func(t *testing.T, r Resolution) {
				if r.Kind != ResolutionAmbiguous || len(r.Candidates) != 1 {
					t.Fatalf("resolution = %+v", r)
				}
				if r.Subject != nil {
					t.Errorf("subject should be nil on the ambiguous arm")
				}
			},
		},
		{
			name:       "topic",
			resolution: `{"kind":"topic"}`,
			check: func(t *testing.T, r Resolution) {
				if r.Kind != ResolutionTopic {
					t.Fatalf("kind = %q", r.Kind)
				}
				// A topic answer leaves every slice nil — which is exactly why
				// callers must branch on Kind rather than on emptiness.
				if r.Subject != nil || r.Candidates != nil || r.Nearest != nil {
					t.Errorf("resolution = %+v, want only Kind set", r)
				}
			},
		},
		{
			name:       "empty",
			resolution: `{"kind":"empty","nearest":[` + entity + `]}`,
			check: func(t *testing.T, r Resolution) {
				if r.Kind != ResolutionEmpty || len(r.Nearest) != 1 {
					t.Fatalf("resolution = %+v", r)
				}
				if r.Nearest[0].Entity.Name != "Acme" {
					t.Errorf("nearest = %+v", r.Nearest)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cs capture
			srv := captureServer(t, `{"resolution":`+tc.resolution+`,`+sections+`}`, &cs)
			defer srv.Close()
			resp, err := newTestClient(t, srv).Lookup(context.Background(), &LookupRequest{Query: "q"})
			if err != nil {
				t.Fatalf("Lookup: %v", err)
			}
			tc.check(t, resp.Resolution)
		})
	}
}

func TestLookupSectionsDecodeWithTruncation(t *testing.T) {
	body := `{"resolution":{"kind":"topic"},` +
		`"entities":{"items":[{"entity":{"id":"e1","name":"Acme","entityType":"company"},"score":0.8,"factCount":3,"distinguisher":"the one in Leeds"}],"truncated":true},` +
		`"facts":{"items":[{"id":"a1","entity":"entity:company/acme","key":"hq","value":"Leeds"}],"truncated":true},` +
		`"relations":{"items":[{"id":"r1","subject":"s","label":"l","object":"o"}],"truncated":false},` +
		`"events":{"items":[{"id":"ac1","actor":"entity:person/alice","verb":"founded","summary":"s","confidence":1,"memoryCategory":"knowledge","createdAt":"now"}],"truncated":false},` +
		`"passages":{"items":[{"text":"Acme was founded in 1999","score":0.7,"documentId":"d1","position":3}],"truncated":true},` +
		`"uncertainty":{"items":[{"id":"u1","about":"hq","reason":"conflict","labels":[],"resolvable":true,"resolved":false,"scope":[["team/acme"]],"createdAt":"now"}],"truncated":false},` +
		`"coverage":{"sourceKinds":{"turn":4,"document":2}}}`

	var cs capture
	srv := captureServer(t, body, &cs)
	defer srv.Close()

	resp, err := newTestClient(t, srv).Lookup(context.Background(), &LookupRequest{Query: "acme"})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if len(resp.Entities.Items) != 1 || resp.Entities.Items[0].Distinguisher != "the one in Leeds" {
		t.Errorf("entities = %+v", resp.Entities)
	}
	if !resp.Facts.Truncated || !resp.Entities.Truncated || !resp.Passages.Truncated {
		t.Errorf("truncation flags = facts %v entities %v passages %v",
			resp.Facts.Truncated, resp.Entities.Truncated, resp.Passages.Truncated)
	}
	if resp.Relations.Truncated || resp.Events.Truncated || resp.Uncertainty.Truncated {
		t.Errorf("unexpected truncation on a complete section")
	}
	if len(resp.Passages.Items) != 1 || resp.Passages.Items[0].Position == nil || *resp.Passages.Items[0].Position != 3 {
		t.Errorf("passages = %+v", resp.Passages.Items)
	}
	if resp.Events.Items[0].Verb != "founded" {
		t.Errorf("events = %+v", resp.Events.Items)
	}
	if len(resp.Uncertainty.Items) != 1 || !resp.Uncertainty.Items[0].Resolvable {
		t.Errorf("uncertainty = %+v", resp.Uncertainty.Items)
	}
	if resp.Coverage == nil || resp.Coverage.SourceKinds["turn"] != 4 {
		t.Errorf("coverage = %+v", resp.Coverage)
	}
	// Scope decodes as the DNF selector the rest of the SDK uses.
	got := resp.Uncertainty.Items[0].Scope
	if len(got) != 1 || len(got[0]) != 1 || got[0][0] != scopeAcme {
		t.Errorf("scope = %+v", got)
	}
}

func TestLookupOmittedSectionIsNotTruncated(t *testing.T) {
	// A section that was not requested comes back empty with truncated false:
	// declined, not cut, so it points at no walk.
	body := `{"resolution":{"kind":"topic"},"entities":{"items":[],"truncated":false},` +
		`"facts":{"items":[],"truncated":false},"relations":{"items":[],"truncated":false},` +
		`"events":{"items":[],"truncated":false},"passages":{"items":[],"truncated":false},` +
		`"uncertainty":{"items":[],"truncated":false}}`
	var cs capture
	srv := captureServer(t, body, &cs)
	defer srv.Close()

	resp, err := newTestClient(t, srv).Lookup(context.Background(), &LookupRequest{
		Query:   "q",
		Include: []LookupSection{SectionFacts},
	})
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if resp.Passages.Truncated || len(resp.Passages.Items) != 0 {
		t.Errorf("declined section = %+v", resp.Passages)
	}
}

func TestLookupSectionConstantsMatchTheSpec(t *testing.T) {
	// "entities" is deliberately absent: it is filled by resolution, not
	// requested.
	want := "facts relations events passages uncertainty"
	got := []string{
		string(SectionFacts), string(SectionRelations), string(SectionEvents),
		string(SectionPassages), string(SectionUncertainty),
	}
	if strings.Join(got, " ") != want {
		t.Errorf("sections = %v, want %q", got, want)
	}
}
