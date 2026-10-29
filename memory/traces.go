package memory

import (
	"context"
	"iter"
	"net/url"
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
	// Parent is the response trace this one was retrieved for, on a single
	// trace read.
	Parent *ResponseTraceSummary `json:"parent,omitempty"`
	// Retrievals are the retrieval traces under this response, on a single
	// trace read. Listings omit both.
	Retrievals []RetrievalTraceSummary `json:"retrievals,omitempty"`
}

// ResponseTraceSummary is the response a retrieval trace belongs to.
type ResponseTraceSummary struct {
	ID           string `json:"id"`
	SessionID    string `json:"sessionId,omitempty"`
	UserMessage  string `json:"userMessage"`
	ResponseText string `json:"responseText"`
	Source       string `json:"source"`
	Cached       bool   `json:"cached"`
	ReusedFrom   string `json:"reusedFrom,omitempty"`
	LatencyMs    int    `json:"latencyMs"`
	CreatedAt    string `json:"createdAt"`
}

// RetrievalTraceSummary is one retrieval performed for a response.
type RetrievalTraceSummary struct {
	ID           string        `json:"id"`
	Mode         string        `json:"mode"`
	TierEntered  int           `json:"tierEntered"`
	TierReason   string        `json:"tierReason"`
	RetrievedIDs []string      `json:"retrievedIds"`
	Returned     []ReturnedRef `json:"returned"`
	Scores       []float64     `json:"scores"`
	LatencyMs    int           `json:"latencyMs"`
	CreatedAt    string        `json:"createdAt"`
}

// ReturnedRef is one row a retrieval returned.
type ReturnedRef struct {
	Table string   `json:"table"`
	ID    string   `json:"id"`
	Ref   string   `json:"ref,omitempty"`
	Label string   `json:"label,omitempty"`
	Rank  *int     `json:"rank,omitempty"`
	Score *float64 `json:"score,omitempty"`
}

// TraceListResponse is a page of traces from [Traces.List].
type TraceListResponse struct {
	Traces []TraceRecord `json:"traces"`
	Page   PageMeta      `json:"page"`
}

// List returns one page of recent query traces, newest first.
func (t *Traces) List(ctx context.Context, opts PageOptions) (*TraceListResponse, error) {
	q := url.Values{}
	opts.apply(q)
	var out TraceListResponse
	if err := t.client.getJSON(ctx, t.client.base+"/traces", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// All walks every page of the trace listing, newest first.
func (t *Traces) All(ctx context.Context, opts PageOptions) iter.Seq2[TraceRecord, error] {
	opts.Count = false
	return walkPages(ctx, opts.Cursor, func(ctx context.Context, cursor string) ([]TraceRecord, PageMeta, error) {
		opts.Cursor = cursor
		page, err := t.List(ctx, opts)
		if err != nil {
			return nil, PageMeta{}, err
		}
		return page.Traces, page.Page, nil
	})
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
	Direct    int `json:"direct"`
	Hybrid    int `json:"hybrid"`
	Escalated int `json:"escalated"`
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
