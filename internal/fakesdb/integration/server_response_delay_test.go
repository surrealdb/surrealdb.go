package tests

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/internal/fakesdb"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/connection/gorillaws"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// TestServerFailureResponseDelay demonstrates how simulated server delays work
func TestServerFailureResponseDelay(t *testing.T) {
	// Create fake SurrealDB server
	server := fakesdb.NewServer("127.0.0.1:0")
	server.TokenSignIn = "test_token_signin"

	// Count requests to introduce delays
	requestCount := 0
	server.AddStubResponse(fakesdb.StubResponse{
		Matcher: fakesdb.MatchMethodWithParams("select", func(params []any) bool {
			requestCount++
			t.Logf("Select request #%d", requestCount)
			// Delay every 3rd request
			return requestCount%3 == 0
		}),
		Response: map[string]any{
			"id": cbor.Tag{Number: 8, Content: []any{"test", "1"}},
		},
		Failures: []fakesdb.FailureConfig{
			{
				Type:        fakesdb.FailureResponseDelay,
				Probability: 1.0,
				MinDelay:    200 * time.Millisecond, // Longer than request timeout
				MaxDelay:    300 * time.Millisecond,
			},
		},
	})

	// Normal response
	server.AddStubResponse(fakesdb.SimpleStubResponse("select", map[string]any{
		"id": cbor.Tag{Number: 8, Content: []any{"test", "1"}},
	}))

	// Start server
	err := server.Start()
	require.NoError(t, err)
	defer func() {
		if stopErr := server.Stop(); stopErr != nil {
			t.Fatalf("Failed to stop server: %v", stopErr)
		}
	}()

	wsURL := "ws://" + server.Address()

	u, err := url.ParseRequestURI(wsURL)
	require.NoError(t, err)

	p := connection.NewConfig(u)

	ws := gorillaws.New(p).
		SetTimeOut(100 * time.Millisecond) // Short request timeout

	db, err := surrealdb.FromConnection(context.Background(), ws)
	require.NoError(t, err)
	defer db.Close(context.Background())

	// Setup
	err = db.Use(context.Background(), "test", "test")
	require.NoError(t, err)

	token, err := db.SignIn(context.Background(), &surrealdb.Auth{
		Username: "root",
		Password: "root",
	})
	require.NoError(t, err)
	require.Equal(t, server.TokenSignIn, token)

	err = db.Authenticate(context.Background(), token)
	require.NoError(t, err)

	type TestRecord struct {
		ID models.RecordID `json:"id"`
	}

	// Run multiple selects
	successCount := 0
	timeoutCount := 0

	for i := 0; i < 10; i++ {
		result, err := surrealdb.Select[TestRecord](
			context.Background(),
			db,
			models.NewRecordID("test", "1"),
		)

		if err != nil {
			if err.Error() == "context deadline exceeded" {
				timeoutCount++
				t.Logf("Request %d timed out (expected for every 3rd)", i+1)
			} else {
				t.Logf("Request %d failed with: %v", i+1, err)
			}
		} else {
			successCount++
			assert.NotNil(t, result)
			assert.Equal(t, "test", result.ID.Table)
		}
	}

	// We should have some timeouts (roughly 3 out of 10)
	assert.Greater(t, timeoutCount, 0, "Should have some timeouts")
	assert.Greater(t, successCount, 5, "Should have mostly successes")

	t.Logf("Results: %d successes, %d timeouts", successCount, timeoutCount)
}
