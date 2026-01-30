package testenv

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	surrealdb "github.com/surrealdb/surrealdb.go"
)

func TestGetVersion_Integration(t *testing.T) {
	// Skip if Docker is not available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	ctx := context.Background()

	// Check if Docker daemon is running
	if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
		t.Skip("Docker daemon not running, skipping integration test")
	}

	tests := []struct {
		dockerTag      string
		expectedMajor  int
		expectedMinor  int
		expectedPatch  int
		expectedPrerel string
		isV3           bool
	}{
		{"v2.6.0", 2, 6, 0, "", false},
		{"v3.0.0-beta.2", 3, 0, 0, "beta.2", true},
	}

	for _, tt := range tests {
		t.Run(tt.dockerTag, func(t *testing.T) {
			ctx := context.Background()
			containerName := fmt.Sprintf("surrealdb-version-test-%s-%d", tt.dockerTag, time.Now().UnixNano())

			// Cleanup any existing container
			_ = exec.CommandContext(ctx, "docker", "rm", "-f", containerName).Run()

			// Start container with dynamic port allocation
			cmd := exec.CommandContext(ctx, "docker", "run", "-d",
				"--name", containerName,
				"-p", "0:8000",
				fmt.Sprintf("surrealdb/surrealdb:%s", tt.dockerTag),
				"start", "--user", "root", "--pass", "root",
			)
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "Failed to start container: %s", string(output))

			// Ensure cleanup on test exit
			t.Cleanup(func() {
				cleanupCtx := context.Background()
				_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", containerName).Run()
			})

			// Get the dynamically allocated port
			portCmd := exec.CommandContext(ctx, "docker", "port", containerName, "8000")
			portOutput, err := portCmd.CombinedOutput()
			require.NoError(t, err, "Failed to get container port: %s", string(portOutput))
			// Output format: "0.0.0.0:12345\n" - extract port number
			portStr := string(portOutput)
			for i := len(portStr) - 1; i >= 0; i-- {
				if portStr[i] == ':' {
					portStr = portStr[i+1:]
					break
				}
			}
			portStr = portStr[:len(portStr)-1] // Remove trailing newline
			wsURL := fmt.Sprintf("ws://localhost:%s/rpc", portStr)

			// Wait for container to be ready
			var db *surrealdb.DB
			for i := 0; i < 30; i++ {
				db, err = surrealdb.FromEndpointURLString(ctx, wsURL)
				if err == nil {
					break
				}
				time.Sleep(1 * time.Second)
			}
			require.NoError(t, err, "Failed to connect to SurrealDB")
			defer db.Close(ctx)

			// Sign in as root
			_, err = db.SignIn(ctx, surrealdb.Auth{
				Username: "root",
				Password: "root",
			})
			require.NoError(t, err)

			// Get and verify version
			v, err := GetVersion(ctx, db)
			require.NoError(t, err)

			require.Equal(t, tt.expectedMajor, v.Major, "Major version mismatch")
			require.Equal(t, tt.expectedMinor, v.Minor, "Minor version mismatch")
			require.Equal(t, tt.expectedPatch, v.Patch, "Patch version mismatch")
			require.Equal(t, tt.expectedPrerel, v.Prerelease, "Prerelease mismatch")
			require.Equal(t, tt.isV3, v.IsV3OrLater(), "IsV3OrLater mismatch")

			// Verify ThingOrRecordFn
			expectedFn := "type::thing"
			if tt.isV3 {
				expectedFn = "type::record"
			}
			require.Equal(t, expectedFn, v.ThingOrRecordFn())
		})
	}
}
