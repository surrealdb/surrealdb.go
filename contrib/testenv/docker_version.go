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

// SetupVersionTest starts a SurrealDB Docker container for the given version tag
// and returns a connected, signed-in database handle. The container is removed
// when cleanup is called.
func SetupVersionTest(t *testing.T, version string) (db *surrealdb.DB, cleanup func()) {
	t.Helper()
	_, db, cleanup = SetupVersionTestWithHTTPURL(t, version)
	return db, cleanup
}

// SetupVersionTestWithHTTPURL is like SetupVersionTest but also returns the WebSocket URL.
func SetupVersionTestWithHTTPURL(t *testing.T, version string, extraArgs ...string) (wsURL string, db *surrealdb.DB, cleanup func()) {
	t.Helper()

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping version behavior test")
	}

	ctx := context.Background()

	if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
		t.Skip("Docker daemon not running, skipping version behavior test")
	}

	containerName := fmt.Sprintf("surrealdb-behavior-test-%s-%d", version, time.Now().UnixNano())

	_ = exec.CommandContext(ctx, "docker", "rm", "-f", containerName).Run()

	args := []string{
		"run", "-d",
		"--name", containerName,
		"-p", "0:8000",
		fmt.Sprintf("surrealdb/surrealdb:%s", version),
		"start", "--user", defaultRootUser, "--pass", defaultRootPass,
	}
	args = append(args, extraArgs...)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to start container: %s", string(output))

	containerCleanup := func() {
		cleanupCtx := context.Background()
		_ = exec.CommandContext(cleanupCtx, "docker", "rm", "-f", containerName).Run()
	}

	portCmd := exec.CommandContext(ctx, "docker", "port", containerName, "8000")
	portOutput, err := portCmd.CombinedOutput()
	if err != nil {
		containerCleanup()
		t.Fatalf("Failed to get container port: %v, output: %s", err, string(portOutput))
	}
	portStr := string(portOutput)
	for i := len(portStr) - 1; i >= 0; i-- {
		if portStr[i] == ':' {
			portStr = portStr[i+1:]
			break
		}
	}
	portStr = portStr[:len(portStr)-1]
	wsURL = fmt.Sprintf("ws://localhost:%s/rpc", portStr)

	for i := 0; i < 30; i++ {
		db, err = surrealdb.FromEndpointURLString(ctx, wsURL)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		containerCleanup()
		t.Fatalf("Failed to connect to SurrealDB: %v", err)
	}

	_, err = db.SignIn(ctx, surrealdb.Auth{
		Username: defaultRootUser,
		Password: defaultRootPass,
	})
	if err != nil {
		containerCleanup()
		t.Fatalf("Failed to sign in: %v", err)
	}

	err = db.Use(ctx, "test_ns", "test_db")
	if err != nil {
		containerCleanup()
		t.Fatalf("Failed to use database: %v", err)
	}

	cleanup = func() {
		db.Close(ctx)
		containerCleanup()
	}
	return wsURL, db, cleanup
}
