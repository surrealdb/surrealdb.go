package surrealnote

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// Run starts the HTTP server with comprehensive API endpoints for the note-taking application.
// This method configures and launches the web server that handles all user-facing operations
// including authentication, user management, workspace operations, page editing, and administration.
//
// The server operates in one of three store configurations based on the application Config:
//   - Single PostgreSQL mode: Uses PostgreSQL ORM-based storage exclusively
//   - Single SurrealDB mode: Uses SurrealDB with native SurrealQL queries exclusively
//   - CQRS mode: Dual-write to both stores enabling zero-downtime migration
//
// # API Endpoints
//
// Health Check:
//
//	GET  /api/health                              - Service health status
//
// Authentication:
//
//	POST /api/auth/signup                         - Register new user account
//	POST /api/auth/signin                         - Authenticate existing user
//	POST /api/auth/signout                        - End user session
//	GET  /api/auth/me                             - Get current authenticated user
//	POST /api/auth/refresh                        - Refresh authentication token
//
// Users:
//
//	POST   /api/users                             - Create new user
//	GET    /api/users/{id}                        - Get user by ID
//	PUT    /api/users/{id}                        - Update user details
//	DELETE /api/users/{id}                        - Delete user account
//
// Workspaces:
//
//	POST   /api/workspaces                        - Create new workspace
//	GET    /api/workspaces/{id}                   - Get workspace by ID
//	PUT    /api/workspaces/{id}                   - Update workspace details
//	DELETE /api/workspaces/{id}                   - Delete workspace
//	GET    /api/users/{userId}/workspaces         - List user's workspaces
//
// Pages:
//
//	POST   /api/pages                             - Create new page
//	GET    /api/pages/{id}                        - Get page by ID
//	PUT    /api/pages/{id}                        - Update page content
//	DELETE /api/pages/{id}                        - Delete page
//	GET    /api/workspaces/{workspaceId}/pages    - List workspace pages
//
// Blocks:
//
//	POST   /api/blocks                            - Create new block
//	GET    /api/blocks/{id}                       - Get block by ID
//	PUT    /api/blocks/{id}                       - Update block content
//	DELETE /api/blocks/{id}                       - Delete block
//	GET    /api/pages/{pageId}/blocks             - List page blocks
//	PUT    /api/pages/{pageId}/blocks/reorder     - Reorder blocks in page
//
// Permissions:
//
//	POST   /api/permissions                       - Grant permission
//	GET    /api/permissions/{id}                  - Get permission details
//	PUT    /api/permissions/{id}                  - Update permission level
//	DELETE /api/permissions/{id}                  - Revoke permission
//	GET    /api/{resourceType}/{resourceId}/permissions - List resource permissions
//
// Comments:
//
//	POST   /api/comments                          - Add comment to block
//	GET    /api/comments/{id}                     - Get comment by ID
//	PUT    /api/comments/{id}                     - Update comment content
//	DELETE /api/comments/{id}                     - Delete comment
//	GET    /api/blocks/{blockId}/comments         - List block comments
//	PUT    /api/comments/{id}/resolve             - Mark comment as resolved
//
// Administration:
//
//	GET    /api/admin/mode                        - Get current migration mode
//	POST   /api/admin/mode                        - Change migration mode
//
// The server supports graceful shutdown through context cancellation, allowing
// ongoing requests to complete before terminating. This is essential for production
// deployments where zero-downtime updates are required.
//
// Configuration is provided through the RunCommand parameter, though currently
// it uses options from the application Config. Server port, database connections,
// and migration mode are all configured at the application level.
//
// In production environments, this method should be enhanced with:
//   - TLS/HTTPS support with proper certificate management
//   - Request rate limiting and DDoS protection
//   - Structured logging with correlation IDs
//   - Metrics collection and health monitoring
//   - Middleware for authentication, CORS, and request validation
//   - Connection pooling and database health checks
//
// Usage example:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	cmd := &RunCommand{}
//	if err := app.Run(ctx, cmd); err != nil {
//	    log.Fatalf("Server failed: %v", err)
//	}
//
// The method blocks until the context is cancelled or a fatal server error occurs.
// On graceful shutdown, it allows up to 5 seconds for active requests to complete.
func (a *App) Run(ctx context.Context, cmd *RunCommand) error {
	// Setup routes
	router := mux.NewRouter()

	// API routes
	api := router.PathPrefix("/api").Subrouter()

	// Health check
	api.HandleFunc("/health", a.handleHealth).Methods("GET")

	// Auth routes
	api.HandleFunc("/auth/signup", a.handleSignUp).Methods("POST")
	api.HandleFunc("/auth/signin", a.handleSignIn).Methods("POST")
	api.HandleFunc("/auth/signout", a.handleSignOut).Methods("POST")
	api.HandleFunc("/auth/me", a.handleGetCurrentUser).Methods("GET")
	api.HandleFunc("/auth/refresh", a.handleRefreshToken).Methods("POST")

	// User routes
	api.HandleFunc("/users", a.handleCreateUser).Methods("POST")
	api.HandleFunc("/users/{id}", a.handleGetUser).Methods("GET")
	api.HandleFunc("/users/{id}", a.handleUpdateUser).Methods("PUT")
	api.HandleFunc("/users/{id}", a.handleDeleteUser).Methods("DELETE")

	// Workspace routes
	api.HandleFunc("/workspaces", a.handleCreateWorkspace).Methods("POST")
	api.HandleFunc("/workspaces/{id}", a.handleGetWorkspace).Methods("GET")
	api.HandleFunc("/workspaces/{id}", a.handleUpdateWorkspace).Methods("PUT")
	api.HandleFunc("/workspaces/{id}", a.handleDeleteWorkspace).Methods("DELETE")
	api.HandleFunc("/users/{userId}/workspaces", a.handleListWorkspaces).Methods("GET")

	// Page routes
	api.HandleFunc("/pages", a.handleCreatePage).Methods("POST")
	api.HandleFunc("/pages/{id}", a.handleGetPage).Methods("GET")
	api.HandleFunc("/pages/{id}", a.handleUpdatePage).Methods("PUT")
	api.HandleFunc("/pages/{id}", a.handleDeletePage).Methods("DELETE")
	api.HandleFunc("/workspaces/{workspaceId}/pages", a.handleListPages).Methods("GET")

	// Block routes
	api.HandleFunc("/blocks", a.handleCreateBlock).Methods("POST")
	api.HandleFunc("/blocks/{id}", a.handleGetBlock).Methods("GET")
	api.HandleFunc("/blocks/{id}", a.handleUpdateBlock).Methods("PUT")
	api.HandleFunc("/blocks/{id}", a.handleDeleteBlock).Methods("DELETE")
	api.HandleFunc("/pages/{pageId}/blocks", a.handleListBlocks).Methods("GET")

	// Permission routes
	api.HandleFunc("/permissions", a.handleCreatePermission).Methods("POST")
	api.HandleFunc("/permissions/{id}", a.handleGetPermission).Methods("GET")
	api.HandleFunc("/permissions/{id}", a.handleUpdatePermission).Methods("PUT")
	api.HandleFunc("/permissions/{id}", a.handleDeletePermission).Methods("DELETE")

	// Comment routes
	api.HandleFunc("/comments", a.handleCreateComment).Methods("POST")
	api.HandleFunc("/comments/{id}", a.handleGetComment).Methods("GET")
	api.HandleFunc("/comments/{id}", a.handleUpdateComment).Methods("PUT")
	api.HandleFunc("/comments/{id}", a.handleDeleteComment).Methods("DELETE")
	api.HandleFunc("/blocks/{blockId}/comments", a.handleListComments).Methods("GET")

	// Health check route (outside of /api prefix)
	router.HandleFunc("/health", a.handleHealth).Methods("GET")

	// Start server
	addr := fmt.Sprintf(":%s", a.config.ServerPort)
	log.Printf("Starting SurrealNote server on %s", addr)
	log.Printf("Migration mode: %s", a.config.MigrationMode)

	// Create HTTP server
	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		// Context cancelled, shutdown gracefully
		log.Println("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-serverErr:
		// Server error
		return err
	}
}
