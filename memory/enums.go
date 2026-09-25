package memory

// This file mirrors the string enums declared in the Agent Memory OpenAPI spec
// (doc/spec/enduser.swagger.json, vendored under openapi/). Each enum is a
// defined string type so callers get compile-time names while the wire form
// stays a plain string. The server is the authority on the accepted set; the
// constants here track the spec and unknown values still decode without error.

// InferMode selects how a /facts write is interpreted.
type InferMode string

const (
	// InferFull runs full LLM extraction over the text (the server default).
	InferFull InferMode = "full"
	// InferTriples consumes caller-supplied triples without LLM extraction.
	InferTriples InferMode = "triples"
	// InferPreview extracts but does not persist, returning the preview.
	InferPreview InferMode = "preview"
	// InferNone stores the text verbatim with no extraction.
	InferNone InferMode = "none"
)

// MemoryCategory is the reconciler's coarse classification of a row.
type MemoryCategory string

const (
	MemoryIdentity  MemoryCategory = "identity"
	MemoryKnowledge MemoryCategory = "knowledge"
	MemoryContext   MemoryCategory = "context"
)

// TurnRole is the speaker role on a conversational turn.
type TurnRole string

const (
	RoleUser      TurnRole = "user"
	RoleAssistant TurnRole = "assistant"
	RoleSystem    TurnRole = "system"
	RoleTool      TurnRole = "tool"
)

// BatchExtractionMode controls how a /facts/batch write groups extraction.
type BatchExtractionMode string

const (
	// ExtractPerMessage extracts each message independently.
	ExtractPerMessage BatchExtractionMode = "per_message"
	// ExtractWholeConversation extracts over the batch as a single conversation.
	ExtractWholeConversation BatchExtractionMode = "whole_conversation"
)

// DocumentStatus is the position of a document in the ingest pipeline.
type DocumentStatus string

const (
	DocQueued          DocumentStatus = "queued"
	DocExtracting      DocumentStatus = "extracting"
	DocChunking        DocumentStatus = "chunking"
	DocEmbedding       DocumentStatus = "embedding"
	DocKeywording      DocumentStatus = "keywording"
	DocExtractingNodes DocumentStatus = "extracting_nodes"
	DocReady           DocumentStatus = "ready"
	DocFailed          DocumentStatus = "failed"
)

// QueryKind is the query-understanding classification returned on /query.
type QueryKind string

const (
	QueryDirectLookup QueryKind = "direct_lookup"
	QueryHybrid       QueryKind = "hybrid"
	QueryFullContext  QueryKind = "full_context"
)

// Tier is the router tier that served a /query read.
type Tier string

const (
	TierDirect Tier = "direct"
	TierCache  Tier = "cache"
	TierHybrid Tier = "hybrid"
	// TierEscalated is the full-context tier. The wire value was renamed from
	// "full_context" to "escalated"; note that [QueryKind] still uses
	// "full_context" for its own, separate, classification.
	TierEscalated Tier = "escalated"

	// Deprecated: the server renamed this tier to "escalated". Comparing
	// against this constant silently never matches. Use [TierEscalated].
	TierFullContext Tier = "full_context"
)

// ResultKind is the substrate row table a recall hit came from.
type ResultKind string

const (
	ResultAttribute   ResultKind = "attribute"
	ResultEntity      ResultKind = "entity"
	ResultAction      ResultKind = "action"
	ResultChunk       ResultKind = "chunk"
	ResultMemoryChunk ResultKind = "memory_chunk"
	ResultTurn        ResultKind = "turn"
	ResultSection     ResultKind = "section"
)

// QueryMode selects the retrieval strategy on the /documents/query read.
type QueryMode string

const (
	ModeHybrid      QueryMode = "hybrid"
	ModeVector      QueryMode = "vector"
	ModeBM25        QueryMode = "bm25"
	ModeHybridGraph QueryMode = "hybrid_graph"
)

// MemoryQueryMode selects the retrieval strategy on the memory /query read.
// It is distinct from [QueryMode] (the document-query enum): the memory router
// exposes "graph" rather than "hybrid_graph".
type MemoryQueryMode string

const (
	MemoryModeHybrid MemoryQueryMode = "hybrid"
	MemoryModeVector MemoryQueryMode = "vector"
	MemoryModeBM25   MemoryQueryMode = "bm25"
	MemoryModeGraph  MemoryQueryMode = "graph"
)

// TraceKind is the category of an audited operation or trace row.
type TraceKind string

const (
	TraceDecision  TraceKind = "decision"
	TraceRetrieval TraceKind = "retrieval"
	TraceResponse  TraceKind = "response"
)

// DecisionKind is the action a consolidation outcome recorded.
type DecisionKind string

const (
	DecisionCreate    DecisionKind = "create"
	DecisionUpdate    DecisionKind = "update"
	DecisionSupersede DecisionKind = "supersede"
)

// InjectionKind classifies a suspected prompt-injection finding.
type InjectionKind string

const (
	InjectionInstructionOverride InjectionKind = "instruction_override"
	InjectionRoleOverride        InjectionKind = "role_override"
	InjectionToolOverride        InjectionKind = "tool_override"
	InjectionSystemPromptLeak    InjectionKind = "system_prompt_leak"
	InjectionPromptTemplate      InjectionKind = "prompt_template_injection"
	InjectionBase64Instruction   InjectionKind = "base64_instruction"
)

// GraphEdgeKind is the kind of edge traversed during graph expansion.
type GraphEdgeKind string

const (
	EdgeKnowledgeHasKeyword GraphEdgeKind = "knowledge_has_keyword"
	EdgeSectionMatch        GraphEdgeKind = "section_match"
	EdgeDocumentLink        GraphEdgeKind = "document_link"
	EdgeDocumentSummary     GraphEdgeKind = "document_summary"
	EdgeHybridGraph         GraphEdgeKind = "hybrid_graph"

	// Deprecated: the server no longer emits this edge kind, and rejects it
	// as a [DocumentQueryRequest.GraphEdges] filter.
	EdgeKeywordCooccurrence GraphEdgeKind = "keyword_cooccurrence"
)
