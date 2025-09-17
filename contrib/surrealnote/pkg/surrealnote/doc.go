// Package surrealnote provides the core application logic for a hierarchical note-taking system
// demonstrating zero-downtime database migration from PostgreSQL to SurrealDB.
// In a real-world project: Add dependency injection container, configuration management via Viper,
// and implement health check probes for Kubernetes readiness/liveness.
//
// Note: This is a backend-only demonstration with REST API endpoints. No user interface is provided.
// The focus is on demonstrating database migration patterns, not building a complete application.
//
// # Getting Started
//
// The application provides a command-line interface for running the server and managing migrations.
// For detailed usage information, see [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/surrealnote.Main].
//
// For API endpoint documentation and server configuration, see [github.com/surrealdb/surrealdb.go/contrib/surrealnote/pkg/surrealnote.App.Run].
//
// # Prerequisites
//
//   - Go 1.21+
//   - Docker (for PostgreSQL)
//   - SurrealDB running on localhost:8000
//
// # Installation
//
//	# Clone the repository
//	git clone https://github.com/surrealdb/surrealdb.go
//	cd surrealdb.go/contrib/surrealnote
//
//	# Start SurrealDB
//	surreal start --user root --pass root
//
//	# Start PostgreSQL (using Docker)
//	make postgres-start
//
//	# Run database migrations
//	make migrate
//
//	# Build and run the application
//	make build
//	./bin/surrealnote
//
// # Basic Usage
//
//	# Run with PostgreSQL only
//	./bin/surrealnote -postgres-only
//
//	# Run with SurrealDB only
//	./bin/surrealnote -surreal-only
//
//	# Run in migration mode (default)
//	./bin/surrealnote -mode single
//
//	# Run with read-only mode during migration
//	./bin/surrealnote -mode read_only
//
//	# Run with switching mode (reads from SurrealDB)
//	./bin/surrealnote -mode switching
//
//	# Run with reversed mode (writes to SurrealDB)
//	./bin/surrealnote -mode reversed
//
//	# Perform database schema migrations
//	./bin/surrealnote -migrate
//
//	# Run synchronization
//	./bin/surrealnote -sync -sync-direction forward -sync-since "2023-12-01T00:00:00Z"
package surrealnote
