#!/bin/bash
set -e

VERSIONS=("v2.6.0" "v3.0.0-beta.2")

for VERSION in "${VERSIONS[@]}"; do
    echo "=========================================="
    echo "Testing against SurrealDB $VERSION"
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

    # Run tests
    echo "Running tests..."
    go test -v -race ./... || {
        echo "Tests failed for SurrealDB $VERSION"
        docker rm -f surrealdb
        exit 1
    }

    # Cleanup
    docker rm -f surrealdb
    echo "Tests passed for SurrealDB $VERSION"
    echo ""
done

echo "All versions passed!"
