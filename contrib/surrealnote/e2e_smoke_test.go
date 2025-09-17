//go:build smoke

// Package surrealnote_test provides smoke testing for the SurrealNote application.
//
// DESIGN DECISION: Smoke Tests Focus on Correctness, Not Performance
//
// These smoke tests are designed to discover correctness bugs, not performance issues.
// All tests ALWAYS verify that created data is accessible and consistent.
// For performance testing, create separate benchmark tests using Go's testing.B.
//
// Test Modes:
//
//  1. Standard Test (default):
//     Each virtual user creates their own workspaces, pages, and blocks independently.
//     Use for: Testing normal application usage patterns and data integrity.
//
//  2. Shared Resource Test (SMOKE_SHARED_RESOURCE=true):
//     All virtual users work on the SAME shared workspace and page concurrently.
//     Use for: Testing concurrent access correctness, race conditions, and data consistency
//     under contention. This validates collaborative editing scenarios work correctly.
//
//  3. Scaling Test (SMOKE_ENABLE_SCALING=true):
//     Progressively increases load through defined stages (10->25->50->100 users).
//     Use for: Verifying correctness remains intact as user count increases.
//     NOT for performance measurement - just ensuring the system remains correct at scale.
//
// Data Verification:
//
//	ALL tests verify that every piece of created data can be retrieved and is correct.
//	This is non-negotiable for smoke tests as they exist to catch correctness bugs.
//
// Examples:
//
//	make smoke-test                                    # Standard test, 10 users
//	SMOKE_SHARED_RESOURCE=true make smoke-test        # Test concurrent access correctness
//	SMOKE_ENABLE_SCALING=true make smoke-test         # Test correctness at various scales
package surrealnote_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/client"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/surrealnotetesting"
)

// SmokeTestConfig holds configuration for smoke tests
type SmokeTestConfig struct {
	// Basic configuration
	BaseURL      string
	NumUsers     int           // Number of concurrent virtual users
	TestDuration time.Duration // How long to run the test (0 for scenario-based)
	Timeout      time.Duration // Overall test timeout
	LaunchDelay  time.Duration // Delay between launching users

	// Scaling configuration
	EnableScaling bool          // Whether to progressively scale users
	ScalingStages []int         // User counts for each scaling stage
	StageCooldown time.Duration // Cooldown between scaling stages

	// Workload configuration
	WorkloadType        WorkloadType // Type of workload to run
	SharedResource      bool         // Whether users should work on shared resources (tests concurrent access to same workspace/page)
	RequiredSuccessRate float64      // Minimum success rate (0-100)

	// Scenario configuration
	MinWorkspaces int // Minimum workspaces per user
	MaxWorkspaces int // Maximum workspaces per user
	MinPages      int // Minimum pages per workspace
	MaxPages      int // Maximum pages per workspace
	MinBlocks     int // Minimum blocks per page
	MaxBlocks     int // Maximum blocks per page
}

// WorkloadType defines the type of workload pattern
type WorkloadType string

const (
	WorkloadScenario   WorkloadType = "scenario"   // Full user scenario
	WorkloadContinuous WorkloadType = "continuous" // Continuous operations until timeout
	WorkloadBurst      WorkloadType = "burst"      // Burst operations
)

// DefaultConfig returns a default smoke test configuration
func DefaultConfig() *SmokeTestConfig {
	return &SmokeTestConfig{
		BaseURL:             getEnvOrDefault("SURREALNOTE_URL", "http://localhost:8080"),
		NumUsers:            getEnvOrDefaultInt("SMOKE_NUM_USERS", 10),
		TestDuration:        getEnvOrDefaultDuration("SMOKE_DURATION", 0),
		Timeout:             getEnvOrDefaultDuration("SMOKE_TIMEOUT", 5*time.Minute),
		LaunchDelay:         getEnvOrDefaultDuration("SMOKE_LAUNCH_DELAY", 10*time.Millisecond),
		EnableScaling:       getEnvOrDefaultBool("SMOKE_ENABLE_SCALING", false),
		ScalingStages:       []int{10, 25, 50, 100},
		StageCooldown:       5 * time.Second,
		WorkloadType:        WorkloadType(getEnvOrDefault("SMOKE_WORKLOAD", string(WorkloadScenario))),
		SharedResource:      getEnvOrDefaultBool("SMOKE_SHARED_RESOURCE", false),
		RequiredSuccessRate: getEnvOrDefaultFloat("SMOKE_SUCCESS_RATE", 95.0),
		MinWorkspaces:       1,
		MaxWorkspaces:       3,
		MinPages:            1,
		MaxPages:            5,
		MinBlocks:           1,
		MaxBlocks:           10,
	}
}

// Helper functions for environment variables
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvOrDefaultBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getEnvOrDefaultFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

func getEnvOrDefaultDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// TestE2ESmoke is the main parameterized smoke test
// Run with: go test -tags=smoke -count=1 ./... -run TestE2ESmoke
// Or use: make smoke-test
func TestE2ESmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping smoke test in short mode")
	}

	config := DefaultConfig()
	runSmokeTest(t, config)
}

// runSmokeTest executes the smoke test with the given configuration
func runSmokeTest(t *testing.T, config *SmokeTestConfig) {
	// Validate configuration
	require.Greater(t, config.NumUsers, 0, "NumUsers must be positive")
	require.GreaterOrEqual(t, config.RequiredSuccessRate, 0.0, "RequiredSuccessRate must be >= 0")
	require.LessOrEqual(t, config.RequiredSuccessRate, 100.0, "RequiredSuccessRate must be <= 100")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	// Check server health first
	healthClient := client.NewClient(config.BaseURL)
	health, err := healthClient.Health(ctx)
	require.NoError(t, err, "Server health check failed")
	require.Equal(t, "healthy", health["status"], "Server is not healthy")

	// Print SurrealQL inspection commands
	printSurrealQLCommands(t, config)

	// Log configuration
	t.Logf("=== Smoke Test Configuration ===")
	t.Logf("Base URL: %s", config.BaseURL)
	t.Logf("Number of users: %d", config.NumUsers)
	t.Logf("Workload type: %s", config.WorkloadType)
	t.Logf("Test duration: %v", config.TestDuration)
	t.Logf("Timeout: %v", config.Timeout)
	t.Logf("Required success rate: %.2f%%", config.RequiredSuccessRate)
	t.Logf("Scaling enabled: %v", config.EnableScaling)
	t.Logf("Shared resource: %v", config.SharedResource)

	if config.EnableScaling {
		runScalingTest(t, ctx, config)
	} else if config.SharedResource {
		runSharedResourceTest(t, ctx, config)
	} else {
		runStandardTest(t, ctx, config)
	}
}

// runStandardTest runs a standard smoke test
func runStandardTest(t *testing.T, ctx context.Context, config *SmokeTestConfig) {
	t.Logf("Starting standard smoke test with %d users", config.NumUsers)

	// Metrics
	var successCount, errorCount int64
	var mu sync.Mutex

	// Create virtual users
	virtualUsers := make([]*surrealnotetesting.VirtualUser, config.NumUsers)
	for i := 0; i < config.NumUsers; i++ {
		virtualUsers[i] = surrealnotetesting.NewVirtualUser(i, config.BaseURL)
	}

	// Channel to collect errors
	errChan := make(chan error, config.NumUsers*10)

	// WaitGroup to wait for all users to complete
	var wg sync.WaitGroup

	// Start time for measuring performance
	startTime := time.Now()

	// Launch virtual users concurrently
	for i := 0; i < config.NumUsers; i++ {
		wg.Add(1)
		go func(user *surrealnotetesting.VirtualUser) {
			defer wg.Done()

			// Run the appropriate workload
			var err error
			switch config.WorkloadType {
			case WorkloadScenario:
				err = runScenarioWorkload(ctx, user, config)
			case WorkloadContinuous:
				err = runContinuousWorkload(ctx, user, config, startTime.Add(config.TestDuration))
			case WorkloadBurst:
				err = runBurstWorkload(ctx, user, config)
			default:
				err = fmt.Errorf("unknown workload type: %s", config.WorkloadType)
			}

			mu.Lock()
			if err != nil {
				errorCount++
				errChan <- fmt.Errorf("user %d failed: %w", user.Index, err)
			} else {
				successCount++
			}
			mu.Unlock()
		}(virtualUsers[i])

		// Launch delay
		if config.LaunchDelay > 0 {
			time.Sleep(config.LaunchDelay)
		}
	}

	// Wait for all users to complete or timeout
	doneChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneChan)
	}()

	select {
	case <-doneChan:
		// All users completed
		duration := time.Since(startTime)
		t.Logf("All %d virtual users completed in %v", config.NumUsers, duration)
	case <-ctx.Done():
		t.Fatalf("Test timed out after %v", config.Timeout)
	}

	// Close error channel and collect errors
	close(errChan)
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	// Calculate and verify success rate
	totalOps := successCount + errorCount
	successRate := float64(successCount) / float64(totalOps) * 100

	// Log results
	duration := time.Since(startTime)
	t.Logf("=== Test Results ===")
	t.Logf("Duration: %v", duration)
	t.Logf("Total operations: %d", totalOps)
	t.Logf("Successful: %d", successCount)
	t.Logf("Failed: %d", errorCount)
	t.Logf("Success rate: %.2f%%", successRate)
	t.Logf("Operations per second: %.2f", float64(totalOps)/duration.Seconds())

	// Show sample errors if any
	if len(errors) > 0 {
		maxErrors := 10
		if len(errors) < maxErrors {
			maxErrors = len(errors)
		}
		t.Logf("Sample errors (showing %d of %d):", maxErrors, len(errors))
		for i := 0; i < maxErrors; i++ {
			t.Logf("  Error %d: %v", i+1, errors[i])
		}
	}

	// Verify success rate meets requirement
	require.GreaterOrEqual(t, successRate, config.RequiredSuccessRate,
		"Success rate %.2f%% below required %.2f%%", successRate, config.RequiredSuccessRate)

	// ALWAYS verify data correctness - this is the primary purpose of smoke tests
	// Smoke tests are for correctness bugs, not performance measurement
	if config.WorkloadType == WorkloadScenario {
		t.Log("Performing data verification (always enabled for correctness testing)...")
		verifyUsers := 10
		if config.NumUsers < verifyUsers {
			verifyUsers = config.NumUsers
		}

		for i := 0; i < verifyUsers; i++ {
			user := virtualUsers[i*config.NumUsers/verifyUsers]
			if err := user.VerifyAllData(ctx); err != nil {
				t.Logf("Warning: Verification failed for user %d: %v", user.Index, err)
			}
		}
	}

	t.Log("Smoke test completed successfully!")

	// Print final SurrealQL inspection commands
	printFinalSurrealQLCommands(t, config)
}

// runScalingTest runs a test with progressive scaling
func runScalingTest(t *testing.T, ctx context.Context, config *SmokeTestConfig) {
	t.Log("Starting scaling test with progressive load levels")

	for stageNum, numUsers := range config.ScalingStages {
		t.Run(fmt.Sprintf("Stage_%d_%d_users", stageNum+1, numUsers), func(t *testing.T) {
			// Create stage config
			stageConfig := *config
			stageConfig.NumUsers = numUsers
			stageConfig.EnableScaling = false // Prevent recursion

			// Run the stage
			runStandardTest(t, ctx, &stageConfig)

			// Cool down between stages
			if stageNum < len(config.ScalingStages)-1 {
				t.Logf("Cooling down for %v before next stage...", config.StageCooldown)
				time.Sleep(config.StageCooldown)
			}
		})
	}
}

// runSharedResourceTest simulates multiple users collaborating on the same workspace and page.
// This tests the system's ability to handle concurrent modifications to shared resources,
// which is critical for collaborative features like real-time editing.
// All users will create blocks in the same page simultaneously, testing for:
// - Race conditions in data access
// - Concurrent write handling
// - Data consistency under contention
// - Performance under collaborative load
func runSharedResourceTest(t *testing.T, ctx context.Context, config *SmokeTestConfig) {
	t.Log("Starting shared resource test - simulating collaborative editing")

	// Create a shared workspace owner (user 0)
	owner := surrealnotetesting.NewVirtualUser(0, config.BaseURL)
	err := owner.SignUp(ctx)
	require.NoError(t, err, "Owner signup failed")

	// Create shared resources
	workspace, err := owner.CreateWorkspace(ctx, "Shared Workspace")
	require.NoError(t, err, "Failed to create shared workspace")

	page, err := owner.CreatePageInWorkspace(ctx, workspace.ID, "Shared Page")
	require.NoError(t, err, "Failed to create shared page")

	t.Logf("Created shared workspace %s and page %s", workspace.ID, page.ID)

	// Metrics
	var operationCount int64
	var errorCount int64
	var mu sync.Mutex

	// Create and launch worker users
	var wg sync.WaitGroup
	startTime := time.Now()
	deadline := startTime.Add(config.TestDuration)

	for i := 1; i <= config.NumUsers; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()

			user := surrealnotetesting.NewVirtualUser(userID, config.BaseURL)

			// Sign up
			if err := user.SignUp(ctx); err != nil {
				mu.Lock()
				errorCount++
				mu.Unlock()
				t.Logf("User %d signup failed: %v", userID, err)
				return
			}

			// Perform operations on shared resources
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return
				default:
					// Create blocks in the shared page
					content := fmt.Sprintf("Block from user %d at %s", userID, time.Now().Format("15:04:05"))
					_, err := user.CreateBlockInPage(ctx, page.ID, models.BlockTypeText, content, userID)

					mu.Lock()
					if err != nil {
						errorCount++
					} else {
						operationCount++
					}
					mu.Unlock()

					// Small delay between operations
					time.Sleep(100 * time.Millisecond)
				}
			}
		}(i)

		// Launch delay
		if config.LaunchDelay > 0 {
			time.Sleep(config.LaunchDelay)
		}
	}

	// Wait for all workers to complete
	wg.Wait()

	// Calculate metrics
	duration := time.Since(startTime)
	totalOps := operationCount + errorCount
	successRate := float64(operationCount) / float64(totalOps) * 100

	t.Logf("=== Shared Resource Test Results ===")
	t.Logf("Duration: %v", duration)
	t.Logf("Total operations: %d", operationCount)
	t.Logf("Errors: %d", errorCount)
	t.Logf("Success rate: %.2f%%", successRate)
	t.Logf("Operations per second: %.2f", float64(operationCount)/duration.Seconds())

	// Verify final state
	blocks, err := owner.Client.ListBlocks(ctx, page.ID)
	require.NoError(t, err, "Failed to list blocks")
	t.Logf("Final block count: %d", len(blocks))

	// Verify success rate
	require.GreaterOrEqual(t, successRate, config.RequiredSuccessRate,
		"Success rate %.2f%% below required %.2f%%", successRate, config.RequiredSuccessRate)

	// Print final commands
	printFinalSurrealQLCommands(t, config)
}

// Workload functions

func runScenarioWorkload(ctx context.Context, user *surrealnotetesting.VirtualUser, config *SmokeTestConfig) error {
	// Run the full scenario
	return user.RunScenario(ctx)
}

func runContinuousWorkload(ctx context.Context, user *surrealnotetesting.VirtualUser, config *SmokeTestConfig, deadline time.Time) error {
	// Sign up first
	if err := user.SignUp(ctx); err != nil {
		return err
	}

	// Create initial workspace
	workspace, err := user.CreateWorkspace(ctx, fmt.Sprintf("Workspace %d", user.Index))
	if err != nil {
		return err
	}

	// Continuous operations until deadline
	pageCount := 0
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Create a page
			page, err := user.CreatePageInWorkspace(ctx, workspace.ID, fmt.Sprintf("Page %d-%d", user.Index, pageCount))
			if err != nil {
				return err
			}
			pageCount++

			// Create some blocks
			for i := 0; i < 5; i++ {
				_, err := user.CreateBlockInPage(ctx, page.ID, models.BlockTypeText,
					fmt.Sprintf("Block %d-%d-%d", user.Index, pageCount, i), i)
				if err != nil {
					return err
				}
			}

			// Small delay
			time.Sleep(10 * time.Millisecond)
		}
	}

	return nil
}

func runBurstWorkload(ctx context.Context, user *surrealnotetesting.VirtualUser, config *SmokeTestConfig) error {
	// Sign up
	if err := user.SignUp(ctx); err != nil {
		return err
	}

	// Create bursts of activity
	for burst := 0; burst < 3; burst++ {
		// Create workspace
		workspace, err := user.CreateWorkspace(ctx, fmt.Sprintf("Burst Workspace %d-%d", user.Index, burst))
		if err != nil {
			return err
		}

		// Rapid page and block creation
		for i := 0; i < 10; i++ {
			page, err := user.CreatePageInWorkspace(ctx, workspace.ID, fmt.Sprintf("Burst Page %d-%d-%d", user.Index, burst, i))
			if err != nil {
				return err
			}

			// Create blocks rapidly
			for j := 0; j < 20; j++ {
				_, err := user.CreateBlockInPage(ctx, page.ID, models.BlockTypeText,
					fmt.Sprintf("Burst Block %d-%d-%d-%d", user.Index, burst, i, j), j)
				if err != nil {
					return err
				}
			}
		}

		// Cool down between bursts
		time.Sleep(1 * time.Second)
	}

	return nil
}

// printSurrealQLCommands prints useful SurrealQL commands for inspection
func printSurrealQLCommands(t *testing.T, config *SmokeTestConfig) {
	separator := strings.Repeat("=", 60)

	// Determine namespace and database from config or use defaults
	ns := getEnvOrDefault("SURREALDB_NS", "surrealnote")
	db := getEnvOrDefault("SURREALDB_DB", "surrealnote")
	url := getEnvOrDefault("SURREALDB_URL", "ws://localhost:8000")

	// Print connection command separately
	t.Logf(`
%s
To inspect the test data, use these SurrealQL commands:
%s

# Connect to SurrealDB:
surreal sql --conn %s --ns %s --db %s
`, separator, separator, url, ns, db)

	// Print all queries as a single block for easy copy-paste
	t.Logf(`
# Then run these queries to inspect the data:

-- Count all records by table
SELECT count() AS total FROM users GROUP ALL;
SELECT count() AS total FROM workspaces GROUP ALL;
SELECT count() AS total FROM pages GROUP ALL;
SELECT count() AS total FROM blocks GROUP ALL;
SELECT count() AS total FROM comments GROUP ALL;

-- List recent users (last 5)
SELECT id, name, email, created_at FROM users ORDER BY created_at DESC LIMIT 5;

-- Show workspace distribution per user
SELECT owner_id, count() AS workspace_count FROM workspaces GROUP BY owner_id;

-- Show page distribution per workspace
SELECT workspace_id, count() AS page_count FROM pages GROUP BY workspace_id LIMIT 10;

-- Show block distribution per page (sample)
SELECT page_id, count() AS block_count FROM blocks GROUP BY page_id LIMIT 10;

-- Show activity timeline (last 20 operations)
SELECT * FROM (
  SELECT id, 'user' AS type, created_at FROM users
  UNION
  SELECT id, 'workspace' AS type, created_at FROM workspaces
  UNION
  SELECT id, 'page' AS type, created_at FROM pages
  UNION
  SELECT id, 'block' AS type, created_at FROM blocks
) ORDER BY created_at DESC LIMIT 20;

-- Check for any orphaned records
SELECT * FROM pages WHERE workspace_id NOT IN (SELECT id FROM workspaces);
SELECT * FROM blocks WHERE page_id NOT IN (SELECT id FROM pages);

%s`, separator)
}

// printFinalSurrealQLCommands prints summary queries after test completion
func printFinalSurrealQLCommands(t *testing.T, config *SmokeTestConfig) {
	ns := getEnvOrDefault("SURREALDB_NS", "surrealnote")
	db := getEnvOrDefault("SURREALDB_DB", "surrealnote")
	url := getEnvOrDefault("SURREALDB_URL", "ws://localhost:8000")

	separator := strings.Repeat("=", 60)

	// Print all final commands as a single block for easy copy-paste
	t.Logf(`
%s
TEST COMPLETED - Final inspection commands:
%s

surreal sql --conn %s --ns %s --db %s

-- Get complete statistics
LET $stats = {
  users: (SELECT count() FROM users GROUP ALL),
  workspaces: (SELECT count() FROM workspaces GROUP ALL),
  pages: (SELECT count() FROM pages GROUP ALL),
  blocks: (SELECT count() FROM blocks GROUP ALL),
  comments: (SELECT count() FROM comments GROUP ALL)
};
RETURN $stats;

-- Find the most active user
SELECT owner_id, count() AS items FROM (
  SELECT owner_id FROM workspaces
) GROUP BY owner_id ORDER BY items DESC LIMIT 1;

-- Find pages with most blocks
SELECT page_id, count() AS block_count FROM blocks
GROUP BY page_id ORDER BY block_count DESC LIMIT 5;

-- Cleanup command (if needed)
-- WARNING: This will delete all test data!
-- DELETE users, workspaces, pages, blocks, comments;

%s`, separator, separator, url, ns, db, separator)
}
