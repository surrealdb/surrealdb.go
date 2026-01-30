package testenv

// SurrealDB host:port constants for test containers.
// Consolidating port numbers here helps avoid conflicts between tests
// that spin up their own Docker containers.
const (
	// DefaultSurrealDBPort is the default port for the main SurrealDB instance
	// used by most tests (typically started externally or by CI).
	DefaultSurrealDBPort = "8000"

	// VersionIntegrationTestPort is used by version_integration_test.go
	// for testing version detection against multiple SurrealDB versions.
	VersionIntegrationTestPort = "18000"

	// VersionBehaviorTestPort is used by version_behavior_test.go
	// for testing behavioral differences between SurrealDB versions.
	VersionBehaviorTestPort = "18001"
)

// WebSocket URL helpers for test containers.
// Note: DefaultWSURL for the main SurrealDB instance is defined in connection.go
const (
	// VersionIntegrationTestWSURL is the WebSocket URL for version integration tests.
	VersionIntegrationTestWSURL = "ws://localhost:" + VersionIntegrationTestPort + "/rpc"

	// VersionBehaviorTestWSURL is the WebSocket URL for version behavior tests.
	VersionBehaviorTestWSURL = "ws://localhost:" + VersionBehaviorTestPort + "/rpc"
)

// Docker port mapping helpers (format: "hostPort:containerPort").
const (
	// VersionIntegrationTestPortMapping is the Docker port mapping for version integration tests.
	VersionIntegrationTestPortMapping = VersionIntegrationTestPort + ":8000"

	// VersionBehaviorTestPortMapping is the Docker port mapping for version behavior tests.
	VersionBehaviorTestPortMapping = VersionBehaviorTestPort + ":8000"
)
