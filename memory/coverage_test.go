package memory

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// implementedOps maps every operationId in the vendored spec to the exported
// Go method that calls it.
//
// This is the drift alarm. The spec is vendored by hand and the client is
// written by hand, so nothing else notices when the service grows an endpoint:
// re-vendor the spec, and this test names exactly what is now unimplemented.
// Add the mapping when you add the method — never to silence the failure.
var implementedOps = map[string]string{
	// Memory verbs.
	"create_fact":        "Client.Remember",
	"create_facts_batch": "Client.RememberMany",
	"query_memory":       "Client.Recall",
	"forget":             "Client.Forget",
	"chat":               "Client.Chat / Client.ChatStream",
	"lookup":             "Client.Lookup",
	"query_context":      "Client.QueryContext",
	"consolidate":        "Client.Consolidate",
	"reflect":            "Client.Reflect",
	"elaborate":          "Client.Elaborate",
	"fsck":               "Client.Fsck",
	"inspect":            "Client.Inspect",

	// Context state.
	"get_state":        "Client.State",
	"get_profile":      "Client.Profile",
	"list_audit":       "Client.Audit",
	"decay_importance": "Client.DecayImportance",
	"expire_context":   "Client.ExpireContext",
	"whoami":           "Client.Whoami",
	"health_check":     "Client.Health",

	// Documents.
	"upload_document":          "Documents.Upload",
	"reprocess_document":       "Documents.Reprocess",
	"list_documents":           "Documents.List",
	"get_document":             "Documents.Get",
	"delete_document":          "Documents.Delete",
	"list_chunks":              "Documents.Chunks",
	"fetch_document_raw":       "Documents.FetchRaw",
	"query_documents":          "Documents.Query",
	"recompute_document_links": "Documents.RecomputeLinks",
	"list_keywords":            "Documents.ListKeywords",
	"search_keywords":          "Documents.SearchKeywords",
	"get_keyword":              "Documents.Keyword",
	"list_document_keywords":   "Documents.KeywordsFor",

	// Entities.
	"list_entities":        "Entities.List",
	"get_entity":           "Entities.Get",
	"delete_entity":        "Entities.Delete",
	"get_entity_history":   "Entities.History",
	"entity_history_all":   "Entities.Changes",
	"search_entities":      "Entities.Search",
	"top_entities":         "Entities.Top",
	"entity_neighbourhood": "Entities.Neighbours",

	// Facts.
	"list_attributes": "Facts.Attributes",
	"list_relations":  "Facts.Relations",
	"list_actions":    "Facts.Actions",

	// Uncertainty.
	"list_uncertainty":    "Uncertainty.List",
	"resolve_uncertainty": "Uncertainty.Resolve",

	// Sessions.
	"create_session":      "Sessions.Create",
	"delete_session":      "Sessions.Delete",
	"get_session_context": "Sessions.Context",
	"list_turns":          "Sessions.Turns",

	// Scopes.
	"list_scopes":       "Scopes.List",
	"register_scope":    "Scopes.Register",
	"delete_scope":      "Scopes.Delete",
	"forget_scope":      "Scopes.Forget",
	"scope_grants_stub": "Scopes.Grants",

	// Principals.
	"list_principals":     "Principals.List",
	"get_principal":       "Principals.Get",
	"effective_principal": "Principals.Effective",
	"grant_principal":     "Principals.Grant",
	"revoke_principal":    "Principals.Revoke",

	// Keys.
	"create_self_key": "Keys.Create",
	"list_self_keys":  "Keys.List",
	"delete_self_key": "Keys.Delete",
	"rotate_self_key": "Keys.Rotate",

	// Traces.
	"list_traces":     "Traces.List",
	"get_trace":       "Traces.Get",
	"get_trace_stats": "Traces.Stats",
}

// specOperations reads every operationId out of the vendored spec.
func specOperations(t *testing.T) map[string]string {
	t.Helper()
	raw, err := os.ReadFile("openapi/enduser.swagger.json")
	if err != nil {
		t.Fatalf("read vendored spec: %v", err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse vendored spec: %v", err)
	}
	ops := make(map[string]string)
	for path, methods := range doc.Paths {
		for method, op := range methods {
			switch strings.ToLower(method) {
			case "get", "post", "put", "patch", "delete":
			default:
				continue
			}
			if op.OperationID == "" {
				t.Errorf("%s %s has no operationId", strings.ToUpper(method), path)
				continue
			}
			ops[op.OperationID] = strings.ToUpper(method) + " " + path
		}
	}
	return ops
}

func TestEverySpecOperationIsImplemented(t *testing.T) {
	ops := specOperations(t)

	var missing []string
	for id, route := range ops {
		if _, ok := implementedOps[id]; !ok {
			missing = append(missing, id+"  ("+route+")")
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d spec operation(s) have no Go method:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}

	var stale []string
	for id := range implementedOps {
		if _, ok := ops[id]; !ok {
			stale = append(stale, id)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("%d mapped operation(s) no longer exist in the spec:\n  %s",
			len(stale), strings.Join(stale, "\n  "))
	}

	if t.Failed() {
		return
	}
	t.Logf("all %d spec operations are implemented", len(ops))
}

// schemaBindings maps a spec schema to the Go type that models it.
//
// The types are hand-written, so nothing else notices when the service adds a
// field: the struct simply drops it on decode, or omits it on encode, in
// silence. This test compares json tags against the vendored spec so a
// re-vendor names exactly what changed.
//
// Schemas deliberately absent: ApiErrorResponse (decoded by hand in
// errors.go), InspectResponseJson and DocumentUploadForm (untyped by design),
// SectionJson_* (one generic Section[T]), ResourceRef and ResolutionJson
// (oneOf flattened onto a Kind discriminator), and the string enums.
var schemaBindings = map[string]any{
	"AttributeDetailJson":        AttributeDetail{},
	"RelationDetailJson":         RelationDetail{},
	"ActionDetailJson":           ActionDetail{},
	"EntityDetailJson":           EntityDetail{},
	"EntityMatchJson":            EntityMatch{},
	"EntityResponseJson":         EntityResponse{},
	"EntityTruncationJson":       EntityTruncation{},
	"NeighbourJson":              Neighbour{},
	"SourceRefJson":              SourceRef{},
	"Triple":                     Triple{},
	"FactsRequest":               RememberRequest{},
	"BatchMessage":               BatchMessage{},
	"ExtractionResultJson":       ExtractionResult{},
	"ActionSummaryJson":          ActionSummary{},
	"MemoryHitJson":              RecallHit{},
	"QueryMemoryRequestJson":     RecallRequest{},
	"QueryMemoryResponseJson":    RecallResponse{},
	"QueryWindowJson":            QueryWindow{},
	"ForgetRequestJson":          forgetRequest{},
	"ChatRequestJson":            ChatRequest{},
	"ChatResponseJson":           ChatResponse{},
	"CitationJson":               Citation{},
	"DocumentJson":               Document{},
	"UploadResponse":             UploadResponse{},
	"UploadMetadataJson":         uploadMetadata{},
	"ContextQueryRequestJson":    ContextQueryRequest{},
	"ConsolidateOutcomeJson":     ConsolidateOutcome{},
	"FsckReportJson":             FsckReport{},
	"UnscopedContentFindingJson": UnscopedContentFinding{},
	"ProfileEntryJson":           ProfileEntry{},
	"ProfileResponseJson":        ProfileResponse{},
	"CategoryStateJson":          CategoryState{},
	"StateResponseJson":          StateResponse{},
	"StateTruncationJson":        StateTruncation{},
	"TierCountsJson":             TierCounts{},
	"TraceRecordJson":            TraceRecord{},
	"ResponseTraceSummaryJson":   ResponseTraceSummary{},
	"RetrievalTraceSummaryJson":  RetrievalTraceSummary{},
	"ReturnedRefJson":            ReturnedRef{},
	"WhoamiJson":                 WhoamiResponse{},
	"UncertaintyJson":            UnknownFact{},
	"CoverageJson":               Coverage{},
	"PageMeta":                   PageMeta{},
}

// fieldsHandledOutsideTheStruct are spec properties a Go struct deliberately
// does not carry, with the reason.
var fieldsHandledOutsideTheStruct = map[string]string{
	// chatWirePayload injects stream into the request map; a struct field
	// would let a caller set it and desync from the method they called.
	"ChatRequestJson.stream": "set by Client.Chat / Client.ChatStream",
}

func jsonTags(t *testing.T, v any) map[string]bool {
	t.Helper()
	rt := reflect.TypeOf(v)
	tags := make(map[string]bool)
	for i := 0; i < rt.NumField(); i++ {
		tag := rt.Field(i).Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if name == "" || name == "-" {
			continue
		}
		tags[name] = true
	}
	return tags
}

func TestGoTypesMatchTheVendoredSpec(t *testing.T) {
	raw, err := os.ReadFile("openapi/enduser.swagger.json")
	if err != nil {
		t.Fatalf("read vendored spec: %v", err)
	}
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse vendored spec: %v", err)
	}

	for schemaName, goValue := range schemaBindings {
		schema, ok := doc.Components.Schemas[schemaName]
		if !ok {
			t.Errorf("%s: bound to %T but no longer in the spec",
				schemaName, goValue)
			continue
		}
		have := jsonTags(t, goValue)

		var missing, extra []string
		for field := range schema.Properties {
			if have[field] {
				continue
			}
			if why, excused := fieldsHandledOutsideTheStruct[schemaName+"."+field]; excused {
				t.Logf("%s.%s: %s", schemaName, field, why)
				continue
			}
			missing = append(missing, field)
		}
		for field := range have {
			if _, ok := schema.Properties[field]; !ok {
				extra = append(extra, field)
			}
		}
		sort.Strings(missing)
		sort.Strings(extra)

		if len(missing) > 0 {
			t.Errorf("%T is missing %d field(s) the spec defines on %s: %s",
				goValue, len(missing), schemaName, strings.Join(missing, ", "))
		}
		if len(extra) > 0 {
			t.Errorf("%T carries %d field(s) %s no longer defines: %s",
				goValue, len(extra), schemaName, strings.Join(extra, ", "))
		}
	}
}
