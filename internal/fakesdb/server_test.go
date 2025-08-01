package fakesdb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	surrealdb "github.com/surrealdb/surrealdb.go"
)

func TestServer(t *testing.T) {
	server := NewServer("127.0.0.1:0")

	server.AddStubResponse(SimpleStubResponse("query", map[string]interface{}{
		"result": []interface{}{
			map[string]interface{}{
				"id":   "user:1",
				"name": "John Doe",
			},
		},
	}))

	require.NoError(t, server.Start())
	assert.NotEmpty(t, server.Address())
	require.NoError(t, server.Stop())
}

func TestAuthenticationFlow(t *testing.T) {
	ctx := context.Background()

	t.Run("SignIn flow", func(t *testing.T) {
		// Create and start server
		server := NewServer("127.0.0.1:0")
		server.TokenSignIn = "test_token_signin"

		server.AddStubResponse(SimpleStubResponse("select", map[string]any{
			"id":   "test:1",
			"name": "Test Record",
		}))

		err := server.Start()
		require.NoError(t, err)
		defer func() {
			if stopErr := server.Stop(); stopErr != nil {
				t.Fatalf("Failed to stop server: %v", stopErr)
			}
		}()

		// Connect to server
		db, err := surrealdb.Connect(ctx, "ws://"+server.Address())
		require.NoError(t, err)
		defer db.Close(ctx)

		// Try to sign in without namespace/database - should fail
		_, err = db.SignIn(ctx, surrealdb.Auth{
			Username: "root",
			Password: "root",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Specify a namespace and database")

		// Query should fail before use and sign in
		_, err = surrealdb.Select[map[string]any](ctx, db, "test:1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "There was a problem with the database: There was a problem with authentication: Session not found")

		// Set namespace and database
		err = db.Use(ctx, "test", "test")
		require.NoError(t, err)

		// Query should fail before sign in
		_, err = surrealdb.Select[map[string]any](ctx, db, "test:1")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "There was a problem with the database: There was a problem with authentication: Not signed in")

		// SignIn should succeed
		token, err := db.SignIn(ctx, surrealdb.Auth{
			Username: "root",
			Password: "root",
		})
		require.NoError(t, err)
		require.Equal(t, server.TokenSignIn, token)

		// Query should work after signin
		_, err = surrealdb.Select[map[string]any](ctx, db, "test:1")
		assert.NoError(t, err)
	})

	t.Run("Authenticate flow", func(t *testing.T) {
		// Create and start server
		server := NewServer("127.0.0.1:0")
		server.TokenSignIn = "test_token_signin"

		// Add stub responses - don't stub signin/authenticate to test the built-in behavior
		server.AddStubResponse(SimpleStubResponse("let", nil))
		server.AddStubResponse(SimpleStubResponse("select", map[string]any{
			"id":   "test:1",
			"name": "Test Record",
		}))

		err := server.Start()
		require.NoError(t, err)
		defer func() {
			if stopErr := server.Stop(); stopErr != nil {
				t.Fatalf("Failed to stop server: %v", stopErr)
			}
		}()

		// Connect first client and sign in
		db1, err := surrealdb.Connect(ctx, "ws://"+server.Address())
		require.NoError(t, err)
		defer db1.Close(ctx)

		err = db1.Use(ctx, "test", "test")
		require.NoError(t, err)

		token, err := db1.SignIn(ctx, surrealdb.Auth{
			Username: "user1",
			Password: "pass1",
		})
		require.NoError(t, err)
		require.Equal(t, server.TokenSignIn, token)

		// Connect second client and authenticate with token
		db2, err := surrealdb.Connect(ctx, "ws://"+server.Address())
		require.NoError(t, err)
		defer db2.Close(ctx)

		// Should fail without namespace/database
		err = db2.Authenticate(ctx, token)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Specify a namespace and database")

		// Set namespace and database
		err = db2.Use(ctx, "test", "test")
		require.NoError(t, err)

		// Authenticate with token
		err = db2.Authenticate(ctx, token)
		require.NoError(t, err)

		// Query should work after authentication
		_, err = surrealdb.Select[map[string]any, string](ctx, db2, "test:1")
		assert.NoError(t, err)
	})

	t.Run("Token expiration", func(t *testing.T) {
		// Create and start server
		server := NewServer("127.0.0.1:0")
		server.TokenSignUp = "test_token_signup"

		// Add stub responses - don't stub authenticate to test the built-in behavior
		server.AddStubResponse(SimpleStubResponse("let", nil))
		server.AddStubResponse(SimpleStubResponse("select", map[string]any{
			"id":   "test:1",
			"name": "Test Record",
		}))

		err := server.Start()
		require.NoError(t, err)
		defer func() {
			if stopErr := server.Stop(); stopErr != nil {
				t.Fatalf("Failed to stop server: %v", stopErr)
			}
		}()

		// Generate token with short expiration
		token, err := server.GenerateTokenWithExpiration("testuser", "mytoken", 100*time.Millisecond)
		require.NoError(t, err)
		require.Equal(t, "mytoken", token)

		// Connect and authenticate
		db, err := surrealdb.Connect(ctx, "ws://"+server.Address())
		require.NoError(t, err)
		defer db.Close(ctx)

		err = db.Use(ctx, "test", "test")
		require.NoError(t, err)

		err = db.Authenticate(ctx, token)
		require.NoError(t, err)

		// Query should work
		_, err = surrealdb.Select[map[string]any, string](ctx, db, "test:1")
		assert.NoError(t, err)

		// Wait for token to expire
		time.Sleep(150 * time.Millisecond)

		// Query should fail with expired token
		_, err = surrealdb.Select[map[string]any, string](ctx, db, "test:2")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "There was a problem with the database: There was a problem with authentication: Expired")
	})

	t.Run("Query without authentication", func(t *testing.T) {
		// Create and start server
		server := NewServer("127.0.0.1:0")

		err := server.Start()
		require.NoError(t, err)
		defer func() {
			if stopErr := server.Stop(); stopErr != nil {
				t.Fatalf("Failed to stop server: %v", stopErr)
			}
		}()

		// Connect to server
		db, err := surrealdb.Connect(ctx, "ws://"+server.Address())
		require.NoError(t, err)
		defer db.Close(ctx)

		// Set namespace and database
		err = db.Use(ctx, "test", "test")
		require.NoError(t, err)

		// Query should fail without authentication
		_, err = surrealdb.Select[map[string]any, string](ctx, db, "test:1")
		require.Error(t, err)
		// Should fail because there's no authenticated session
		assert.Contains(t, err.Error(), `There was a problem with the database: There was a problem with authentication: Not signed in`)
	})
}
