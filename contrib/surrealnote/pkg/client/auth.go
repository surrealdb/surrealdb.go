package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models"
)

// SignUpRequest represents a sign-up request
type SignUpRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// SignInRequest represents a sign-in request
type SignInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse represents an authentication response
type AuthResponse struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

// SignUp creates a new user account
func (c *Client) SignUp(ctx context.Context, email, password, name string) (*AuthResponse, error) {
	req := SignUpRequest{
		Email:    email,
		Password: password,
		Name:     name,
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/auth/signup", req)
	if err != nil {
		return nil, fmt.Errorf("signup request failed: %w", err)
	}

	var result AuthResponse
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode signup response: %w", err)
	}

	// Automatically set the auth token for subsequent requests
	c.SetAuthToken(result.Token)

	return &result, nil
}

// SignIn authenticates an existing user
func (c *Client) SignIn(ctx context.Context, email, password string) (*AuthResponse, error) {
	req := SignInRequest{
		Email:    email,
		Password: password,
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/api/auth/signin", req)
	if err != nil {
		return nil, fmt.Errorf("signin request failed: %w", err)
	}

	var result AuthResponse
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode signin response: %w", err)
	}

	// Automatically set the auth token for subsequent requests
	c.SetAuthToken(result.Token)

	return &result, nil
}

// SignOut signs out the current user
func (c *Client) SignOut(ctx context.Context) error {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/auth/signout", nil)
	if err != nil {
		return fmt.Errorf("signout request failed: %w", err)
	}

	if err := decodeResponse(resp, nil); err != nil {
		return fmt.Errorf("failed to process signout response: %w", err)
	}

	// Clear the auth token
	c.SetAuthToken("")

	return nil
}

// GetCurrentUser retrieves the currently authenticated user
func (c *Client) GetCurrentUser(ctx context.Context) (*models.User, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/auth/me", nil)
	if err != nil {
		return nil, fmt.Errorf("get current user request failed: %w", err)
	}

	var result models.User
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode current user response: %w", err)
	}

	return &result, nil
}

// RefreshToken refreshes the authentication token
func (c *Client) RefreshToken(ctx context.Context) (*AuthResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/auth/refresh", nil)
	if err != nil {
		return nil, fmt.Errorf("refresh token request failed: %w", err)
	}

	var result AuthResponse
	if err := decodeResponse(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	// Update the auth token
	c.SetAuthToken(result.Token)

	return &result, nil
}
