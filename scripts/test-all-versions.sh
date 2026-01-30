#!/bin/bash
set -e

VERSIONS=("v2.6.0" "v3.0.0-beta.2")
PROTOCOLS=("ws" "http")

# Create logs directory
LOGS_DIR="testlog"
mkdir -p "$LOGS_DIR"

for VERSION in "${VERSIONS[@]}"; do
    for PROTOCOL in "${PROTOCOLS[@]}"; do
        echo "=========================================="
        echo "Testing against SurrealDB $VERSION ($PROTOCOL)"
        echo "=========================================="

        # Log file name based on version and protocol
        LOG_FILE="$LOGS_DIR/${VERSION}_${PROTOCOL}.log"

        # Stop and remove any existing container
        docker rm -f surrealdb 2>/dev/null || true

        # Start SurrealDB
        docker run -d --name surrealdb -p 8000:8000 \
            surrealdb/surrealdb:$VERSION start --user root --pass root

        # Wait for SurrealDB to be ready
        echo "Waiting for SurrealDB to start..."
        for i in {1..30}; do
            if curl -sf http://localhost:8000/health > /dev/null 2>&1; then
                echo "SurrealDB is ready"
                break
            fi
            sleep 1
        done

        # Set the SURREALDB_URL based on protocol
        if [ "$PROTOCOL" = "http" ]; then
            export SURREALDB_URL="http://localhost:8000"
        else
            export SURREALDB_URL="ws://localhost:8000/rpc"
        fi

        # Run tests (use -count=1 to disable test caching)
        echo "Running tests with SURREALDB_URL=$SURREALDB_URL..."
        echo "Log file: $LOG_FILE"
        if ! go test -v -race -count=1 ./... > "$LOG_FILE" 2>&1; then
            echo "Tests failed for SurrealDB $VERSION ($PROTOCOL)"
            echo "See log file: $LOG_FILE"
            docker rm -f surrealdb
            exit 1
        fi

        # Cleanup
        docker rm -f surrealdb
        echo "Tests passed for SurrealDB $VERSION ($PROTOCOL)"
        echo ""
    done
done

echo "All versions and protocols passed!"
