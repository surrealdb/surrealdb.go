package connection

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code        int    `json:"code"`
	Message     string `json:"message,omitempty"`
	Description string `json:"description,omitempty"`
}

func (r RPCError) Error() string {
	if r.Description != "" {
		return r.Description
	}
	return r.Message
}

func (r *RPCError) Is(target error) bool {
	if target == nil {
		return r == nil
	}

	_, ok := target.(*RPCError)
	return ok
}

// RPCRequest represents an incoming JSON-RPC request.
// For SurrealDB v3+, Session and Txn can be set to scope the request to a specific session or transaction.
type RPCRequest struct {
	ID      any    `json:"id"`
	Method  string `json:"method,omitempty"`
	Params  []any  `json:"params,omitempty"`
	Session any    `json:"session,omitempty"` // SurrealDB v3: session UUID for session-scoped operations
	Txn     any    `json:"txn,omitempty"`     // SurrealDB v3: transaction UUID for transaction-scoped operations
}

// RPCResponse represents an outgoing JSON-RPC response
type RPCResponse[T any] struct {
	// ID is the ID of the request this response corresponds to.
	// Note that this is always nil in case of HTTPConnection.
	ID     any       `json:"id"`
	Error  *RPCError `json:"error,omitempty"`
	Result *T        `json:"result,omitempty"`
}

// RPCNotification represents an outgoing JSON-RPC notification
type RPCNotification struct {
	ID     any    `json:"id"`
	Method string `json:"method,omitempty"`
	Params []any  `json:"params,omitempty"`
}

type RPCFunction string

type ResponseID[T any] struct {
	ID *T `json:"id"`
}

var (
	Use          RPCFunction = "use"
	Info         RPCFunction = "info"
	SignUp       RPCFunction = "signup"
	SignIn       RPCFunction = "signin"
	Authenticate RPCFunction = "authenticate"
	Invalidate   RPCFunction = "invalidate"
	Let          RPCFunction = "let"
	Unset        RPCFunction = "unset"
	Live         RPCFunction = "live"
	Kill         RPCFunction = "kill"
	Query        RPCFunction = "query"
	Select       RPCFunction = "select"
	Create       RPCFunction = "create"
	Insert       RPCFunction = "insert"
	Update       RPCFunction = "update"
	Upsert       RPCFunction = "upsert"
	Relate       RPCFunction = "relate"
	Merge        RPCFunction = "merge"
	Patch        RPCFunction = "patch"
	Delete       RPCFunction = "delete"

	// SurrealDB v3+ session and transaction methods
	Attach RPCFunction = "attach"
	Detach RPCFunction = "detach"
	Begin  RPCFunction = "begin"
	Commit RPCFunction = "commit"
	Cancel RPCFunction = "cancel"
)
