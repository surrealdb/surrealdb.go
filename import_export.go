package surrealdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/surrealdb/surrealdb.go/pkg/connection"
)

// ImportError is returned by [DB.Import] when statements of the imported
// document failed. The statements which succeeded are not rolled back.
type ImportError struct {
	// Failures holds the error of each failed statement, in order. Each is an
	// *RPCError which unwraps to a *ServerError, as a failed query statement is.
	Failures []error
}

func (e *ImportError) Error() string {
	if len(e.Failures) == 1 {
		return "import: 1 statement failed: " + e.Failures[0].Error()
	}
	return fmt.Sprintf("import: %d statements failed, the first: %v", len(e.Failures), e.Failures[0])
}

// Unwrap returns the errors of the failed statements.
func (e *ImportError) Unwrap() []error {
	return e.Failures
}

// ExportOptions chooses what [DB.Export] writes. The zero value is what the
// server exports by default: every definition and every record.
//
// There is no option for the history of versioned tables: SurrealDB 3.0 to
// 3.3.0 accept the export's versions flag but ignore it, and later versions
// refuse it, so no export over HTTP holds that history.
type ExportOptions struct {
	ExcludeUsers     bool
	ExcludeAccesses  bool
	ExcludeParams    bool
	ExcludeFunctions bool
	ExcludeAnalyzers bool
	// ExcludeRecords exports the definitions of tables, but not their records.
	ExcludeRecords bool
	// Tables, when not empty, limits the export to these tables.
	Tables []string
}

// config returns the export configuration the server takes, with only the
// options that differ from its defaults, so that servers which do not know
// an option are not sent it.
func (o *ExportOptions) config() map[string]any {
	cfg := map[string]any{}
	if o == nil {
		return cfg
	}
	exclude := map[string]bool{
		"users":     o.ExcludeUsers,
		"accesses":  o.ExcludeAccesses,
		"params":    o.ExcludeParams,
		"functions": o.ExcludeFunctions,
		"analyzers": o.ExcludeAnalyzers,
		"records":   o.ExcludeRecords,
	}
	for key, excluded := range exclude {
		if excluded {
			cfg[key] = false
		}
	}
	if len(o.Tables) > 0 {
		cfg["tables"] = o.Tables
	}
	return cfg
}

// Import runs a SurrealQL document, such as one written by [DB.Export], in the
// namespace and database of the default session, with POST /import. It works
// over WebSocket and HTTP connections alike, authenticated as the default
// session is.
//
// SurrealDB 3.x requires the document to start with OPTION IMPORT;, as an
// export does.
//
// When statements fail, Import returns an *ImportError listing them, read from
// the server's answer: SurrealDB 3.0.5 and later list the failed statements,
// with status 422 from 3.3.1. The statements which succeeded are kept.
func (db *DB) Import(ctx context.Context, document io.Reader) error {
	res, err := db.httpRequest(ctx, "/import", document, "text/plain", "application/json")
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("import: reading the answer: %w", err)
	}

	if failures := failedStatements(body); len(failures) > 0 {
		return &ImportError{Failures: failures}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("import failed: %s: %s", res.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

// failedStatements reads the statement results /import answers with, and
// returns the error of each that failed.
func failedStatements(body []byte) []error {
	var results []struct {
		Status  string `json:"status"`
		Result  any    `json:"result"`
		Kind    string `json:"kind"`
		Details any    `json:"details"`
	}
	if json.Unmarshal(body, &results) != nil {
		return nil
	}

	var failures []error
	for _, r := range results {
		if r.Status != "ERR" {
			continue
		}
		message, ok := r.Result.(string)
		if !ok {
			message = fmt.Sprint(r.Result)
		}
		failures = append(failures, connection.NewStatementError(message, r.Kind, r.Details))
	}
	return failures
}

// Export writes the namespace and database of the default session as a
// SurrealQL document, with POST /export, as the server produces it. It works
// over WebSocket and HTTP connections alike, authenticated as the default
// session is. A nil options exports what the server does by default.
func (db *DB) Export(ctx context.Context, w io.Writer, options *ExportOptions) error {
	cfg, err := json.Marshal(options.config())
	if err != nil {
		return err
	}

	res, err := db.httpRequest(ctx, "/export", strings.NewReader(string(cfg)), "application/json", "text/plain")
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
		return fmt.Errorf("export failed: %s: %s", res.Status, strings.TrimSpace(string(body)))
	}
	if _, err := io.Copy(w, res.Body); err != nil {
		return fmt.Errorf("export: %w", err)
	}
	return nil
}

// errNoHTTPEndpoint is returned for a connection which cannot reach the
// server's HTTP endpoints.
var errNoHTTPEndpoint = errors.New("the connection does not know the server's HTTP endpoint")

// httpRequest posts a body to one of the server's HTTP endpoints, with the
// namespace, database and token of the default session.
func (db *DB) httpRequest(ctx context.Context, path string, body io.Reader, contentType, accept string) (*http.Response, error) {
	endpoint, ok := db.con.(connection.HTTPEndpoint)
	if !ok {
		return nil, errNoHTTPEndpoint
	}
	baseURL, client := endpoint.HTTPEndpoint()
	if baseURL == "" {
		return nil, errNoHTTPEndpoint
	}
	base, err := connection.HTTPBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = http.DefaultClient
	}
	if client.Timeout != 0 {
		// A bulk transfer may outlast the timeout meant for RPC requests, and
		// cutting an import short could leave it half applied; ctx bounds it.
		unbounded := *client
		unbounded.Timeout = 0
		client = &unbounded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", accept)

	snap := db.state.snapshot(db.renew)
	if snap.namespace != "" {
		req.Header.Set("Surreal-NS", snap.namespace)
	}
	if snap.database != "" {
		req.Header.Set("Surreal-DB", snap.database)
	}
	if snap.token != "" {
		req.Header.Set("Authorization", "Bearer "+snap.token)
	}

	return client.Do(req)
}
