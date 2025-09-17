package surrealnote

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models"
)

// User handlers provide CRUD operations for user management in the note-taking system.
// These handlers form the foundation of user account management and are used both
// for administrative purposes and user self-service operations.

// handleCreateUser creates a new user account in the system with the provided user data.
// This handler accepts a JSON payload containing user information and persists it
// to the configured store (PostgreSQL, SurrealDB, or both in CQRS mode).
//
// HTTP Method: POST
// Endpoint: /api/users
// Content-Type: application/json
//
// Request body should contain a User object with:
//   - Name: User's display name (required)
//   - Email: User's email address (required, should be unique)
//   - Any other user fields defined in the models.User struct
//
// Response:
//   - 201 Created: User successfully created, returns created user with assigned ID
//   - 400 Bad Request: Invalid JSON payload or malformed request
//   - 500 Internal Server Error: Database operation failed or other system error
//
// The handler automatically assigns a unique ID to the new user and sets creation timestamps.
// In CQRS mode, the user is created in both PostgreSQL and SurrealDB stores.
//
// Production considerations:
//   - Input validation for email format, name length, required fields
//   - Rate limiting to prevent abuse and spam account creation
//   - Duplicate email checking with proper error handling
//   - Password hashing if authentication credentials are included
//   - Email verification workflow for account activation
//   - Audit logging for security and compliance
//   - Request size limits and timeout handling
//   - Proper error messages that don't leak sensitive information
//
// Usage example:
//
//	POST /api/users
//	{
//	  "name": "John Doe",
//	  "email": "john.doe@example.com"
//	}
func (a *App) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var user models.User
	// Should validate request size, add request timeout, and sanitize inputs
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	ctx := r.Context()
	if err := a.store.CreateUser(ctx, &user); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, user)
}

func (a *App) handleGetUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParseUserID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	ctx := r.Context()
	user, err := a.store.GetUser(ctx, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if user == nil {
		respondError(w, http.StatusNotFound, "User not found")
		return
	}

	respondJSON(w, http.StatusOK, user)
}

func (a *App) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParseUserID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	user.ID = id

	ctx := r.Context()
	if err := a.store.UpdateUser(ctx, &user); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, user)
}

func (a *App) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParseUserID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	ctx := r.Context()
	if err := a.store.DeleteUser(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusNoContent, nil)
}

// Workspace handlers
func (a *App) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var workspace models.Workspace
	if err := json.NewDecoder(r.Body).Decode(&workspace); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	ctx := r.Context()
	if err := a.store.CreateWorkspace(ctx, &workspace); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, workspace)
}

func (a *App) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParseWorkspaceID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid workspace ID")
		return
	}

	ctx := r.Context()
	workspace, err := a.store.GetWorkspace(ctx, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if workspace == nil {
		respondError(w, http.StatusNotFound, "Workspace not found")
		return
	}

	respondJSON(w, http.StatusOK, workspace)
}

func (a *App) handleUpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParseWorkspaceID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid workspace ID")
		return
	}

	var workspace models.Workspace
	if err := json.NewDecoder(r.Body).Decode(&workspace); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	workspace.ID = id

	ctx := r.Context()
	if err := a.store.UpdateWorkspace(ctx, &workspace); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, workspace)
}

func (a *App) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParseWorkspaceID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid workspace ID")
		return
	}

	ctx := r.Context()
	if err := a.store.DeleteWorkspace(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusNoContent, nil)
}

func (a *App) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIdStr := vars["userId"]
	userId, err := models.ParseUserID(userIdStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	ctx := r.Context()
	workspaces, err := a.store.ListWorkspaces(ctx, userId)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, workspaces)
}

// Page handlers
func (a *App) handleCreatePage(w http.ResponseWriter, r *http.Request) {
	var page models.Page
	if err := json.NewDecoder(r.Body).Decode(&page); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	ctx := r.Context()
	if err := a.store.CreatePage(ctx, &page); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, page)
}

func (a *App) handleGetPage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParsePageID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid page ID")
		return
	}

	ctx := r.Context()
	page, err := a.store.GetPage(ctx, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if page == nil {
		respondError(w, http.StatusNotFound, "Page not found")
		return
	}

	respondJSON(w, http.StatusOK, page)
}

func (a *App) handleUpdatePage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParsePageID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid page ID")
		return
	}

	var page models.Page
	if err := json.NewDecoder(r.Body).Decode(&page); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	page.ID = id

	log.Printf("handleUpdatePage: ID=%s, Title=%s", page.ID, page.Title)

	ctx := r.Context()
	if err := a.store.UpdatePage(ctx, &page); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, page)
}

func (a *App) handleDeletePage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParsePageID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid page ID")
		return
	}

	ctx := r.Context()
	if err := a.store.DeletePage(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusNoContent, nil)
}

func (a *App) handleListPages(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	workspaceIdStr := vars["workspaceId"]
	workspaceId, err := models.ParseWorkspaceID(workspaceIdStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid workspace ID")
		return
	}

	ctx := r.Context()
	pages, err := a.store.ListPages(ctx, workspaceId)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, pages)
}

// Block handlers
func (a *App) handleCreateBlock(w http.ResponseWriter, r *http.Request) {
	var block models.Block
	if err := json.NewDecoder(r.Body).Decode(&block); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	ctx := r.Context()
	if err := a.store.CreateBlock(ctx, &block); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, block)
}

func (a *App) handleGetBlock(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParseBlockID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid block ID")
		return
	}

	ctx := r.Context()
	block, err := a.store.GetBlock(ctx, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if block == nil {
		respondError(w, http.StatusNotFound, "Block not found")
		return
	}

	respondJSON(w, http.StatusOK, block)
}

func (a *App) handleUpdateBlock(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParseBlockID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid block ID")
		return
	}

	var block models.Block
	if err := json.NewDecoder(r.Body).Decode(&block); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	block.ID = id

	ctx := r.Context()
	if err := a.store.UpdateBlock(ctx, &block); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, block)
}

func (a *App) handleDeleteBlock(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParseBlockID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid block ID")
		return
	}

	ctx := r.Context()
	if err := a.store.DeleteBlock(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusNoContent, nil)
}

func (a *App) handleListBlocks(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pageIdStr := vars["pageId"]
	pageId, err := models.ParsePageID(pageIdStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid page ID")
		return
	}

	ctx := r.Context()
	blocks, err := a.store.ListBlocks(ctx, pageId)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, blocks)
}

// Permission handlers
func (a *App) handleCreatePermission(w http.ResponseWriter, r *http.Request) {
	var permission models.Permission
	if err := json.NewDecoder(r.Body).Decode(&permission); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	ctx := r.Context()
	if err := a.store.CreatePermission(ctx, &permission); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, permission)
}

func (a *App) handleGetPermission(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParseUserID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	ctx := r.Context()
	permissions, err := a.store.GetUserPermissions(ctx, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, permissions)
}

func (a *App) handleUpdatePermission(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParsePermissionID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid permission ID")
		return
	}

	var permission models.Permission
	if err := json.NewDecoder(r.Body).Decode(&permission); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	permission.ID = id

	ctx := r.Context()
	if err := a.store.UpdatePermission(ctx, &permission); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, permission)
}

func (a *App) handleDeletePermission(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParsePermissionID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid permission ID")
		return
	}

	ctx := r.Context()
	if err := a.store.DeletePermission(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusNoContent, nil)
}

// Comment handlers
func (a *App) handleCreateComment(w http.ResponseWriter, r *http.Request) {
	var comment models.Comment
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	ctx := r.Context()
	if err := a.store.CreateComment(ctx, &comment); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, comment)
}

func (a *App) handleGetComment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParseCommentID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid comment ID")
		return
	}

	ctx := r.Context()
	comment, err := a.store.GetComment(ctx, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if comment == nil {
		respondError(w, http.StatusNotFound, "Comment not found")
		return
	}

	respondJSON(w, http.StatusOK, comment)
}

func (a *App) handleUpdateComment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParseCommentID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid comment ID")
		return
	}

	var comment models.Comment
	if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	comment.ID = id

	ctx := r.Context()
	if err := a.store.UpdateComment(ctx, &comment); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, comment)
}

func (a *App) handleDeleteComment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := models.ParseCommentID(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid comment ID")
		return
	}

	ctx := r.Context()
	if err := a.store.DeleteComment(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusNoContent, nil)
}

func (a *App) handleListComments(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	blockIdStr := vars["blockId"]
	blockId, err := models.ParseBlockID(blockIdStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid block ID")
		return
	}

	ctx := r.Context()
	comments, err := a.store.ListComments(ctx, blockId)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, comments)
}

// Admin handlers provide administrative operations for managing application state during migrations.
// These endpoints are critical for zero-downtime database migrations and operational control.

// Helper functions provide common HTTP response handling for consistent API behavior.

// respondJSON sends a JSON response with the specified HTTP status code and payload.
// This function standardizes JSON response formatting across all API endpoints,
// ensuring consistent Content-Type headers and proper HTTP status codes.
//
// Parameters:
//   - w: HTTP response writer for sending the response
//   - status: HTTP status code (200, 201, 400, 404, 500, etc.)
//   - payload: Go object to be marshaled as JSON (can be nil for empty responses)
//
// The function automatically:
//   - Sets Content-Type to "application/json"
//   - Marshals the payload to JSON format
//   - Writes the status code before response body
//   - Handles nil payloads gracefully (sends empty response)
//
// Used by all API handlers to ensure consistent response formatting.
// JSON marshaling errors are silently ignored, which is acceptable for this
// demo application but should be handled properly in production systems.
//
// Production considerations:
//   - Add proper error handling for JSON marshaling failures
//   - Include request correlation IDs in responses
//   - Add response compression for large payloads
//   - Implement structured logging for response metrics
func respondJSON(w http.ResponseWriter, status int, payload any) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		_, _ = w.Write(response)
	}
}

// respondError sends a standardized JSON error response with the specified status and message.
// This function provides consistent error response formatting across all API endpoints,
// making it easier for client applications to handle errors uniformly.
//
// Parameters:
//   - w: HTTP response writer for sending the error response
//   - status: HTTP error status code (400, 404, 500, etc.)
//   - message: Human-readable error message for client consumption
//
// Response format:
//
//	{"error": "error message here"}
//
// Common usage patterns:
//   - respondError(w, http.StatusBadRequest, "Invalid request payload")
//   - respondError(w, http.StatusNotFound, "User not found")
//   - respondError(w, http.StatusInternalServerError, err.Error())
//
// The function ensures all error responses follow the same JSON structure,
// making client-side error handling predictable and consistent.
//
// Production considerations:
//   - Sanitize error messages to avoid information leakage
//   - Add error codes for programmatic client handling
//   - Include correlation IDs for request tracing
//   - Log detailed error information server-side while returning safe messages
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// API Handlers provide core system functionality including health checks and monitoring.

// handleHealth provides a health check endpoint for monitoring system status and configuration.
// This handler is used by load balancers, monitoring systems, and deployment pipelines
// to verify the application is running and responsive.
//
// HTTP Method: GET
// Endpoints: /health, /api/health
// Content-Type: application/json
//
// Response always returns HTTP 200 OK with a JSON object containing:
//   - status: Always "healthy" when the server can respond
//   - mode: Current migration mode (single, dual_write, validation, switching)
//   - time: Unix timestamp of the health check
//
// This endpoint provides valuable operational information:
//   - Service availability for load balancer health checks
//   - Current migration mode for operational dashboards
//   - Server time for clock synchronization verification
//   - Basic connectivity test for troubleshooting
//
// The health check is accessible at both root level (/health) and under the API prefix
// (/api/health) to support different monitoring configurations and legacy systems.
//
// Production considerations:
//   - This endpoint should not perform expensive operations
//   - Consider adding database connectivity checks for deeper health validation
//   - May include version information, build details, or feature flags
//   - Should be accessible without authentication for monitoring systems
//   - Response time should be minimized for load balancer timeouts
//
// Usage example:
//
//	GET /health
//	Response: {"status":"healthy","mode":"dual_write","time":1640995200}
func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]any{
		"status": "healthy",
		"mode":   a.config.MigrationMode,
		"time":   time.Now().Unix(),
	}
	respondJSON(w, http.StatusOK, response)
}
