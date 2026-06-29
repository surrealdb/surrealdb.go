package spectron

import (
	"context"
	"net/url"
	"strconv"
)

// Traces is the trace sub-client returned by [Client.Traces].
type Traces struct {
	client *Client
}

// TraceRecord is a single recorded query trace.
type TraceRecord struct {
	ID             string `json:"id"`
	QueryText      string `json:"queryText"`
	ResolutionTier string `json:"resolutionTier"`
	TierReason     string `json:"tierReason"`
	Cached         bool   `json:"cached"`
	LatencyMs      int    `json:"latencyMs"`
	CreatedAt      string `json:"createdAt"`
}

// TraceListResponse is the result of [Traces.List].
type TraceListResponse struct {
	Traces []TraceRecord `json:"traces"`
}

// List returns recent query traces, newest first. A limit of 0 lets the server
// choose its default.
func (t *Traces) List(ctx context.Context, limit int) (*TraceListResponse, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var out TraceListResponse
	if err := t.client.getJSON(ctx, t.client.base+"/traces", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a single trace by id.
func (t *Traces) Get(ctx context.Context, traceID string) (*TraceRecord, error) {
	path := t.client.base + "/traces/" + url.PathEscape(traceID)
	var out TraceRecord
	if err := t.client.getJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ContradictionStats summarizes contradiction handling over the window.
type ContradictionStats struct {
	Contradictions    int     `json:"contradictions"`
	Reconciliations   int     `json:"reconciliations"`
	ContradictionRate float64 `json:"contradictionRate"`
}

// RetrievalStats summarizes candidate-set sizes over the window.
type RetrievalStats struct {
	Traces          int     `json:"traces"`
	AvgCandidateSet float64 `json:"avgCandidateSet"`
	MaxCandidateSet int     `json:"maxCandidateSet"`
}

// SourceKindCount counts hits attributed to a result source kind.
type SourceKindCount struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// SupersessionStats summarizes supersession churn over the window.
type SupersessionStats struct {
	SupersessionEvents int     `json:"supersessionEvents"`
	EntitiesChurned    int     `json:"entitiesChurned"`
	ChurnPerEntity     float64 `json:"churnPerEntity"`
}

// TierCounts counts queries served by each router tier.
type TierCounts struct {
	Direct      int `json:"direct"`
	Hybrid      int `json:"hybrid"`
	FullContext int `json:"fullContext"`
}

// TraceStatsResponse is the result of [Traces.Stats]: aggregate query and cache
// statistics over a recent window.
type TraceStatsResponse struct {
	WindowHours          int                `json:"windowHours"`
	TotalQueries         int                `json:"totalQueries"`
	AvgLatencyMs         float64            `json:"avgLatencyMs"`
	CacheHits            int                `json:"cacheHits"`
	CacheHitRate         float64            `json:"cacheHitRate"`
	ResponseTracesTotal  int                `json:"responseTracesTotal"`
	ResponseTracesCached int                `json:"responseTracesCached"`
	TierCounts           TierCounts         `json:"tierCounts"`
	SourceKindDist       []SourceKindCount  `json:"sourceKindDistribution"`
	Retrieval            RetrievalStats     `json:"retrieval"`
	Supersession         SupersessionStats  `json:"supersession"`
	Contradiction        ContradictionStats `json:"contradiction"`
}

// Stats returns aggregate trace statistics for the context.
func (t *Traces) Stats(ctx context.Context) (*TraceStatsResponse, error) {
	var out TraceStatsResponse
	if err := t.client.getJSON(ctx, t.client.base+"/traces/stats", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
