package surrealnote

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/client"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models"
)

// Simple in-memory session store for demo purposes
// In production, use a proper session store (Redis, etc.)
var (
	sessions  = make(map[string]*models.User)
	sessionMu sync.RWMutex
)

// generateToken creates a cryptographically secure random token for authentication sessions.
// This function generates a 32-byte random token encoded as a hexadecimal string,
// providing 256 bits of entropy suitable for secure session identification.
//
// The token generation process creates a 32-byte buffer for random data,
// fills it with cryptographically secure random bytes using crypto/rand,
// then encodes the bytes as a hexadecimal string for safe transport and storage.
// This produces a 64-character hex string (32 bytes * 2 hex chars per byte).
//
// Return values:
//   - string: 64-character hexadecimal token on success
//   - error: Non-nil if system random number generator fails
//
// Security properties:
//   - Uses crypto/rand for cryptographically secure randomness
//   - 256-bit entropy provides excellent security against brute force attacks
//   - Hexadecimal encoding ensures safe use in HTTP headers and JSON
//   - No predictable patterns or time-based components
//
// Production considerations:
//   - Token expiration should be implemented for enhanced security
//   - Consider JWT tokens for stateless authentication in distributed systems
//   - Implement token rotation for long-lived sessions
//   - Add rate limiting for token generation endpoints
//   - Consider shorter tokens with database lookup for reduced attack surface
//
// The generated tokens are suitable for:
//   - HTTP Bearer token authentication
//   - Session cookies (with proper security flags)
//   - API key generation
//   - Temporary access tokens
//
// Usage example:
//
//	token, err := generateToken()
//	if err != nil {
//	    return fmt.Errorf("failed to generate auth token: %w", err)
//	}
//	// token is now a 64-character hex string like "a1b2c3d4..."
func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// handleSignUp handles user registration and account creation with automatic authentication.
// This endpoint creates new user accounts and immediately provides authentication tokens,
// enabling seamless onboarding without requiring separate signin after registration.
//
// HTTP Method: POST
// Endpoint: /api/auth/signup
// Content-Type: application/json
//
// Request body should contain SignUpRequest with:
//   - Email: User's email address (required, used as unique identifier)
//   - Name: User's display name (required)
//   - Password: User's chosen password (currently not validated for demo)
//
// Response:
//   - 200 OK: User successfully created, returns AuthResponse with token and user data
//   - 400 Bad Request: Invalid JSON payload or malformed request
//   - 500 Internal Server Error: Database operation failed or token generation error
//
// The registration process validates and parses the JSON request payload,
// creates a new User record with unique ID and timestamps, then persists
// it to the configured store (handling CQRS dual-write if enabled).
// After successfully storing the user, it generates a secure authentication token,
// stores the session in memory for immediate authentication, and returns
// both the token and user data for client use.
//
// Security considerations for production:
//   - Password hashing with bcrypt or similar secure algorithms
//   - Email format validation and normalization
//   - Rate limiting to prevent registration abuse
//   - Email verification workflow to confirm account ownership
//   - Duplicate email checking with proper error handling
//   - CAPTCHA integration to prevent automated registration
//   - Input sanitization to prevent injection attacks
//   - Audit logging for security monitoring
//
// The current implementation uses a simple in-memory session store suitable
// for demonstration purposes but should be replaced with a proper session store
// (Redis, database) for production deployments with multiple application instances.
//
// Usage example:
//
//	POST /api/auth/signup
//	{
//	  "email": "jane.doe@example.com",
//	  "name": "Jane Doe",
//	  "password": "securepassword123"
//	}
//	Response: {"token":"abc123...","user":{...}}
func (a *App) handleSignUp(w http.ResponseWriter, r *http.Request) {
	var req client.SignUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create user with unique ID
	user := &models.User{
		Email:     req.Email,
		Name:      req.Name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save user to database
	if err := a.store.CreateUser(r.Context(), user); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate auth token
	token, err := generateToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Store session
	sessionMu.Lock()
	sessions[token] = user
	sessionMu.Unlock()

	// Return response
	resp := client.AuthResponse{
		Token: token,
		User:  user,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleSignIn handles user authentication
func (a *App) handleSignIn(w http.ResponseWriter, r *http.Request) {
	var req client.SignInRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Find user by email
	user, err := a.store.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// In a real app, verify password hash here
	// For demo, we accept any password

	// Generate auth token
	token, err := generateToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Store session
	sessionMu.Lock()
	sessions[token] = user
	sessionMu.Unlock()

	// Return response
	resp := client.AuthResponse{
		Token: token,
		User:  user,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// getTokenFromHeader extracts the token from the Authorization header
func getTokenFromHeader(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	// Remove "Bearer " prefix if present
	const bearerPrefix = "Bearer "
	if len(auth) > len(bearerPrefix) && auth[:len(bearerPrefix)] == bearerPrefix {
		return auth[len(bearerPrefix):]
	}
	return auth
}

// handleSignOut handles user logout
func (a *App) handleSignOut(w http.ResponseWriter, r *http.Request) {
	token := getTokenFromHeader(r)
	if token != "" {
		sessionMu.Lock()
		delete(sessions, token)
		sessionMu.Unlock()
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// handleGetCurrentUser handles getting the current authenticated user
func (a *App) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	token := getTokenFromHeader(r)
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionMu.RLock()
	user, ok := sessions[token]
	sessionMu.RUnlock()

	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// handleRefreshToken handles token refresh
func (a *App) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	oldToken := getTokenFromHeader(r)
	if oldToken == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	sessionMu.RLock()
	user, ok := sessions[oldToken]
	sessionMu.RUnlock()

	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Generate new token
	newToken, err := generateToken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update session
	sessionMu.Lock()
	delete(sessions, oldToken)
	sessions[newToken] = user
	sessionMu.Unlock()

	// Return response
	resp := client.AuthResponse{
		Token: newToken,
		User:  user,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
