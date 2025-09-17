.PHONY: build run test lint format clean postgres-start postgres-stop postgres-restart migrate help server-start server-stop

# Variables
POSTGRES_CONTAINER = surrealnote-postgres
POSTGRES_PORT = 5438
POSTGRES_USER = surrealnote
POSTGRES_PASSWORD = surrealnote123
POSTGRES_DB = surrealnote
POSTGRES_DSN = postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@localhost:$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable

SURREALDB_URL = ws://localhost:8000/rpc
SURREALDB_NS = surrealnote
SURREALDB_DB = surrealnote
SURREALDB_USER = root
SURREALDB_PASS = root

# Default target
all: build

# Build the application
build:
	@echo "Building surrealnote..."
	@go build -o bin/surrealnote cmd/surrealnote/main.go
	@echo "Build complete: bin/surrealnote"

# Run the application with PostgreSQL management
run: postgres-start
	@echo "Running surrealnote..."
	@echo "Press Ctrl+C to stop..."
	@trap 'make postgres-stop' INT; \
		go run cmd/surrealnote/main.go run --port 8080 || true
	@echo "Application stopped"

# Run the application without stopping PostgreSQL afterwards (for development)
run-dev: postgres-start
	@echo "Running surrealnote in development mode..."
	@echo "PostgreSQL will remain running after exit"
	@echo "Press Ctrl+C to stop..."
	@go run cmd/surrealnote/main.go run --port 8080

# Run tests - executes all Go tests in the project
test:
	@echo "Running tests..."
	@go test -v ./...

# Run linting
lint:
	@echo "Running linting..."
	@go fmt ./...
	@go vet ./...
	@echo "Linting complete"

# Format code and remove trailing whitespace
format:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Removing trailing whitespace..."
	@find . -name "*.go" -type f -exec sed -i 's/[[:space:]]*$$//' {} \;
	@echo "Formatting complete"

# Run E2E migration test
test-e2e: build
	@echo "Running E2E migration test..."
	@go test -v -run TestE2EMigration -timeout 10m .

# Start server in background for testing
server-start: build postgres-start
	@echo "Starting surrealnote server in background..."
	@if lsof -Pi :8080 -sTCP:LISTEN -t >/dev/null 2>&1 ; then \
		echo "Server already running on port 8080"; \
	else \
		POSTGRES_DSN="$(POSTGRES_DSN)" \
		SURREALDB_URL="$(SURREALDB_URL)" \
		SURREALDB_NS="$(SURREALDB_NS)" \
		SURREALDB_DB="$(SURREALDB_DB)" \
		SURREALDB_USER="$(SURREALDB_USER)" \
		SURREALDB_PASS="$(SURREALDB_PASS)" \
		./bin/surrealnote run --port 8080 > surrealnote.log 2>&1 & echo $$! > surrealnote.pid; \
		echo "Server started with PID $$(cat surrealnote.pid)"; \
		echo "Waiting for server to be ready..."; \
		sleep 3; \
	fi

# Stop the background server
server-stop:
	@echo "Stopping surrealnote server..."
	@if [ -f surrealnote.pid ]; then \
		kill $$(cat surrealnote.pid) 2>/dev/null || true; \
		rm -f surrealnote.pid; \
		echo "Server stopped"; \
	else \
		echo "No server PID file found"; \
	fi
	@rm -f surrealnote.log

# Run smoke tests (requires build tag and starts server automatically)
smoke-test: server-start
	@echo "Running smoke test..."
	@echo "=============================================="
	@echo "Configuration (set env vars to customize):"
	@echo "  SMOKE_NUM_USERS    - Number of virtual users (default: 10)"
	@echo "  SMOKE_DURATION     - Test duration (default: 0 for scenario-based)"
	@echo "  SMOKE_TIMEOUT      - Overall timeout (default: 5m)"
	@echo "  SMOKE_WORKLOAD     - Workload type: scenario|continuous|burst (default: scenario)"
	@echo "  SMOKE_SUCCESS_RATE - Required success rate (default: 95.0)"
	@echo "  SMOKE_ENABLE_SCALING - Enable progressive scaling (default: false)"
	@echo "  SMOKE_SHARED_RESOURCE - Test shared resource access (default: false)"
	@echo ""
	@echo "Examples:"
	@echo "  make smoke-test                          # Default test with 10 users"
	@echo "  SMOKE_NUM_USERS=100 make smoke-test      # High load test"
	@echo "  SMOKE_ENABLE_SCALING=true make smoke-test # Scaling test"
	@echo "  SMOKE_WORKLOAD=continuous SMOKE_DURATION=2m make smoke-test # Stress test"
	@echo "=============================================="
	@go test -tags=smoke -count=1 -v -run TestE2ESmoke -timeout 30m .
	@make server-stop

# Clean build artifacts - removes compiled binaries and temporary files
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f surrealnote.pid surrealnote.log
	@echo "Clean complete"

# Database Management Commands
# postgres-start - Start PostgreSQL container on port 5438
postgres-start:
	@echo "Starting PostgreSQL container..."
	@docker run -d \
		--name $(POSTGRES_CONTAINER) \
		-e POSTGRES_USER=$(POSTGRES_USER) \
		-e POSTGRES_PASSWORD=$(POSTGRES_PASSWORD) \
		-e POSTGRES_DB=$(POSTGRES_DB) \
		-p $(POSTGRES_PORT):5432 \
		postgres:15-alpine 2>/dev/null || \
		(echo "Container already exists, starting it..." && docker start $(POSTGRES_CONTAINER))
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 3
	@echo "PostgreSQL is ready at localhost:$(POSTGRES_PORT)"

# postgres-stop - Stop PostgreSQL container without removing it
postgres-stop:
	@echo "Stopping PostgreSQL container..."
	@docker stop $(POSTGRES_CONTAINER) 2>/dev/null || true
	@echo "PostgreSQL container stopped"

# postgres-restart - Restart PostgreSQL container (stop then start)
postgres-restart: postgres-stop postgres-start

# postgres-remove - Stop and remove PostgreSQL container completely
postgres-remove: postgres-stop
	@echo "Removing PostgreSQL container..."
	@docker rm $(POSTGRES_CONTAINER) 2>/dev/null || true
	@echo "PostgreSQL container removed"

# Run database migrations
migrate:
	@echo "Running database migrations..."
	@go run cmd/surrealnote/main.go migrate
	@echo "Migrations complete"

# Display help
help:
	@echo "SurrealNote Makefile Commands:"
	@echo ""
	@echo "Basic Commands:"
	@echo "  make build          - Build the application"
	@echo "  make run            - Run the application (starts/stops PostgreSQL)"
	@echo "  make run-dev        - Run the application (keeps PostgreSQL running)"
	@echo "  make test           - Run tests"
	@echo "  make test-e2e       - Run E2E migration test"
	@echo "  make lint           - Run linting (go fmt and go vet)"
	@echo "  make format         - Format code and remove trailing whitespace"
	@echo "  make clean          - Clean build artifacts"
	@echo ""
	@echo "Server Management:"
	@echo "  make server-start   - Start server in background for testing"
	@echo "  make server-stop    - Stop background server"
	@echo ""
	@echo "Smoke Testing:"
	@echo "  make smoke-test     - Run smoke test (configurable via env vars)"
	@echo "                        Set SMOKE_NUM_USERS, SMOKE_DURATION, etc."
	@echo "                        Run 'make smoke-test' to see all options"
	@echo ""
	@echo "Database Commands:"
	@echo "  make postgres-start - Start PostgreSQL container"
	@echo "  make postgres-stop  - Stop PostgreSQL container"
	@echo "  make postgres-restart - Restart PostgreSQL container"
	@echo "  make postgres-remove - Remove PostgreSQL container"
	@echo "  make migrate        - Run database migrations"
	@echo ""
	@echo "Environment Variables:"
	@echo "  POSTGRES_DSN: $(POSTGRES_DSN)"
	@echo "  SURREALDB_URL: $(SURREALDB_URL)"