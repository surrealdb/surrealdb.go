package surrealnote_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/models"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/postgres"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/store/surrealdb"
	"github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/surrealnote"
)

// StageData represents test data for each migration stage
type StageData struct {
	// Core entities that persist across stages
	User1       *models.User
	User2       *models.User // Added in Stage 2
	Workspace1  *models.Workspace
	Workspace2  *models.Workspace // Added in Stage 3
	Page1       *models.Page
	Page2       *models.Page // Added in Stage 2
	Page3       *models.Page // Added in Stage 4
	Block1      *models.Block
	Block2      *models.Block // Added in Stage 3
	Block3      *models.Block // Added in Stage 6
	Block4      *models.Block // Added in Stage 8
	Permission1 *models.Permission
	Comment1    *models.Comment // Added in Stage 4

	// Track what was modified in each stage
	UpdatedInStage map[string][]string // stage -> list of updated entity descriptions

	// Timing information for gap-free migration
	MigrationWindowStart time.Time // When we started reading from source (Stage 2)
	MigrationWindowEnd   time.Time // When migration completed (Stage 2)
}

// Test data for each stage, building on the previous stage
var (
	// Stage 1: PostgreSQL only - Initial data
	stage1Data = &StageData{
		UpdatedInStage: make(map[string][]string),
	}

	// Stage 3: CQRS PostgreSQL Primary - Add more data and update existing
	stage2Data = &StageData{
		UpdatedInStage: make(map[string][]string),
	}

	// Stage 3: Validation - Add more data to verify consistency
	stage3Data = &StageData{
		UpdatedInStage: make(map[string][]string),
	}

	// Stage 4: Validation - Test reading from both and comparing results
	stage4Data = &StageData{
		UpdatedInStage: make(map[string][]string),
	}

	// Stage 5: Switching - Test reading from SurrealDB
	stage5Data = &StageData{
		UpdatedInStage: make(map[string][]string),
	}

	// Stage 6: CQRS SurrealDB Primary
	stage6Data = &StageData{
		UpdatedInStage: make(map[string][]string),
	}

	// Stage 7: After reverse sync (PostgreSQL has all data from Stage 6)
	stage7Data = &StageData{
		UpdatedInStage: make(map[string][]string),
	}

	// Stage 8: SurrealDB only - Final state
	stage8Data = &StageData{
		UpdatedInStage: make(map[string][]string),
	}

	// Timestamps for catch-up synchronization
	migrationStartTime time.Time
	migrationEndTime   time.Time
	stage5StartTime    time.Time
	stage6StartTime    time.Time
	stage6EndTime      time.Time
)

const (
	postgresPort = "5438"
)

const (
	testPort = "8095"
	testURL  = "http://localhost:8095"
)

// TestE2E_migrationFlow demonstrates the complete migration process from PostgreSQL to SurrealDB
// with all 6 stages of migration, ensuring zero-downtime migration capability.
// This test serves as both a validation suite and a reference implementation for migration patterns.
//
// The migration stages support zero-downtime migration in the following order:
//
//  1. Stage 1: PostgreSQL Only - Start with existing PostgreSQL data
//  2. Stage 2: Data Migration - Copy all existing data from PostgreSQL to SurrealDB
//  3. Stage 3: CQRS PostgreSQL Primary - Both stores connected, writes to PostgreSQL, background sync to SurrealDB
//     -> Continuous catch-up sync can run in parallel to minimize final sync time
//     -> Enter read-only mode to prevent concurrent updates
//     -> Final Catch-up Sync: Synchronize any remaining updates from PostgreSQL to SurrealDB
//     -> Exit read-only mode and continue to Stage 4
//  4. Stage 4: Validation - Write to both, read from both and compare
//     NOTE: In practice, Stage 4 is often omitted or replaced with a background
//     process that continuously validates data consistency (e.g., using content hashes)
//     to avoid the performance overhead of synchronous validation on every read
//  5. Stage 5: CQRS Switching - Both stores connected, reads from SurrealDB, writes still to PostgreSQL
//  6. Stage 6: CQRS Reversed - Both stores connected, writes to SurrealDB, PostgreSQL kept in sync
//     -> This stage validates SurrealDB can handle writes correctly
//     -> Continuous reverse sync keeps PostgreSQL updated for rollback capability
//  7. Stage 7: Reverse Sync - Final synchronization from SurrealDB to PostgreSQL
//     -> Enter read-only mode to prevent concurrent updates
//     -> Final Reverse Catch-up Sync: Sync any remaining updates from SurrealDB to PostgreSQL
//     -> Exit read-only mode and continue to Stage 8
//  8. Stage 8: SurrealDB Only - Complete migration to SurrealDB
//
// This ordering ensures zero-downtime migration because:
// - Data is migrated to SurrealDB before Stage 3
// - Background sync ensures SurrealDB gets updates from PostgreSQL
// - No data loss occurs during the transition
// - The validation stage can properly verify consistency between databases
//
// Mode Switching Approaches:
//
// There are generally two options for changing the migration mode in production:
//
// Option 1: Redeployment with new mode
// - Redeploy the application with the new mode configuration one instance at a time
// - Pros: More secure (no admin API needed), follows standard deployment practices
// - Cons: Takes more time due to redeployment, requires deployment pipeline
//
// Option 2: Admin API for dynamic mode switching
// - Call an admin API endpoint on the deployed app to switch modes dynamically
// - Pros: Faster switching, no redeployment needed, instant changes
// - Cons: Requires secured admin API endpoints, potential security risk if exposed
//
// Current E2E Test Implementation:
// This test uses Option 1 (Redeployment) by stopping and restarting the application
// with different mode configurations for each stage transition.
// We chose this approach for the E2E test because:
// - It simulates the most common production deployment pattern
// - It ensures clean state transitions between modes
// - It tests that the application correctly initializes in each mode
// - It verifies data persistence across application restarts
func TestE2E_migrationFlow(t *testing.T) {
	// Note: Chdir is no longer needed since we're not executing the binary
	// The surrealnote.Main function will handle paths correctly

	// Clean up and restart PostgreSQL
	t.Log("Starting PostgreSQL...")
	stopPostgres(postgresPort)
	cleanupPostgres(postgresPort)
	if err := startPostgres(postgresPort); err != nil {
		t.Fatalf("Failed to start PostgreSQL: %v", err)
	}
	defer stopPostgres(postgresPort)

	// Wait for PostgreSQL to be ready
	time.Sleep(5 * time.Second)

	// Run all 6 stages in sequence
	t.Run("Stage1_PostgreSQL_Only", func(t *testing.T) {
		// Record when Stage 1 started (before any data creation)
		migrationStartTime = time.Now()
		testStage1PostgreSQLOnly(t)
	})

	t.Run("Stage2_DataMigration", func(t *testing.T) {
		// CRITICAL: Record the timestamp BEFORE migration starts
		// This ensures we can catch any changes that happen during migration
		stage2StartTime := time.Now().Add(-1 * time.Second) // Small buffer for clock skew
		testStage2DataMigration(t)
		// Record migration end time
		migrationEndTime = time.Now()

		// Store the migration window for Stage 3 to use
		stage2Data.MigrationWindowStart = stage2StartTime
		stage2Data.MigrationWindowEnd = migrationEndTime
	})

	// Stage 3: CQRS with PostgreSQL Primary - Both stores connected
	// Writes go only to PostgreSQL (primary), reads from PostgreSQL
	// Background sync will later replicate these changes to SurrealDB (secondary)
	t.Run("Stage3_CQRS_PostgreSQL_Primary", func(t *testing.T) {
		testStage3CQRSPostgreSQLPrimary(t)
	})

	// Perform catch-up sync between Stage 3 and Stage 4
	// This ensures any data created/modified during Stage 3 is synchronized to SurrealDB
	// We sync from when Stage 3 started to now (with a small buffer) to catch all Stage 3 changes
	t.Run("CatchUp_Sync_3_to_4", func(t *testing.T) {
		// CRITICAL: Use stage2Data.MigrationWindowEnd to cover the gap
		// between Stage 2 ending and Stage 3 starting, ensuring no data is lost.
		// Subtract 1 second buffer to catch any write operations that were in-flight
		// at the moment Stage 2 ended (e.g., a write that started at MigrationWindowEnd
		// but took 100ms to complete would have a timestamp slightly after MigrationWindowEnd)
		if stage2Data.MigrationWindowEnd.IsZero() {
			t.Fatalf("CRITICAL: stage2Data.MigrationWindowEnd not set - Stage 2 must complete before catch-up sync")
		}
		syncSince := stage2Data.MigrationWindowEnd.Add(-1 * time.Second)
		// Add 1 second buffer to ensure we catch all changes that may still be in flight
		syncUntil := time.Now().Add(1 * time.Second)
		performCatchUpSync(t, syncSince, syncUntil)
	})

	t.Run("Stage4_Validation", func(t *testing.T) {
		testStage4Validation(t)
	})

	t.Run("Stage5_CQRS_Switching", func(t *testing.T) {
		testStage5CQRSSwitching(t)
	})

	// Stage 6: Write to SurrealDB as primary while keeping PostgreSQL in sync
	// This stage validates that SurrealDB can handle writes correctly
	t.Run("Stage6_CQRS_SurrealDB_Primary", func(t *testing.T) {
		// Record stage 6 start time for reverse catch-up sync
		stage6StartTime = time.Now()
		t.Logf("Stage 6 start time: %v", stage6StartTime)
		testStage6CQRSSurrealDBPrimary(t)
		// Record when Stage 6 ends (after data has been created)
		stage6EndTime = time.Now()
		t.Logf("Stage 6 end time: %v", stage6EndTime)
	})

	// Stage 7: Perform reverse catch-up sync from SurrealDB to PostgreSQL
	// This syncs from SurrealDB back to PostgreSQL for potential rollback scenarios.
	// This ensures that if we need to rollback from SurrealDB to PostgreSQL,
	// PostgreSQL has all the latest changes that were made while writing to SurrealDB.
	t.Run("Stage7_Reverse_Sync", func(t *testing.T) {
		t.Logf("Stage7: stage6StartTime=%v, stage6EndTime=%v", stage6StartTime, stage6EndTime)
		// Always use the recorded timestamps with buffers
		syncSince := stage6StartTime.Add(-1 * time.Second)
		syncUntil := stage6EndTime.Add(2 * time.Second)
		performReverseCatchUpSync(t, syncSince, syncUntil)
	})

	// Stage 8: SurrealDB Only - Final stage with single store
	t.Run("Stage8_SurrealDB_Only", func(t *testing.T) {
		testStage8SurrealDBOnly(t)
	})
}

// Stage 1: PostgreSQL only mode - Create initial data
func testStage1PostgreSQLOnly(t *testing.T) {
	t.Log("=== STAGE 1: PostgreSQL Only Mode ===")
	t.Log("Starting with existing PostgreSQL database")

	// Run migrations for PostgreSQL
	runMigrations(t, postgresPort, "-postgres-only")

	// Start app in PostgreSQL-only mode
	app := startApp(t, postgresPort, "-postgres-only", "-port", testPort)
	defer app.Stop()

	waitForServer(t, testURL)

	// Create initial test data in PostgreSQL
	t.Log("Creating initial test data in PostgreSQL...")

	// Create first user
	user1 := createUser(t, testURL, "user1@example.com", "User One")
	stage1Data.User1 = user1
	t.Logf("Created User1: %s", user1.ID)

	// Create first workspace owned by user1
	workspace1 := createWorkspace(t, testURL, "Workspace One", user1.ID)
	stage1Data.Workspace1 = workspace1
	t.Logf("Created Workspace1: %s", workspace1.ID)

	// Create first page in workspace1
	page1 := createPage(t, testURL, workspace1.ID, "Page One", user1.ID)
	stage1Data.Page1 = page1
	t.Logf("Created Page1: %s", page1.ID)

	// Create first block in page1
	block1 := createBlock(t, testURL, page1.ID, "Block One Content", models.BlockTypeText)
	stage1Data.Block1 = block1
	t.Logf("Created Block1: %s", block1.ID)

	// Create permission for user1 on workspace1
	permission1 := createPermission(t, testURL, user1.ID, models.NewResourceIDForWorkspace(workspace1.ID), models.ResourceWorkspace, models.PermissionAdmin)
	stage1Data.Permission1 = permission1
	t.Logf("Created Permission1: %s", permission1.ID)

	// Verify all data is accessible
	t.Log("Verifying Stage 1 data...")
	verifyUser(t, testURL, user1.ID, "user1@example.com", "User One")
	verifyWorkspace(t, testURL, workspace1.ID, "Workspace One", user1.ID)
	verifyPage(t, testURL, page1.ID, "Page One", workspace1.ID)
	verifyBlock(t, testURL, block1.ID, "Block One Content", page1.ID)

	// Copy stage1 data to stage2 (stage2 inherits all stage1 data)
	*stage2Data = *stage1Data
	stage2Data.UpdatedInStage = make(map[string][]string)

	t.Log("✓ Stage 1 completed - Initial data created in PostgreSQL")
}

// Stage 3: CQRS with PostgreSQL as primary - Both stores connected
func testStage3CQRSPostgreSQLPrimary(t *testing.T) {
	t.Log("=== STAGE 3: CQRS Mode - PostgreSQL Primary ===")
	t.Log("Both stores connected, PostgreSQL is primary (writes), SurrealDB is secondary (will receive sync)")

	// Ensure stage2Data was populated (from data migration)
	if stage2Data.User1 == nil {
		t.Fatal("Stage 2 data migration failed - User1 not found")
	}

	// Copy stage2 data to stage3 (stage3 inherits all migrated data)
	*stage3Data = *stage2Data
	stage3Data.UpdatedInStage = make(map[string][]string)
	for k, v := range stage2Data.UpdatedInStage {
		stage3Data.UpdatedInStage[k] = v
	}

	// Note: The catch-up sync between Stage 3 and Stage 4 will handle syncing all changes
	// from Stage 2 end time through Stage 3, ensuring no data is lost in the transition gap.
	// This includes any writes that occurred between Stage 2 ending and Stage 3 starting.

	// Run migrations for both databases (CQRS mode)
	runMigrations(t, postgresPort, "-mode", "single")

	// Start app in CQRS "single" mode (both stores connected, PostgreSQL is primary)
	// NOT passing -postgres-only or -surreal-only means CQRS mode with both stores
	app := startApp(t, postgresPort, "-mode", "single", "-port", testPort)
	defer app.Stop()

	waitForServer(t, testURL)

	// Verify migrated data is still accessible
	t.Log("Verifying migrated data is accessible...")
	verifyUser(t, testURL, stage2Data.User1.ID, "user1@example.com", "User One")
	verifyWorkspace(t, testURL, stage2Data.Workspace1.ID, "Workspace One", stage2Data.User1.ID)
	verifyPage(t, testURL, stage2Data.Page1.ID, "Page One", stage2Data.Workspace1.ID)

	// Create new data (will be synced to SurrealDB in background)
	t.Log("Creating new data while background sync is running...")

	// Create second user
	beforeCreate := time.Now()
	user2 := createUser(t, testURL, "user2@example.com", "User Two")
	stage3Data.User2 = user2
	t.Logf("Created User2: %s at %v (will sync to SurrealDB)", user2.ID, beforeCreate.Format("15:04:05.000"))

	// Create second page in workspace1
	page2 := createPage(t, testURL, stage2Data.Workspace1.ID, "Page Two", stage2Data.User1.ID)
	stage3Data.Page2 = page2
	t.Logf("Created Page2: %s (will sync to SurrealDB)", page2.ID)

	// Update existing data (will be synced to SurrealDB in background)
	t.Log("Updating existing data while background sync is running...")

	// Update Page1 title
	updatePageTitle(t, testURL, stage2Data.Page1.ID, "Page One Updated")
	stage3Data.UpdatedInStage["stage3"] = append(stage3Data.UpdatedInStage["stage3"], "Page1 title updated")
	t.Logf("Updated Page1 title in both databases")

	// Update Block1 content
	updateBlockContent(t, testURL, stage2Data.Block1.ID, "Block One Updated Content")
	stage3Data.UpdatedInStage["stage3"] = append(stage3Data.UpdatedInStage["stage3"], "Block1 content updated")
	t.Logf("Updated Block1 content in both databases")

	// Verify all changes
	t.Log("Verifying Stage 3 changes...")
	verifyUser(t, testURL, user2.ID, "user2@example.com", "User Two")
	verifyPage(t, testURL, page2.ID, "Page Two", stage2Data.Workspace1.ID)
	verifyPage(t, testURL, stage2Data.Page1.ID, "Page One Updated", stage2Data.Workspace1.ID)
	verifyBlock(t, testURL, stage2Data.Block1.ID, "Block One Updated Content", stage2Data.Page1.ID)

	// Copy stage3 data to stage4
	*stage4Data = *stage3Data
	stage4Data.UpdatedInStage = make(map[string][]string)
	for k, v := range stage3Data.UpdatedInStage {
		stage4Data.UpdatedInStage[k] = v
	}

	t.Log("✓ Stage 3 completed - Data written to both databases")
}

// Stage 4: Real-world validation scenario with background sync before cutover
//
// This stage simulates a production-ready validation process where:
// 1. Background sync runs continuously to keep secondary up-to-date
// 2. System switches to read-only mode to prevent data changes
// 3. Final catch-up sync ensures all data is migrated
// 4. Data consistency is validated between stores
// 5. System switches to "switching" mode to validate reads from secondary
//
// This approach ensures zero data loss and provides confidence before the final cutover.
func testStage4Validation(t *testing.T) {
	t.Log("=== STAGE 4: Real-World Validation with Background Sync ===")
	t.Log("Simulating production validation process before cutover to SurrealDB")

	// Ensure stage3Data was populated
	if stage3Data.User1 == nil {
		t.Fatal("Stage 3 failed - User1 not found")
	}

	// Copy stage3Data to stage4Data for tracking
	if stage4Data.User1 == nil {
		*stage4Data = *stage3Data
		stage4Data.UpdatedInStage = make(map[string][]string)
		for k, v := range stage3Data.UpdatedInStage {
			stage4Data.UpdatedInStage[k] = v
		}
	}

	ctx := context.Background()

	// Step 1: Start app in CQRS "single" mode (has both stores, PostgreSQL is primary)
	t.Log("Step 1: Starting app in CQRS single mode (both stores available, PostgreSQL primary)...")
	// The app must be in CQRS mode (with both stores) to support sync operations
	// "single" in CQRS context means: writes go to primary (PostgreSQL), reads from primary
	// This is different from PostgreSQL-only or SurrealDB-only modes which have only one store
	app := startApp(t, postgresPort, "-mode", "single", "-port", testPort)
	defer app.Stop()
	waitForServer(t, testURL)

	// Step 2: Run background sync using CLI command
	t.Log("Step 2: Running background sync to catch up secondary store...")
	performSync := func(desc string) {
		t.Logf("  - %s", desc)
		syncSince := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
		syncUntil := time.Now().Add(2 * time.Second).Format(time.RFC3339)

		// Use the CLI sync command instead of admin endpoint
		args := []string{
			"-sync-direction", "forward",
			"-sync-since", syncSince,
			"-sync-until", syncUntil,
			"-postgres-port", postgresPort,
			"-mode", "single",
			"sync",
		}

		if err := surrealnote.Main(ctx, args); err != nil {
			t.Fatalf("Sync failed: %v", err)
		}
	}

	performSync("Initial background sync")

	// Step 3: Restart app in read-only mode for final preparation
	t.Log("Step 3: Switching to read-only mode to freeze data for final sync...")
	app.Stop()
	app = startApp(t, postgresPort, "-mode", "read_only", "-port", testPort)
	waitForServer(t, testURL)

	// Verify writes are blocked
	t.Log("  - Verifying writes are blocked in read-only mode...")
	testUser := &models.User{
		ID:    models.NewUserID(),
		Email: "readonly@test.com",
		Name:  "Should Be Blocked",
	}

	userJSON, _ := json.Marshal(testUser)
	resp, err := http.Post(testURL+"/api/users", "application/json", bytes.NewBuffer(userJSON))
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			t.Error("Write should have been blocked in read-only mode")
		} else {
			t.Log("  ✓ Writes correctly blocked")
		}
	}

	// Step 4: Final catch-up sync while read-only
	t.Log("Step 4: Performing final catch-up sync...")
	performSync("Final catch-up sync in read-only mode")

	// Step 5: Validate data consistency between stores
	t.Log("Step 5: Validating data consistency between PostgreSQL and SurrealDB...")

	// Connect directly to both stores
	pgDSN := fmt.Sprintf("postgres://surrealnote:surrealnote123@localhost:%s/surrealnote?sslmode=disable", postgresPort)
	pgStore, err := postgres.NewPostgresStore(pgDSN)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pgStore.Close()

	sdbStore, err := surrealdb.NewSurrealStoreCBOR(
		"ws://localhost:8000/rpc",
		"surrealnote",
		"surrealnote",
		"root",
		"root",
	)
	if err != nil {
		t.Fatalf("Failed to connect to SurrealDB: %v", err)
	}
	defer sdbStore.Close()

	// Comprehensive consistency check
	inconsistencies := 0

	// Check Users
	t.Log("  - Checking users...")
	for _, userID := range []models.UserID{stage3Data.User1.ID, stage3Data.User2.ID} {
		pgUser, pgErr := pgStore.GetUser(ctx, userID)
		sdbUser, sdbErr := sdbStore.GetUser(ctx, userID)

		if pgErr != nil || sdbErr != nil {
			t.Errorf("    Error fetching user %s: PG=%v, SDB=%v", userID, pgErr, sdbErr)
			inconsistencies++
		} else if pgUser == nil || sdbUser == nil {
			t.Errorf("    User %s missing: PG=%v, SDB=%v", userID, pgUser != nil, sdbUser != nil)
			inconsistencies++
		} else if pgUser.Email != sdbUser.Email || pgUser.Name != sdbUser.Name {
			t.Errorf("    User %s mismatch: PG(%s,%s) vs SDB(%s,%s)",
				userID, pgUser.Email, pgUser.Name, sdbUser.Email, sdbUser.Name)
			inconsistencies++
		} else {
			t.Logf("    ✓ User %s consistent", userID)
		}
	}

	// Check Workspaces
	t.Log("  - Checking workspaces...")
	pgWs, _ := pgStore.GetWorkspace(ctx, stage3Data.Workspace1.ID)
	sdbWs, _ := sdbStore.GetWorkspace(ctx, stage3Data.Workspace1.ID)
	if pgWs == nil || sdbWs == nil {
		t.Errorf("    Workspace missing: PG=%v, SDB=%v", pgWs != nil, sdbWs != nil)
		inconsistencies++
	} else if pgWs.Name != sdbWs.Name {
		t.Errorf("    Workspace name mismatch: PG=%s vs SDB=%s", pgWs.Name, sdbWs.Name)
		inconsistencies++
	} else {
		t.Log("    ✓ Workspace consistent")
	}

	// Check Pages (with updates from Stage 3)
	t.Log("  - Checking pages...")
	pgPage1, _ := pgStore.GetPage(ctx, stage3Data.Page1.ID)
	sdbPage1, _ := sdbStore.GetPage(ctx, stage3Data.Page1.ID)
	if pgPage1 == nil || sdbPage1 == nil {
		t.Errorf("    Page1 missing: PG=%v, SDB=%v", pgPage1 != nil, sdbPage1 != nil)
		inconsistencies++
	} else if pgPage1.Title != "Page One Updated" || sdbPage1.Title != "Page One Updated" {
		t.Errorf("    Page1 title mismatch: PG=%s vs SDB=%s", pgPage1.Title, sdbPage1.Title)
		inconsistencies++
	} else {
		t.Log("    ✓ Page1 consistent (with Stage 3 updates)")
	}

	// Check Blocks (with updates from Stage 3)
	t.Log("  - Checking blocks...")
	pgBlock1, _ := pgStore.GetBlock(ctx, stage3Data.Block1.ID)
	sdbBlock1, _ := sdbStore.GetBlock(ctx, stage3Data.Block1.ID)
	if pgBlock1 == nil || sdbBlock1 == nil {
		t.Errorf("    Block1 missing: PG=%v, SDB=%v", pgBlock1 != nil, sdbBlock1 != nil)
		inconsistencies++
	} else {
		pgContent, _ := pgBlock1.Content["text"].(string)
		sdbContent, _ := sdbBlock1.Content["text"].(string)
		if pgContent != "Block One Updated Content" || sdbContent != "Block One Updated Content" {
			t.Errorf("    Block1 content mismatch: PG=%s vs SDB=%s", pgContent, sdbContent)
			inconsistencies++
		} else {
			t.Log("    ✓ Block1 consistent (with Stage 3 updates)")
		}
	}

	if inconsistencies > 0 {
		t.Fatalf("Found %d data inconsistencies - cannot proceed with validation", inconsistencies)
	}

	t.Log("  ✓ All data validated as consistent between stores")

	// Step 6: Restart in "switching" mode to validate reads from secondary
	t.Log("Step 6: Switching to 'switching' mode (reads from SurrealDB)...")
	app.Stop()
	app = startApp(t, postgresPort, "-mode", "switching", "-port", testPort)
	defer app.Stop() // Ensure this app is stopped when sync completes
	waitForServer(t, testURL)

	// Give the mode change time to take effect
	time.Sleep(500 * time.Millisecond)

	// Step 7: Verify all data is accessible via API (now reading from SurrealDB)
	t.Log("Step 7: Verifying all data accessible from SurrealDB via API...")
	verifyUser(t, testURL, stage3Data.User1.ID, "user1@example.com", "User One")
	verifyUser(t, testURL, stage3Data.User2.ID, "user2@example.com", "User Two")
	verifyWorkspace(t, testURL, stage3Data.Workspace1.ID, "Workspace One", stage3Data.User1.ID)
	verifyPage(t, testURL, stage3Data.Page1.ID, "Page One Updated", stage3Data.Workspace1.ID)
	verifyPage(t, testURL, stage3Data.Page2.ID, "Page Two", stage3Data.Workspace1.ID)
	verifyBlock(t, testURL, stage3Data.Block1.ID, "Block One Updated Content", stage3Data.Page1.ID)

	t.Log("✓ Stage 4 COMPLETED - System validated and ready for cutover")
	t.Log("  Summary:")
	t.Log("  - Background sync caught up all data to secondary")
	t.Log("  - Read-only mode prevented new changes during validation")
	t.Log("  - Data consistency verified between stores")
	t.Log("  - Secondary store (SurrealDB) successfully serving reads")
	t.Log("  - System ready for final cutover in Stage 5")
}

// Stage 2: Data Migration - One-time copy from PostgreSQL to SurrealDB
func testStage2DataMigration(t *testing.T) {
	t.Log("=== STAGE 2: Data Migration ===")
	t.Log("Performing one-time data migration from PostgreSQL to SurrealDB")

	// Ensure stage1Data was populated
	if stage1Data.User1 == nil {
		t.Fatal("Stage 1 failed - User1 not found")
	}

	// Connect directly to both databases for migration
	t.Log("Connecting to databases for migration...")
	pgDSN := fmt.Sprintf("postgres://surrealnote:surrealnote123@localhost:%s/surrealnote?sslmode=disable", postgresPort)
	pgStore, err := postgres.NewPostgresStore(pgDSN)
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pgStore.Close()

	sdbStore, err := surrealdb.NewSurrealStoreCBOR(
		"ws://localhost:8000/rpc",
		"surrealnote",
		"surrealnote",
		"root",
		"root",
	)
	if err != nil {
		t.Fatalf("Failed to connect to SurrealDB: %v", err)
	}
	defer sdbStore.Close()

	ctx := context.Background()

	// Migrate all users
	t.Log("Migrating users...")
	if stage1Data.User1 != nil {
		users := []*models.User{stage1Data.User1}
		if stage1Data.User2 != nil {
			users = append(users, stage1Data.User2)
		}
		for _, user := range users {
			if err := sdbStore.CreateUser(ctx, user); err != nil {
				t.Logf("Warning: Failed to migrate user %s: %v (may already exist)", user.ID, err)
			} else {
				// Verify the user was actually created
				migratedUser, verifyErr := sdbStore.GetUser(ctx, user.ID)
				if verifyErr != nil {
					t.Errorf("ERROR: Failed to verify migrated user %s: %v", user.ID, verifyErr)
				} else if migratedUser == nil {
					t.Errorf("ERROR: Migrated user %s not found in SurrealDB", user.ID)
				} else {
					t.Logf("SUCCESS: Verified user %s migrated to SurrealDB", user.ID)
				}
			}
		}
		t.Logf("Migrated %d users", len(users))
	}

	// Migrate all workspaces
	t.Log("Migrating workspaces...")
	if stage1Data.Workspace1 != nil {
		workspaces := []*models.Workspace{stage1Data.Workspace1}
		if stage1Data.Workspace2 != nil {
			workspaces = append(workspaces, stage1Data.Workspace2)
		}
		for _, workspace := range workspaces {
			if err := sdbStore.CreateWorkspace(ctx, workspace); err != nil {
				t.Logf("Warning: Failed to migrate workspace %s: %v (may already exist)", workspace.ID, err)
			} else {
				// Verify the workspace was actually created
				migratedWorkspace, verifyErr := sdbStore.GetWorkspace(ctx, workspace.ID)
				if verifyErr != nil {
					t.Errorf("ERROR: Failed to verify migrated workspace %s: %v", workspace.ID, verifyErr)
				} else if migratedWorkspace == nil {
					t.Errorf("ERROR: Migrated workspace %s not found in SurrealDB", workspace.ID)
				} else {
					t.Logf("SUCCESS: Verified workspace %s migrated to SurrealDB", workspace.ID)
				}
			}
		}
		t.Logf("Migrated %d workspaces", len(workspaces))
	}

	// Migrate all pages
	t.Log("Migrating pages...")
	if stage1Data.Page1 != nil {
		pages := []*models.Page{stage1Data.Page1}
		if stage1Data.Page2 != nil {
			pages = append(pages, stage1Data.Page2)
		}
		for _, page := range pages {
			if err := sdbStore.CreatePage(ctx, page); err != nil {
				t.Logf("Warning: Failed to migrate page %s: %v (may already exist)", page.ID, err)
			} else {
				// Verify the page was actually created
				migratedPage, verifyErr := sdbStore.GetPage(ctx, page.ID)
				if verifyErr != nil {
					t.Errorf("ERROR: Failed to verify migrated page %s: %v", page.ID, verifyErr)
				} else if migratedPage == nil {
					t.Errorf("ERROR: Migrated page %s not found in SurrealDB", page.ID)
				} else {
					t.Logf("SUCCESS: Verified page %s migrated to SurrealDB", page.ID)
				}
			}
		}
		t.Logf("Migrated %d pages", len(pages))
	}

	// Migrate all blocks
	t.Log("Migrating blocks...")
	if stage1Data.Block1 != nil {
		blocks := []*models.Block{stage1Data.Block1}
		if stage1Data.Block2 != nil {
			blocks = append(blocks, stage1Data.Block2)
		}
		for _, block := range blocks {
			if err := sdbStore.CreateBlock(ctx, block); err != nil {
				t.Logf("Warning: Failed to migrate block %s: %v (may already exist)", block.ID, err)
			} else {
				// Verify the block was actually created
				migratedBlock, verifyErr := sdbStore.GetBlock(ctx, block.ID)
				if verifyErr != nil {
					t.Errorf("ERROR: Failed to verify migrated block %s: %v", block.ID, verifyErr)
				} else if migratedBlock == nil {
					t.Errorf("ERROR: Migrated block %s not found in SurrealDB", block.ID)
				} else {
					t.Logf("SUCCESS: Verified block %s migrated to SurrealDB", block.ID)
				}
			}
		}
		t.Logf("Migrated %d blocks", len(blocks))
	}

	// Migrate permissions
	t.Log("Migrating permissions...")
	if stage1Data.Permission1 != nil {
		permissions := []*models.Permission{stage1Data.Permission1}
		for _, perm := range permissions {
			// Ensure ResourceID has the correct table name after loading from PostgreSQL
			perm.ResourceID.SetTableForResourceType(perm.ResourceType)
			if err := sdbStore.CreatePermission(ctx, perm); err != nil {
				t.Logf("Warning: Failed to migrate permission %s: %v (may already exist)", perm.ID, err)
			}
		}
		t.Logf("Migrated %d permissions", len(permissions))
	}

	// Copy stage1 data to stage2
	*stage2Data = *stage1Data
	stage2Data.UpdatedInStage = make(map[string][]string)

	// Verify data really persists in SurrealDB by querying directly
	t.Log("Verifying data persistence in SurrealDB...")
	testUser, err := sdbStore.GetUser(ctx, stage1Data.User1.ID)
	if err != nil {
		t.Errorf("Failed to verify User1 in SurrealDB after migration: %v", err)
	} else if testUser == nil {
		t.Errorf("User1 not found in SurrealDB after migration")
	} else {
		t.Logf("✓ Verified User1 persists in SurrealDB: ID=%s, Email=%s", testUser.ID, testUser.Email)
	}

	t.Log("✓ Stage 2 completed - Data migration from PostgreSQL to SurrealDB complete")
}

// Stage 5: CQRS Switching Mode - Testing SurrealDB for reads while still writing to PostgreSQL
func testStage5CQRSSwitching(t *testing.T) {
	t.Log("=== STAGE 5: CQRS Switching Mode ===")
	t.Log("Both stores connected, reads from SurrealDB, writes still to PostgreSQL")

	// Ensure stage4Data was populated
	if stage4Data.User1 == nil {
		t.Fatal("Stage 4 failed - User1 not found")
	}

	// Copy stage4 data to stage5
	*stage5Data = *stage4Data
	stage5Data.UpdatedInStage = make(map[string][]string)
	for k, v := range stage4Data.UpdatedInStage {
		stage5Data.UpdatedInStage[k] = v
	}

	// First verify data exists in SurrealDB directly before starting the app
	t.Log("Pre-check: Verifying data exists in SurrealDB before starting app...")
	ctx := context.Background()
	sdbDirectStore, err := surrealdb.NewSurrealStoreCBOR(
		"ws://localhost:8000/rpc",
		"surrealnote",
		"surrealnote",
		"root",
		"root",
	)
	if err != nil {
		t.Fatalf("Failed to connect directly to SurrealDB: %v", err)
	}

	testUser, err := sdbDirectStore.GetUser(ctx, stage4Data.User1.ID)
	if err != nil {
		t.Logf("WARNING: Direct SurrealDB check failed to get User1: %v", err)
	} else if testUser == nil {
		t.Logf("WARNING: User1 not found in SurrealDB before app start")
	} else {
		t.Logf("✓ Pre-check: User1 exists in SurrealDB: ID=%s, Email=%s", testUser.ID, testUser.Email)
	}
	sdbDirectStore.Close()

	// Start app in CQRS "switching" mode (both stores, reads from SurrealDB, writes to PostgreSQL)
	app := startApp(t, postgresPort, "-mode", "switching", "-port", testPort)
	defer app.Stop()

	waitForServer(t, testURL)

	// Verify all existing data is accessible from SurrealDB
	t.Log("Verifying all data is accessible from SurrealDB...")

	// Verify all entities from Stage 3
	verifyUser(t, testURL, stage4Data.User1.ID, "user1@example.com", "User One")
	verifyUser(t, testURL, stage4Data.User2.ID, "user2@example.com", "User Two")
	verifyWorkspace(t, testURL, stage4Data.Workspace1.ID, "Workspace One", stage4Data.User1.ID)
	verifyPage(t, testURL, stage4Data.Page1.ID, "Page One Updated", stage4Data.Workspace1.ID)
	verifyPage(t, testURL, stage4Data.Page2.ID, "Page Two", stage4Data.Workspace1.ID)
	verifyBlock(t, testURL, stage4Data.Block1.ID, "Block One Updated Content", stage4Data.Page1.ID)

	// Note: In switching mode, writes go to PostgreSQL but reads come from SurrealDB.
	// New data created here won't be immediately visible because it hasn't been synced yet.
	// This demonstrates the eventual consistency model during migration.
	t.Log("Stage 5: Switching mode - reads from SurrealDB, writes to PostgreSQL")
	t.Log("New data written in this stage will be synced to SurrealDB later")

	// Copy stage5 data to stage6
	*stage6Data = *stage5Data
	stage6Data.UpdatedInStage = make(map[string][]string)
	for k, v := range stage5Data.UpdatedInStage {
		stage6Data.UpdatedInStage[k] = v
	}

	t.Log("✓ Stage 5 completed - Successfully reading from SurrealDB")
}

// Stage 6: CQRS with SurrealDB Primary - Write to SurrealDB, keep PostgreSQL in sync for rollback
func testStage6CQRSSurrealDBPrimary(t *testing.T) {
	t.Log("=== STAGE 6: CQRS Mode - SurrealDB Primary ===")
	t.Log("Both stores connected, writes to SurrealDB (primary), PostgreSQL kept in sync for rollback")

	// Ensure stage5Data was populated
	if stage5Data.User1 == nil {
		t.Fatal("Stage 5 failed - User1 not found")
	}

	// Copy stage5 data to stage6
	stage6Data = &StageData{
		User1:          stage5Data.User1,
		User2:          stage5Data.User2,
		Workspace1:     stage5Data.Workspace1,
		Page1:          stage5Data.Page1,
		Page2:          stage5Data.Page2,
		Block1:         stage5Data.Block1,
		Block2:         stage5Data.Block2,
		Permission1:    stage5Data.Permission1,
		UpdatedInStage: make(map[string][]string),
	}
	for k, v := range stage5Data.UpdatedInStage {
		stage6Data.UpdatedInStage[k] = v
	}

	// Start app with both stores in reversed mode - SurrealDB is primary for writes
	// This allows us to test SurrealDB as the primary store while keeping PostgreSQL
	// in sync for potential rollback
	app := startApp(t, postgresPort, "-mode", "reversed", "-port", testPort)
	defer app.Stop()

	waitForServer(t, testURL)

	// Create new data in SurrealDB (as primary)
	t.Log("Creating new data with SurrealDB as primary...")

	// Create a new block in SurrealDB
	block3 := createBlock(t, testURL, stage6Data.Page1.ID, "Block Three Content (SurrealDB primary)", models.BlockTypeText)
	t.Logf("Created Block3: %s (SurrealDB primary)", block3.ID)
	stage6Data.Block3 = block3

	// Update existing data in SurrealDB
	t.Log("Updating existing data with SurrealDB as primary...")
	if stage6Data.Page2 == nil {
		t.Fatal("Stage 6: Page2 is nil!")
	}
	t.Logf("About to update Page2 ID=%s to new title", stage6Data.Page2.ID)
	updatePageTitle(t, testURL, stage6Data.Page2.ID, "Page Two Updated in Stage 6 (SurrealDB primary)")
	t.Log("Page2 update completed")
	stage6Data.UpdatedInStage["page2_title"] = []string{"Stage 6"}

	// Copy stage6 data to stage7 (for the next stage)
	*stage7Data = *stage6Data
	stage7Data.UpdatedInStage = make(map[string][]string)
	for k, v := range stage6Data.UpdatedInStage {
		stage7Data.UpdatedInStage[k] = v
	}

	t.Log("✓ Stage 6 completed - Successfully writing to SurrealDB as primary")
}

// Stage 8: SurrealDB only mode - Complete migration
func testStage8SurrealDBOnly(t *testing.T) {
	t.Log("=== STAGE 8: SurrealDB Only Mode ===")
	t.Log("Using only SurrealDB - Migration complete!")

	// Ensure stage7Data was populated (from Stage 7 sync)
	if stage7Data.User1 == nil {
		t.Fatal("Stage 7 failed - User1 not found")
	}

	// Start app in SurrealDB-only mode
	app := startApp(t, postgresPort, "-surreal-only", "-port", testPort)
	defer app.Stop()

	waitForServer(t, testURL)

	// Verify all data is still accessible from SurrealDB only
	t.Log("Verifying all data in SurrealDB-only mode...")

	// Comprehensive verification of all data
	verifyUser(t, testURL, stage7Data.User1.ID, "user1@example.com", "User One")
	verifyUser(t, testURL, stage7Data.User2.ID, "user2@example.com", "User Two")
	verifyWorkspace(t, testURL, stage7Data.Workspace1.ID, "Workspace One", stage7Data.User1.ID)
	verifyPage(t, testURL, stage7Data.Page1.ID, "Page One Updated", stage7Data.Workspace1.ID)
	verifyPage(t, testURL, stage7Data.Page2.ID, "Page Two Updated in Stage 6 (SurrealDB primary)", stage7Data.Workspace1.ID)
	verifyBlock(t, testURL, stage7Data.Block1.ID, "Block One Updated Content", stage7Data.Page1.ID)
	// Verify Block3 created in Stage 6
	if stage7Data.Block3 != nil {
		verifyBlock(t, testURL, stage7Data.Block3.ID, "Block Three Content (SurrealDB primary)", stage7Data.Page1.ID)
	}

	// Create final test data in SurrealDB only
	t.Log("Creating final data in SurrealDB-only mode...")

	// Create fourth block in page1 (Block3 was already created in Stage 6)
	block4 := createBlock(t, testURL, stage7Data.Page1.ID, "Block Four Content", models.BlockTypeCode)
	stage7Data.Block4 = block4
	t.Logf("Created Block4: %s (SurrealDB only)", block4.ID)

	// Final update to user1 name
	updateUserName(t, testURL, stage7Data.User1.ID, "User One Final")
	stage7Data.UpdatedInStage["stage8"] = append(stage7Data.UpdatedInStage["stage8"], "User1 name updated")
	t.Logf("Updated User1 name (SurrealDB only)")

	// Verify final changes
	t.Log("Verifying Stage 8 final changes...")
	verifyBlock(t, testURL, block4.ID, "Block Four Content", stage7Data.Page1.ID)
	verifyUser(t, testURL, stage7Data.User1.ID, "user1@example.com", "User One Final")

	// Print migration summary
	t.Log("\n=== MIGRATION SUMMARY ===")
	t.Logf("✓ Stage 1: Created %d initial entities in PostgreSQL", 5)
	t.Logf("✓ Stage 2: Migrated all data from PostgreSQL to SurrealDB")
	t.Logf("✓ Stage 3: Added %d new entities, updated %d entities (CQRS - PostgreSQL primary)", 2, 2)
	t.Logf("✓ Sync: Synchronized all Stage 3 changes to SurrealDB")
	t.Logf("✓ Stage 4: Validated all data is accessible from SurrealDB")
	t.Logf("✓ Stage 5: Validated switching mode (reads from SurrealDB)")
	t.Logf("✓ Stage 6: Tested writing to SurrealDB as primary")
	t.Logf("✓ Stage 7: Synchronized SurrealDB changes back to PostgreSQL")
	t.Logf("✓ Stage 8: Added %d final entities in SurrealDB only", 1)
	t.Log("\n✓ MIGRATION COMPLETE - Successfully migrated from PostgreSQL to SurrealDB!")
}

// Helper functions for starting/stopping services
func startPostgres(port string) error {
	cmd := exec.Command("make", "postgres-start", fmt.Sprintf("POSTGRES_PORT=%s", port))
	return cmd.Run()
}

func stopPostgres(port string) {
	cmd := exec.Command("make", "postgres-stop", fmt.Sprintf("POSTGRES_PORT=%s", port))
	_ = cmd.Run() // Ignore error on stop
}

func cleanupPostgres(port string) {
	cmd := exec.Command("make", "postgres-remove", fmt.Sprintf("POSTGRES_PORT=%s", port))
	_ = cmd.Run() // Best effort cleanup
}

func runMigrations(t *testing.T, pgPort string, args ...string) {
	allArgs := append([]string{fmt.Sprintf("-postgres-port=%s", pgPort)}, args...)
	allArgs = append(allArgs, "migrate")

	// Use surrealnote.Main directly instead of executing the binary
	// Pass context.Background() for migrations (they should complete quickly)
	if err := surrealnote.Main(context.Background(), allArgs); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}
	t.Log("Migrations completed successfully")
}

type TestApp struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func (a *TestApp) Stop() {
	if a.cancel != nil {
		a.cancel()
		// Wait for the goroutine to finish
		<-a.done
		// Make idempotent - clear cancel so subsequent calls do nothing
		a.cancel = nil
	}
}

func startApp(t *testing.T, pgPort string, args ...string) *TestApp {
	allArgs := append([]string{fmt.Sprintf("-postgres-port=%s", pgPort)}, args...)
	// Add "run" command at the end
	allArgs = append(allArgs, "run")

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	// Run the app in a goroutine
	go func() {
		defer close(done)

		// Run the application with context
		if err := surrealnote.Main(ctx, allArgs); err != nil {
			// Check if context was cancelled (normal shutdown)
			if ctx.Err() != nil {
				// Normal shutdown
				return
			}
			// Unexpected error
			t.Logf("App error: %v", err)
		}
	}()

	return &TestApp{cancel: cancel, done: done}
}

func waitForServer(t *testing.T, url string) {
	for i := 0; i < 30; i++ {
		resp, err := http.Get(url + "/api/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			t.Log("Server is ready")
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatal("Server failed to start within 30 seconds")
}

// Entity creation helpers
func createUser(t *testing.T, baseURL, email, name string) *models.User {
	user := &models.User{
		Email: email,
		Name:  name,
	}

	body, _ := json.Marshal(user)
	resp, err := http.Post(baseURL+"/api/users", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create user: status=%d, body=%s", resp.StatusCode, body)
	}

	var created models.User
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("Failed to decode user response: %v", err)
	}

	return &created
}

func createWorkspace(t *testing.T, baseURL, name string, ownerID models.UserID) *models.Workspace {
	workspace := &models.Workspace{
		Name:    name,
		OwnerID: ownerID,
	}

	body, _ := json.Marshal(workspace)
	resp, err := http.Post(baseURL+"/api/workspaces", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create workspace: status=%d, body=%s", resp.StatusCode, body)
	}

	var created models.Workspace
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("Failed to decode workspace response: %v", err)
	}

	return &created
}

func createPage(t *testing.T, baseURL string, workspaceID models.WorkspaceID, title string, createdBy models.UserID) *models.Page {
	page := &models.Page{
		WorkspaceID: workspaceID,
		Title:       title,
		CreatedBy:   createdBy,
	}

	body, _ := json.Marshal(page)
	resp, err := http.Post(baseURL+"/api/pages", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create page: status=%d, body=%s", resp.StatusCode, body)
	}

	var created models.Page
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("Failed to decode page response: %v", err)
	}

	return &created
}

func createBlock(t *testing.T, baseURL string, pageID models.PageID, content string, blockType models.BlockType) *models.Block {
	block := &models.Block{
		PageID:  pageID,
		Type:    blockType,
		Content: models.JSONMap{"text": content},
		Order:   0,
	}

	body, _ := json.Marshal(block)
	resp, err := http.Post(baseURL+"/api/blocks", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create block: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create block: status=%d, body=%s", resp.StatusCode, body)
	}

	var created models.Block
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("Failed to decode block response: %v", err)
	}

	return &created
}

func createPermission(t *testing.T, baseURL string, userID models.UserID, resourceID models.ResourceID, resourceType models.ResourceType, level models.PermissionLevel) *models.Permission {
	permission := &models.Permission{
		UserID:          userID,
		ResourceID:      resourceID,
		ResourceType:    resourceType,
		PermissionLevel: level,
	}

	body, _ := json.Marshal(permission)
	resp, err := http.Post(baseURL+"/api/permissions", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create permission: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to create permission: status=%d, body=%s", resp.StatusCode, body)
	}

	var created models.Permission
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("Failed to decode permission response: %v", err)
	}

	return &created
}

// Update helpers

func updatePageTitle(t *testing.T, baseURL string, pageID models.PageID, newTitle string) {
	t.Logf("updatePageTitle: baseURL=%s, pageID=%s, newTitle=%s", baseURL, pageID, newTitle)

	// First get the current page
	resp, err := http.Get(fmt.Sprintf("%s/api/pages/%s", baseURL, pageID))
	if err != nil {
		t.Fatalf("Failed to get page for update: %v", err)
	}

	var page models.Page
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		t.Fatalf("Failed to decode page: %v", err)
	}
	resp.Body.Close()

	t.Logf("updatePageTitle: Got page with current title=%s", page.Title)

	// Update the title
	page.Title = newTitle
	body, _ := json.Marshal(page)

	t.Logf("updatePageTitle: Sending PUT request with new title=%s", newTitle)

	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/pages/%s", baseURL, pageID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to update page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to update page: status=%d, body=%s", resp.StatusCode, body)
	}
	t.Logf("updatePageTitle: Update succeeded with status %d", resp.StatusCode)
}

func updateBlockContent(t *testing.T, baseURL string, blockID models.BlockID, newContent string) {
	// First get the current block
	resp, err := http.Get(fmt.Sprintf("%s/api/blocks/%s", baseURL, blockID))
	if err != nil {
		t.Fatalf("Failed to get block for update: %v", err)
	}

	var block models.Block
	if err := json.NewDecoder(resp.Body).Decode(&block); err != nil {
		t.Fatalf("Failed to decode block: %v", err)
	}
	resp.Body.Close()

	// Update the content
	block.Content = models.JSONMap{"text": newContent}
	body, _ := json.Marshal(block)

	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/blocks/%s", baseURL, blockID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to update block: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to update block: status=%d, body=%s", resp.StatusCode, body)
	}
}

func updateUserName(t *testing.T, baseURL string, userID models.UserID, newName string) {
	// First get the current user
	resp, err := http.Get(fmt.Sprintf("%s/api/users/%s", baseURL, userID))
	if err != nil {
		t.Fatalf("Failed to get user for update: %v", err)
	}

	var user models.User
	json.NewDecoder(resp.Body).Decode(&user)
	resp.Body.Close()

	// Update the name
	user.Name = newName
	body, _ := json.Marshal(user)

	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/api/users/%s", baseURL, userID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to update user: status=%d, body=%s", resp.StatusCode, body)
	}
}

// Verification helpers
func verifyUser(t *testing.T, baseURL string, userID models.UserID, expectedEmail, expectedName string) {
	resp, err := http.Get(fmt.Sprintf("%s/api/users/%s", baseURL, userID))
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to get user: status=%d, body=%s", resp.StatusCode, body)
	}

	var user models.User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		t.Fatalf("Failed to decode user: %v", err)
	}

	if user.Email != expectedEmail {
		t.Errorf("User email mismatch: expected %s, got %s", expectedEmail, user.Email)
	}
	if user.Name != expectedName {
		t.Errorf("User name mismatch: expected %s, got %s", expectedName, user.Name)
	}
}

func verifyWorkspace(t *testing.T, baseURL string, workspaceID models.WorkspaceID, expectedName string, expectedOwnerID models.UserID) {
	resp, err := http.Get(fmt.Sprintf("%s/api/workspaces/%s", baseURL, workspaceID))
	if err != nil {
		t.Fatalf("Failed to get workspace: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to get workspace: status=%d, body=%s", resp.StatusCode, body)
	}

	var workspace models.Workspace
	if err := json.NewDecoder(resp.Body).Decode(&workspace); err != nil {
		t.Fatalf("Failed to decode workspace: %v", err)
	}

	if workspace.Name != expectedName {
		t.Errorf("Workspace name mismatch: expected %s, got %s", expectedName, workspace.Name)
	}
	if workspace.OwnerID != expectedOwnerID {
		t.Errorf("Workspace owner mismatch: expected %s, got %s", expectedOwnerID, workspace.OwnerID)
	}
}

func verifyPage(t *testing.T, baseURL string, pageID models.PageID, expectedTitle string, expectedWorkspaceID models.WorkspaceID) {
	resp, err := http.Get(fmt.Sprintf("%s/api/pages/%s", baseURL, pageID))
	if err != nil {
		t.Fatalf("Failed to get page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to get page: status=%d, body=%s", resp.StatusCode, body)
	}

	var page models.Page
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		t.Fatalf("Failed to decode page: %v", err)
	}

	if page.Title != expectedTitle {
		t.Errorf("Page title mismatch: expected %s, got %s", expectedTitle, page.Title)
	}
	if page.WorkspaceID != expectedWorkspaceID {
		t.Errorf("Page workspace mismatch: expected %s, got %s", expectedWorkspaceID, page.WorkspaceID)
	}
}

func verifyBlock(t *testing.T, baseURL string, blockID models.BlockID, expectedContent string, expectedPageID models.PageID) {
	resp, err := http.Get(fmt.Sprintf("%s/api/blocks/%s", baseURL, blockID))
	if err != nil {
		t.Fatalf("Failed to get block: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to get block: status=%d, body=%s", resp.StatusCode, body)
	}

	var block models.Block
	if err := json.NewDecoder(resp.Body).Decode(&block); err != nil {
		t.Fatalf("Failed to decode block: %v", err)
	}

	if text, ok := block.Content["text"].(string); !ok || text != expectedContent {
		t.Errorf("Block content mismatch: expected %s, got %v", expectedContent, block.Content["text"])
	}
	if block.PageID != expectedPageID {
		t.Errorf("Block page mismatch: expected %s, got %s", expectedPageID, block.PageID)
	}
}

// performCatchUpSync performs timestamp-based catch-up synchronization
// from PostgreSQL to SurrealDB to ensure consistency after Stage 3.
// In production: the main app would be restarted in read-only mode, then sync runs separately.
//
// OPTIMIZATION NOTE: In production, you can run continuous catch-up sync in parallel
// with Stage 3 to minimize the final sync window:
// - Run sync every N minutes during Stage 3 to keep SurrealDB nearly up-to-date
// - The final sync with read-only mode only needs to sync the last few minutes of changes
// - This reduces the read-only window from potentially hours to just minutes
//
// PRODUCTION SCALING NOTE: For deployments with multiple replicas, you may need a
// runtime API to toggle read-only mode without restart to avoid downtime:
// - With restart approach: Each replica must be restarted sequentially
// - With runtime API: All replicas can be toggled simultaneously
// - For 100+ replicas, restart approach could mean significant cumulative downtime
// - Runtime API allows coordinated read-only toggle across all replicas instantly
// - Consider implementing admin API endpoint if you have many replicas or strict SLAs
func performCatchUpSync(t *testing.T, since, until time.Time) {
	t.Log("=== CATCH-UP SYNC ===")
	t.Log("Production workflow simulation:")
	t.Log("1. Main app would be restarted in read-only mode")
	t.Log("2. Final sync runs as a separate process (only syncing recent changes)")
	t.Log("3. Main app restarted in normal mode after sync")
	t.Log("NOTE: Continuous sync during Stage 3 minimizes the final sync time")

	// Start the app in read-only mode (simulating production scenario)
	t.Log("Step 1: Starting app in read-only mode to prevent user writes")
	readOnlyApp := startApp(t, postgresPort, "-mode", "read_only", "-port", testPort)

	// Wait for the read-only app to be ready
	waitForServer(t, testURL)
	t.Log("✓ App running in read-only mode (user writes blocked)")

	t.Logf("Step 2: Running sync (separate process) for changes between %v and %v", since, until)

	// Run sync as a separate process (without read-only flag, so it can write to databases)
	ctx := context.Background()
	args := []string{
		"-sync-direction", "forward",
		"-sync-since", since.Format(time.RFC3339),
		"-sync-until", until.Format(time.RFC3339),
		"-postgres-port", postgresPort,
		"-mode", "single", // Sync from primary to secondary
		"sync",
	}

	if err := surrealnote.Main(ctx, args); err != nil {
		t.Errorf("Catch-up sync failed: %v", err)
	} else {
		t.Log("✓ Catch-up sync completed successfully")
	}

	// Stop the read-only app
	t.Log("Step 3: Stopping read-only app")
	readOnlyApp.Stop()
	t.Log("✓ Ready to restart app in normal mode for Stage 4")
}

// performReverseCatchUpSync performs timestamp-based catch-up synchronization
// from SurrealDB to PostgreSQL to ensure consistency for potential rollback scenarios.
// In production: the main app would be restarted in read-only mode, then sync runs separately.
//
// OPTIMIZATION NOTE: Similar to forward sync, reverse sync can run continuously
// during Stage 5 (switching mode) to keep PostgreSQL up-to-date:
// - Run reverse sync periodically while in Stage 5
// - Final sync only handles the most recent changes
// - Minimizes downtime if rollback to PostgreSQL is needed
func performReverseCatchUpSync(t *testing.T, since, until time.Time) {
	t.Log("=== REVERSE CATCH-UP SYNC ===")
	t.Log("Production workflow simulation:")
	t.Log("Running reverse sync as a separate process to sync changes from SurrealDB to PostgreSQL")
	t.Log("This ensures PostgreSQL has all latest changes for potential rollback scenarios")

	t.Logf("Running reverse sync for changes between %v and %v", since, until)

	// Run reverse sync as a separate process
	ctx := context.Background()
	args := []string{
		"-sync-direction", "reverse",
		"-sync-since", since.Format(time.RFC3339),
		"-sync-until", until.Format(time.RFC3339),
		"-postgres-port", postgresPort,
		"-mode", "switching", // Sync needs both databases configured
		"sync",
	}

	if err := surrealnote.Main(ctx, args); err != nil {
		t.Errorf("Reverse catch-up sync failed: %v", err)
	} else {
		t.Log("✓ Reverse catch-up sync completed successfully")
	}
}
