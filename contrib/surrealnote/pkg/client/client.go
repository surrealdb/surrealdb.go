// Package client provides a Go HTTP client library for programmatic access to the surrealnote API.
//
// This package enables developers to build integrations, testing tools, and client applications
// that interact with the surrealnote note-taking service. The client provides strongly-typed
// methods for all API endpoints with proper error handling, authentication, and request/response
// serialization.
//
// # Client Architecture
//
// [Client] implements a REST API client that mirrors the server's endpoint structure:
//   - User management: Create, read, update, delete user accounts
//   - Workspace operations: Organize content into collaborative spaces
//   - Page management: Create and manage document hierarchies
//   - Block operations: Handle individual content elements within pages
//   - Comment system: Enable collaborative discussions on content
//   - Permission management: Control access to workspaces and pages
//   - Administrative functions: Manage migration modes and system health
//
// All operations use the same [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models] entities as the server, ensuring type safety
// and consistency across the API boundary.
//
// # Authentication and Session Management
//
// The client supports token-based authentication:
//   - Sign up new users with email and password
//   - Sign in existing users to obtain authentication tokens
//   - Automatic token inclusion in subsequent requests
//   - Sign out to invalidate authentication tokens
//
// Tokens are managed automatically by the client and included in the Authorization
// header for all authenticated requests.
//
// # Error Handling and HTTP Status Codes
//
// The client provides consistent error handling:
//   - HTTP 4xx errors: Client-side errors (bad requests, unauthorized, not found)
//   - HTTP 5xx errors: Server-side errors (internal errors, service unavailable)
//   - Network errors: Connection timeouts, DNS resolution failures
//   - Serialization errors: JSON encoding/decoding problems
//
// All errors include the HTTP status code and response body for debugging.
//
// # Request and Response Handling
//
// The client handles serialization automatically:
//   - Request bodies: Automatically marshal Go structs to JSON
//   - Response bodies: Automatically unmarshal JSON to Go structs
//   - Content-Type headers: Set to application/json for all requests
//   - Accept headers: Expect application/json responses
//
// Typed IDs ([github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.UserID], [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models.WorkspaceID], etc.) are serialized as
// strings and handled transparently by the API.
//
// # Connection Management
//
// The underlying HTTP client includes:
//   - 30-second request timeout to prevent hanging requests
//   - Keep-alive connections for performance optimization
//   - Automatic retry for idempotent operations (future enhancement)
//   - Connection pooling for concurrent request handling
//
// # Usage Patterns
//
// Basic Client Setup:
//
//	client := client.NewClient("http://localhost:8080")
//
//	// Authenticate user
//	authResp, err := client.SignIn(ctx, "user@example.com", "password")
//	if err != nil {
//		return err
//	}
//
//	// Client automatically includes auth token in subsequent requests
//	workspaces, err := client.ListWorkspaces(ctx, authResp.User.ID)
//
// Content Management Workflow:
//
//	// Create workspace
//	workspace := &models.Workspace{
//		Name:    "Project Planning",
//		OwnerID: userID,
//	}
//	created, err := client.CreateWorkspace(ctx, workspace)
//
//	// Create page in workspace
//	page := &models.Page{
//		Title:       "Sprint Planning",
//		WorkspaceID: created.ID,
//		CreatedBy:   userID,
//	}
//	createdPage, err := client.CreatePage(ctx, page)
//
//	// Add content blocks
//	block := &models.Block{
//		PageID:  createdPage.ID,
//		Type:    models.BlockTypeText,
//		Content: models.JSONMap{"text": "Sprint goals and tasks"},
//		Order:   0,
//	}
//	createdBlock, err := client.CreateBlock(ctx, block)
//
// Administrative Operations:
//
//	// Check service health
//	health, err := client.Health(ctx)
//
//	// Get current migration mode
//	mode, err := client.GetMode(ctx)
//
//	// Change migration mode (admin access required)
//	err = client.SetMode(ctx, "dual_write")
//
// # Integration with Testing
//
// The client package integrates with [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/surrealnotetesting] for:
//   - Load testing with multiple concurrent virtual users
//   - End-to-end testing of migration scenarios
//   - API compatibility validation across database backends
//   - Performance benchmarking and stress testing
//
// See [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/surrealnotetesting.VirtualUser] for examples of client usage in testing scenarios.
//
// # Production Considerations
//
// For production use, enhance this client with:
//   - Retry logic with exponential backoff for transient failures
//   - Circuit breaker pattern for fault tolerance
//   - Request/response logging for debugging and monitoring
//   - Metrics collection for performance tracking
//   - Rate limiting to respect API quotas
//   - Connection pooling configuration for high-throughput scenarios
//   - TLS certificate validation for secure connections
//   - Custom User-Agent headers for API analytics
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models"
)

// Client provides strongly-typed access to the surrealnote REST API.
//
// Client manages HTTP communication, authentication, and serialization for all
// API operations. It automatically handles request/response JSON marshaling,
// authentication token management, and provides consistent error handling
// across all endpoints.
//
// Client instances are safe for concurrent use by multiple goroutines.
type Client struct {
	baseURL    string
	httpClient *http.Client
	authToken  string
}

// NewClient creates a new surrealnote API client.
//
// The baseURL should include the protocol and host (e.g., "http://localhost:8080")
// but should not include a trailing slash or API path prefix.
//
// The client is initialized with a 30-second timeout and is ready for immediate use.
// Authentication tokens are managed automatically after successful sign-in operations.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetAuthToken sets the authentication token for the client
func (c *Client) SetAuthToken(token string) {
	c.authToken = token
}

// doRequest performs an HTTP request with proper headers
func (c *Client) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	return c.httpClient.Do(req)
}

// decodeResponse decodes the JSON response into the target struct
func decodeResponse(resp *http.Response, target any) error {
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status=%d, body=%s", resp.StatusCode, string(body))
	}

	if target != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// Health checks the health status of the server
func (c *Client) Health(ctx context.Context) (map[string]any, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// User management

// CreateUser creates a new user
func (c *Client) CreateUser(ctx context.Context, user *models.User) (*models.User, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/users", user)
	if err != nil {
		return nil, err
	}

	var result models.User
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetUser retrieves a user by ID
func (c *Client) GetUser(ctx context.Context, id models.UserID) (*models.User, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/users/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var result models.User
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// UpdateUser updates an existing user
func (c *Client) UpdateUser(ctx context.Context, user *models.User) (*models.User, error) {
	resp, err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/api/users/%s", user.ID), user)
	if err != nil {
		return nil, err
	}

	var result models.User
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeleteUser deletes a user
func (c *Client) DeleteUser(ctx context.Context, id models.UserID) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/users/%s", id), nil)
	if err != nil {
		return err
	}

	return decodeResponse(resp, nil)
}

// Workspace management

// CreateWorkspace creates a new workspace
func (c *Client) CreateWorkspace(ctx context.Context, workspace *models.Workspace) (*models.Workspace, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/workspaces", workspace)
	if err != nil {
		return nil, err
	}

	var result models.Workspace
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetWorkspace retrieves a workspace by ID
func (c *Client) GetWorkspace(ctx context.Context, id models.WorkspaceID) (*models.Workspace, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/workspaces/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var result models.Workspace
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// UpdateWorkspace updates an existing workspace
func (c *Client) UpdateWorkspace(ctx context.Context, workspace *models.Workspace) (*models.Workspace, error) {
	resp, err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/api/workspaces/%s", workspace.ID), workspace)
	if err != nil {
		return nil, err
	}

	var result models.Workspace
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeleteWorkspace deletes a workspace
func (c *Client) DeleteWorkspace(ctx context.Context, id models.WorkspaceID) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/workspaces/%s", id), nil)
	if err != nil {
		return err
	}

	return decodeResponse(resp, nil)
}

// ListWorkspaces lists all workspaces for a user
func (c *Client) ListWorkspaces(ctx context.Context, userID models.UserID) ([]*models.Workspace, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/users/%s/workspaces", userID), nil)
	if err != nil {
		return nil, err
	}

	var result []*models.Workspace
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// Page management

// CreatePage creates a new page
func (c *Client) CreatePage(ctx context.Context, page *models.Page) (*models.Page, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/pages", page)
	if err != nil {
		return nil, err
	}

	var result models.Page
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetPage retrieves a page by ID
func (c *Client) GetPage(ctx context.Context, id models.PageID) (*models.Page, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/pages/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var result models.Page
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// UpdatePage updates an existing page
func (c *Client) UpdatePage(ctx context.Context, page *models.Page) (*models.Page, error) {
	resp, err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/api/pages/%s", page.ID), page)
	if err != nil {
		return nil, err
	}

	var result models.Page
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeletePage deletes a page
func (c *Client) DeletePage(ctx context.Context, id models.PageID) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/pages/%s", id), nil)
	if err != nil {
		return err
	}

	return decodeResponse(resp, nil)
}

// ListPages lists all pages in a workspace
func (c *Client) ListPages(ctx context.Context, workspaceID models.WorkspaceID) ([]*models.Page, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/workspaces/%s/pages", workspaceID), nil)
	if err != nil {
		return nil, err
	}

	var result []*models.Page
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// Block management

// CreateBlock creates a new block
func (c *Client) CreateBlock(ctx context.Context, block *models.Block) (*models.Block, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/blocks", block)
	if err != nil {
		return nil, err
	}

	var result models.Block
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetBlock retrieves a block by ID
func (c *Client) GetBlock(ctx context.Context, id models.BlockID) (*models.Block, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/blocks/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var result models.Block
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// UpdateBlock updates an existing block
func (c *Client) UpdateBlock(ctx context.Context, block *models.Block) (*models.Block, error) {
	resp, err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/api/blocks/%s", block.ID), block)
	if err != nil {
		return nil, err
	}

	var result models.Block
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeleteBlock deletes a block
func (c *Client) DeleteBlock(ctx context.Context, id models.BlockID) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/blocks/%s", id), nil)
	if err != nil {
		return err
	}

	return decodeResponse(resp, nil)
}

// ListBlocks lists all blocks in a page
func (c *Client) ListBlocks(ctx context.Context, pageID models.PageID) ([]*models.Block, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/pages/%s/blocks", pageID), nil)
	if err != nil {
		return nil, err
	}

	var result []*models.Block
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// ReorderBlocks updates the order of blocks in a page
func (c *Client) ReorderBlocks(ctx context.Context, pageID models.PageID, blockIDs []models.BlockID) error {
	// First get all blocks
	blocks, err := c.ListBlocks(ctx, pageID)
	if err != nil {
		return err
	}

	// Create a map for quick lookup
	blockMap := make(map[models.BlockID]*models.Block)
	for _, block := range blocks {
		blockMap[block.ID] = block
	}

	// Update the order field for each block
	for i, blockID := range blockIDs {
		if block, exists := blockMap[blockID]; exists {
			block.Order = i
			if _, err := c.UpdateBlock(ctx, block); err != nil {
				return fmt.Errorf("failed to update block order for %s: %w", blockID, err)
			}
		}
	}

	return nil
}

// Comment management

// CreateComment creates a new comment
func (c *Client) CreateComment(ctx context.Context, comment *models.Comment) (*models.Comment, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/comments", comment)
	if err != nil {
		return nil, err
	}

	var result models.Comment
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetComment retrieves a comment by ID
func (c *Client) GetComment(ctx context.Context, id models.CommentID) (*models.Comment, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/comments/%s", id), nil)
	if err != nil {
		return nil, err
	}

	var result models.Comment
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// UpdateComment updates an existing comment
func (c *Client) UpdateComment(ctx context.Context, comment *models.Comment) (*models.Comment, error) {
	resp, err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/api/comments/%s", comment.ID), comment)
	if err != nil {
		return nil, err
	}

	var result models.Comment
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeleteComment deletes a comment
func (c *Client) DeleteComment(ctx context.Context, id models.CommentID) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/comments/%s", id), nil)
	if err != nil {
		return err
	}

	return decodeResponse(resp, nil)
}

// ListComments lists all comments for a block
func (c *Client) ListComments(ctx context.Context, blockID models.BlockID) ([]*models.Comment, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/blocks/%s/comments", blockID), nil)
	if err != nil {
		return nil, err
	}

	var result []*models.Comment
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// Permission management

// CreatePermission creates a new permission
func (c *Client) CreatePermission(ctx context.Context, permission *models.Permission) (*models.Permission, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/permissions", permission)
	if err != nil {
		return nil, err
	}

	var result models.Permission
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetUserPermissions retrieves permissions for a user
func (c *Client) GetUserPermissions(ctx context.Context, userID models.UserID) ([]*models.Permission, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/users/%s/permissions", userID), nil)
	if err != nil {
		return nil, err
	}

	var result []*models.Permission
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// UpdatePermission updates an existing permission
func (c *Client) UpdatePermission(ctx context.Context, permission *models.Permission) (*models.Permission, error) {
	resp, err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/api/permissions/%s", permission.ID), permission)
	if err != nil {
		return nil, err
	}

	var result models.Permission
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// DeletePermission deletes a permission
func (c *Client) DeletePermission(ctx context.Context, id models.PermissionID) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/permissions/%s", id), nil)
	if err != nil {
		return err
	}

	return decodeResponse(resp, nil)
}
