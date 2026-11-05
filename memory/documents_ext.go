package memory

import (
	"context"
	"fmt"
	"io"
	"iter"
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
	Error                 string         `json:"error,omitempty"`
	ProcessingStartedAt   string         `json:"processingStartedAt,omitempty"`
	ProcessingCompletedAt string         `json:"processingCompletedAt,omitempty"`
	CreatedAt             string         `json:"createdAt"`
	UpdatedAt             string         `json:"updatedAt"`
	// ObservedAt is the assertion instant the document's facts are dated from.
	ObservedAt string `json:"observedAt,omitempty"`
}

// DocumentPage is a page of documents from [Documents.List].
type DocumentPage struct {
	Documents []Document `json:"documents"`
	Page      PageMeta   `json:"page"`
}

// ListDocumentsOptions filters and paginates a [Documents.List] call. All
// fields are optional.
type ListDocumentsOptions struct {
	Status   DocumentStatus
	MimeType string

	// Limit, Cursor and Count are the cursor pagination inputs; see
	// [PageOptions], whose fields these mirror.
	Limit  int
	Cursor string
	Count  bool

	// Page and PageSize select the deprecated offset mode; see
	// [OffsetOptions]. Mixing them with Limit/Cursor/Count is an error.
	Page     int
	PageSize int
}

func (o ListDocumentsOptions) page() PageOptions {
	return PageOptions{Limit: o.Limit, Cursor: o.Cursor, Count: o.Count}
}

func (o ListDocumentsOptions) offset() OffsetOptions {
	return OffsetOptions{Page: o.Page, PageSize: o.PageSize}
}

func (o ListDocumentsOptions) values() url.Values {
	q := url.Values{}
	if o.Status != "" {
		q.Set("status", string(o.Status))
	}
	if o.MimeType != "" {
		q.Set("mimeType", o.MimeType)
	}
	o.page().apply(q)
	o.offset().apply(q)
	return q
}

// List returns one page of the documents in the context.
func (d *Documents) List(ctx context.Context, opts ListDocumentsOptions) (*DocumentPage, error) {
	if err := checkPageMode(opts.page(), opts.offset()); err != nil {
		return nil, err
	}
	var out DocumentPage
	if err := d.client.getJSON(ctx, d.client.base+"/documents", opts.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// All walks every page of the document listing. opts.Cursor seeds the walk and
// opts.Limit sets the page size; opts.Count is ignored, since a walk visits
// every row anyway.
func (d *Documents) All(ctx context.Context, opts ListDocumentsOptions) iter.Seq2[Document, error] {
	opts.Count = false
	return walkPages(ctx, opts.Cursor, func(ctx context.Context, cursor string) ([]Document, PageMeta, error) {
		opts.Cursor = cursor
		page, err := d.List(ctx, opts)
		if err != nil {
			return nil, PageMeta{}, err
		}
		return page.Documents, page.Page, nil
	})
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
	Chunks []Chunk  `json:"chunks"`
	Page   PageMeta `json:"page"`
}

// ListChunksOptions paginates a [Documents.Chunks] call.
type ListChunksOptions struct {
	// Limit, Cursor and Count are the cursor pagination inputs; see
	// [PageOptions].
	Limit  int
	Cursor string
	Count  bool

	// Page and PageSize select the deprecated offset mode; see
	// [OffsetOptions].
	Page     int
	PageSize int
}

func (o ListChunksOptions) page() PageOptions {
	return PageOptions{Limit: o.Limit, Cursor: o.Cursor, Count: o.Count}
}

func (o ListChunksOptions) offset() OffsetOptions {
	return OffsetOptions{Page: o.Page, PageSize: o.PageSize}
}

// Chunks returns one page of the chunks derived from a document.
func (d *Documents) Chunks(ctx context.Context, id string, opts ListChunksOptions) (*ChunkPage, error) {
	if err := checkPageMode(opts.page(), opts.offset()); err != nil {
		return nil, err
	}
	q := url.Values{}
	opts.page().apply(q)
	opts.offset().apply(q)
	path := d.client.base + "/documents/" + url.PathEscape(id) + "/chunks"
	var out ChunkPage
	if err := d.client.getJSON(ctx, path, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AllChunks walks every page of a document's chunks.
func (d *Documents) AllChunks(ctx context.Context, id string, opts ListChunksOptions) iter.Seq2[Chunk, error] {
	opts.Count = false
	return walkPages(ctx, opts.Cursor, func(ctx context.Context, cursor string) ([]Chunk, PageMeta, error) {
		opts.Cursor = cursor
		page, err := d.Chunks(ctx, id, opts)
		if err != nil {
			return nil, PageMeta{}, err
		}
		return page.Chunks, page.Page, nil
	})
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
	// Lens narrows the read to a scope region. It can only narrow: the
	// caller's grants still gate on top, so a lens outside the caller's
	// region yields no results rather than a 403.
	Lens ScopeSets `json:"lens,omitempty"`
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
	if err := d.client.doJSON(ctx, http.MethodPost, d.client.base+"/documents/query", req, &out, true); err != nil {
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
	Page     PageMeta  `json:"page"`
}

// ListKeywordsOptions filters and paginates a [Documents.ListKeywords] call.
type ListKeywordsOptions struct {
	// Q filters keywords by a text prefix.
	Q string
	// MinDocumentCount drops keywords appearing in fewer documents.
	MinDocumentCount int
	// Sort selects the ordering (server-defined values).
	Sort string

	// Limit, Cursor and Count are the cursor pagination inputs; see
	// [PageOptions].
	Limit  int
	Cursor string
	Count  bool

	// Page and PageSize select the deprecated offset mode; see
	// [OffsetOptions].
	Page     int
	PageSize int
}

func (o ListKeywordsOptions) page() PageOptions {
	return PageOptions{Limit: o.Limit, Cursor: o.Cursor, Count: o.Count}
}

func (o ListKeywordsOptions) offset() OffsetOptions {
	return OffsetOptions{Page: o.Page, PageSize: o.PageSize}
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
	o.page().apply(q)
	o.offset().apply(q)
	return q
}

// ListKeywords returns one page of corpus keywords.
func (d *Documents) ListKeywords(ctx context.Context, opts ListKeywordsOptions) (*KeywordPage, error) {
	if err := checkPageMode(opts.page(), opts.offset()); err != nil {
		return nil, err
	}
	var out KeywordPage
	if err := d.client.getJSON(ctx, d.client.base+"/documents/keywords", opts.values(), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AllKeywords walks every page of the corpus keyword listing.
func (d *Documents) AllKeywords(ctx context.Context, opts ListKeywordsOptions) iter.Seq2[Keyword, error] {
	opts.Count = false
	return walkPages(ctx, opts.Cursor, func(ctx context.Context, cursor string) ([]Keyword, PageMeta, error) {
		opts.Cursor = cursor
		page, err := d.ListKeywords(ctx, opts)
		if err != nil {
			return nil, PageMeta{}, err
		}
		return page.Keywords, page.Page, nil
	})
}

// KeywordSearchRequest is the input to [Documents.SearchKeywords].
type KeywordSearchRequest struct {
	Query     string  `json:"query"`
	K         int     `json:"k,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
	// Lens narrows the search to a scope region, with the same
	// narrow-only semantics as [DocumentQueryRequest.Lens].
	Lens ScopeSets `json:"lens,omitempty"`
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
	if err := d.client.doJSON(ctx, http.MethodPost, d.client.base+"/documents/keywords/search", req, &out, true); err != nil {
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
