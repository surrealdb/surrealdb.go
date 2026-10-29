package memory

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// Document is a document row in the ingest store. The optional integer and
// string fields are pointers so an absent value is distinguishable from a zero.
type Document struct {
	ID                    string         `json:"id"`
	Title                 string         `json:"title"`
	Source                string         `json:"source"`
	MimeType              string         `json:"mimeType"`
	ContentHash           string         `json:"contentHash"`
	Status                DocumentStatus `json:"status"`
	SizeBytes             int            `json:"sizeBytes"`
	Version               int            `json:"version"`
	Language              string         `json:"language,omitempty"`
	Error                 string         `json:"error,omitempty"`
	ChunkCount            *int           `json:"chunkCount,omitempty"`
	KeywordCount          *int           `json:"keywordCount,omitempty"`
	ProcessingStartedAt   string         `json:"processingStartedAt,omitempty"`
	ProcessingCompletedAt string         `json:"processingCompletedAt,omitempty"`
	CreatedAt             string         `json:"createdAt"`
	UpdatedAt             string         `json:"updatedAt"`
}

// DocumentPage is a page of documents from [Documents.List].
type DocumentPage struct {
	Documents []Document `json:"documents"`
	Page      int        `json:"page"`
	PageSize  int        `json:"pageSize"`
	Total     int        `json:"total"`
}

// ListDocumentsOptions filter a [Documents.List] call. All fields are optional.
type ListDocumentsOptions struct {
	Status   DocumentStatus
	MimeType string
	Page     int
	PageSize int
}

func (o ListDocumentsOptions) values() url.Values {
	q := url.Values{}
	if o.Status != "" {
		q.Set("status", string(o.Status))
	}
	if o.MimeType != "" {
		q.Set("mime_type", o.MimeType)
	}
	if o.Page > 0 {
		q.Set("page", strconv.Itoa(o.Page))
	}
	if o.PageSize > 0 {
		q.Set("page_size", strconv.Itoa(o.PageSize))
	}
	return q
}

// List returns a page of documents in the context.
func (d *Documents) List(ctx context.Context, opts ListDocumentsOptions) (*DocumentPage, error) {
	var out DocumentPage
	if err := d.client.getJSON(ctx, d.client.base+"/documents", opts.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches a single document by id.
func (d *Documents) Get(ctx context.Context, id string) (*Document, error) {
	path := d.client.base + "/documents/" + url.PathEscape(id)
	var out Document
	if err := d.client.getJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Reprocess re-ingests an existing document with replacement bytes
// (PUT /documents/{id}). It shares the multipart behavior of Upload.
func (d *Documents) Reprocess(ctx context.Context, id string, body io.Reader, opts ...UploadOption) (*UploadResponse, error) {
	path := d.client.base + "/documents/" + url.PathEscape(id)
	return d.upload(ctx, http.MethodPut, path, body, opts...)
}

// Delete removes a document and its derived rows.
func (d *Documents) Delete(ctx context.Context, id string) error {
	path := d.client.base + "/documents/" + url.PathEscape(id)
	return d.client.doJSON(ctx, http.MethodDelete, path, nil, nil, false)
}

// Chunk is a single text chunk produced from a document.
type Chunk struct {
	ID         string `json:"id"`
	Document   string `json:"document"`
	Position   int    `json:"position"`
	CharStart  int    `json:"charStart"`
	CharEnd    int    `json:"charEnd"`
	Text       string `json:"text"`
	Section    string `json:"section,omitempty"`
	TokenCount *int   `json:"tokenCount,omitempty"`
}

// ChunkPage is a page of chunks from [Documents.Chunks].
type ChunkPage struct {
	Chunks   []Chunk `json:"chunks"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
	Total    int     `json:"total"`
}

// Chunks returns a page of the chunks derived from a document.
func (d *Documents) Chunks(ctx context.Context, id string, page, pageSize int) (*ChunkPage, error) {
	q := url.Values{}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if pageSize > 0 {
		q.Set("page_size", strconv.Itoa(pageSize))
	}
	path := d.client.base + "/documents/" + url.PathEscape(id) + "/chunks"
	var out ChunkPage
	if err := d.client.getJSON(ctx, path, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FetchRaw returns the original stored bytes of a document. The caller owns the
// returned bytes; the response is not JSON-decoded.
func (d *Documents) FetchRaw(ctx context.Context, id string) ([]byte, error) {
	path := d.client.base + "/documents/" + url.PathEscape(id) + "/raw"
	resp, err := d.client.do(ctx, http.MethodGet, path, nil, "", nil, true, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &APIError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("read raw document: %v", err)}
	}
	return data, nil
}

// QueryFilter narrows a document query to specific documents or MIME types.
type QueryFilter struct {
	DocumentIDs []string `json:"documentIds,omitempty"`
	MimeType    []string `json:"mimeType,omitempty"`
}

// DocumentQueryRequest is the input to [Documents.Query]. It maps to the spec's
// QueryRequestJson; the tuning knobs default server-side when left at zero.
type DocumentQueryRequest struct {
	Query          string          `json:"query"`
	K              int             `json:"k,omitempty"`
	Mode           QueryMode       `json:"mode,omitempty"`
	Threshold      float64         `json:"threshold,omitempty"`
	Filter         *QueryFilter    `json:"filter,omitempty"`
	Location       *GeoFilter      `json:"location,omitempty"`
	ExpandGraph    bool            `json:"expandGraph,omitempty"`
	GraphDepth     int             `json:"graphDepth,omitempty"`
	GraphAlpha     float64         `json:"graphAlpha,omitempty"`
	GraphEdges     []GraphEdgeKind `json:"graphEdges,omitempty"`
	RRFK           float64         `json:"rrfK,omitempty"`
	VectorWeight   float64         `json:"vectorWeight,omitempty"`
	DecomposeQuery *bool           `json:"decomposeQuery,omitempty"`
	UseHyde        *bool           `json:"useHyde,omitempty"`
	UseReranker    *bool           `json:"useReranker,omitempty"`
}

// GraphEvidence explains why a graph-expanded hit was surfaced.
type GraphEvidence struct {
	EdgeKind       GraphEdgeKind `json:"edgeKind"`
	NeighbourLabel string        `json:"neighbourLabel"`
	Weight         float64       `json:"weight"`
}

// QueryHitChunk is the chunk portion of a [QueryHit].
type QueryHitChunk struct {
	ID        string `json:"id"`
	Document  string `json:"document"`
	Position  int    `json:"position"`
	CharStart int    `json:"charStart"`
	CharEnd   int    `json:"charEnd"`
	Text      string `json:"text"`
	Section   string `json:"section,omitempty"`
}

// QueryHitDocument is the document portion of a [QueryHit].
type QueryHitDocument struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Source string `json:"source"`
}

// QueryHit is a single document-search match.
type QueryHit struct {
	Score          float64          `json:"score"`
	Chunk          QueryHitChunk    `json:"chunk"`
	Document       QueryHitDocument `json:"document"`
	GraphEvidence  []GraphEvidence  `json:"graphEvidence,omitempty"`
	GraphExpansion rawObject        `json:"graphExpansion,omitempty"`
}

// DocumentQueryResponse is the result of [Documents.Query].
type DocumentQueryResponse struct {
	Results []QueryHit `json:"results"`
	QueryMS int        `json:"queryMs"`
}

// Query runs passage retrieval over the context's documents.
func (d *Documents) Query(ctx context.Context, req *DocumentQueryRequest) (*DocumentQueryResponse, error) {
	var out DocumentQueryResponse
	if err := d.client.doJSON(ctx, http.MethodPost, d.client.base+"/documents/query", req, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

// RecomputeLinksResponse is the result of [Documents.RecomputeLinks].
type RecomputeLinksResponse struct {
	LinksEmitted int `json:"linksEmitted"`
}

// RecomputeLinks rebuilds the inter-document link graph for the context.
func (d *Documents) RecomputeLinks(ctx context.Context) (*RecomputeLinksResponse, error) {
	var out RecomputeLinksResponse
	if err := d.client.doJSON(ctx, http.MethodPost, d.client.base+"/documents/recompute-links", nil, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

// Keyword is a keyword extracted across the document corpus.
type Keyword struct {
	ID            string `json:"id"`
	Normalised    string `json:"normalised"`
	Text          string `json:"text"`
	DocumentCount int    `json:"documentCount"`
}

// KeywordPage is a page of keywords from [Documents.ListKeywords].
type KeywordPage struct {
	Keywords []Keyword `json:"keywords"`
	Page     int       `json:"page"`
	PageSize int       `json:"pageSize"`
	Total    int       `json:"total"`
}

// ListKeywordsOptions filter a [Documents.ListKeywords] call.
type ListKeywordsOptions struct {
	// Q filters keywords by a text prefix.
	Q string
	// MinDocumentCount drops keywords appearing in fewer documents.
	MinDocumentCount int
	// Sort selects the ordering (server-defined values).
	Sort     string
	Page     int
	PageSize int
}

func (o ListKeywordsOptions) values() url.Values {
	q := url.Values{}
	if o.Q != "" {
		q.Set("q", o.Q)
	}
	if o.MinDocumentCount > 0 {
		q.Set("minDocumentCount", strconv.Itoa(o.MinDocumentCount))
	}
	if o.Sort != "" {
		q.Set("sort", o.Sort)
	}
	if o.Page > 0 {
		q.Set("page", strconv.Itoa(o.Page))
	}
	if o.PageSize > 0 {
		q.Set("pageSize", strconv.Itoa(o.PageSize))
	}
	return q
}

// ListKeywords returns a page of corpus keywords.
func (d *Documents) ListKeywords(ctx context.Context, opts ListKeywordsOptions) (*KeywordPage, error) {
	var out KeywordPage
	if err := d.client.getJSON(ctx, d.client.base+"/documents/keywords", opts.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// KeywordSearchRequest is the input to [Documents.SearchKeywords].
type KeywordSearchRequest struct {
	Query     string  `json:"query"`
	K         int     `json:"k,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
}

// KeywordSearchHit is a single fuzzy keyword match.
type KeywordSearchHit struct {
	ID            string  `json:"id"`
	Normalised    string  `json:"normalised"`
	Text          string  `json:"text"`
	Score         float64 `json:"score"`
	DocumentCount int     `json:"documentCount"`
}

// KeywordSearchResponse is the result of [Documents.SearchKeywords].
type KeywordSearchResponse struct {
	Results []KeywordSearchHit `json:"results"`
	QueryMS int                `json:"queryMs"`
}

// SearchKeywords fuzzily matches keywords against a query string.
func (d *Documents) SearchKeywords(ctx context.Context, req KeywordSearchRequest) (*KeywordSearchResponse, error) {
	var out KeywordSearchResponse
	if err := d.client.doJSON(ctx, http.MethodPost, d.client.base+"/documents/keywords/search", req, &out, false); err != nil {
		return nil, err
	}
	return &out, nil
}

// KeywordDocument is a document a keyword appears in, with its match score.
type KeywordDocument struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	Score float64 `json:"score"`
}

// KeywordDetail is the result of [Documents.Keyword]: a keyword with the
// documents it appears in.
type KeywordDetail struct {
	ID            string            `json:"id"`
	Normalised    string            `json:"normalised"`
	Text          string            `json:"text"`
	DocumentCount int               `json:"documentCount"`
	Documents     []KeywordDocument `json:"documents"`
}

// Keyword fetches a single keyword by its normalised form.
func (d *Documents) Keyword(ctx context.Context, normalised string) (*KeywordDetail, error) {
	path := d.client.base + "/documents/keywords/" + url.PathEscape(normalised)
	var out KeywordDetail
	if err := d.client.getJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DocumentKeyword is a keyword attached to a specific document.
type DocumentKeyword struct {
	ID         string  `json:"id"`
	Normalised string  `json:"normalised"`
	Text       string  `json:"text"`
	Score      float64 `json:"score"`
}

// DocumentKeywordsResponse is the result of [Documents.KeywordsFor].
type DocumentKeywordsResponse struct {
	Keywords []DocumentKeyword `json:"keywords"`
}

// KeywordsFor returns the keywords extracted from a single document.
func (d *Documents) KeywordsFor(ctx context.Context, id string) (*DocumentKeywordsResponse, error) {
	path := d.client.base + "/documents/" + url.PathEscape(id) + "/keywords"
	var out DocumentKeywordsResponse
	if err := d.client.getJSON(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
