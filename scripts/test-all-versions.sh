#!/bin/bash
set -e

VERSIONS=("v2.6.0" "v3.0.0-beta.2")
PROTOCOLS=("ws" "http")

for VERSION in "${VERSIONS[@]}"; do
    for PROTOCOL in "${PROTOCOLS[@]}"; do
        echo "=========================================="
        echo "Testing against SurrealDB $VERSION ($PROTOCOL)"
        echo "=========================================="

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

        # Run tests
        echo "Running tests with SURREALDB_URL=$SURREALDB_URL..."
        go test -v -race ./... || {
            echo "Tests failed for SurrealDB $VERSION ($PROTOCOL)"
            docker rm -f surrealdb
            exit 1
        }

        # Cleanup
        docker rm -f surrealdb
        echo "Tests passed for SurrealDB $VERSION ($PROTOCOL)"
        echo ""
    done
done

echo "All versions and protocols passed!"
