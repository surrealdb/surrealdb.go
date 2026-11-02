#!/bin/bash
# Checks out github.com/surrealdb/surrealdb-protocol at the release the CBOR
# conformance vector tests (surrealcbor/conformance_vectors_test.go) are
# pinned to, and prints the line that points the tests at it:
#
#   eval "$(scripts/fetch-surrealdb-protocol.sh)"
#   go test ./surrealcbor -run TestConformanceVectors
#
# The checkout goes to .surrealdb-protocol at the repository root (ignored by
# git), or to the directory given as the first argument. CI checks out the same
# tag (see .github/workflows/test.yml); keep the two in step.
#
# The vectors are fetched rather than vendored because this release of
# surrealdb-protocol is not under an open-source licence; they can be vendored
# once a release is.
set -euo pipefail

TAG="v0.14.0"
# The annotated tag object, so a moved tag is noticed.
TAG_OBJECT="be258307dfd989a232ff81ace6f16a6bbcaf991d"
REPO="https://github.com/surrealdb/surrealdb-protocol.git"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIR="${1:-$ROOT/.surrealdb-protocol}"

if [ -d "$DIR/.git" ]; then
	git -C "$DIR" fetch --quiet --depth 1 origin "refs/tags/$TAG:refs/tags/$TAG" >&2
else
	git -c advice.detachedHead=false clone --quiet --depth 1 --branch "$TAG" "$REPO" "$DIR" >&2
fi
git -C "$DIR" -c advice.detachedHead=false checkout --quiet "$TAG" >&2

GOT="$(git -C "$DIR" rev-parse "refs/tags/$TAG")"
if [ "$GOT" != "$TAG_OBJECT" ]; then
	echo "surrealdb-protocol tag $TAG is $GOT, expected $TAG_OBJECT" >&2
	exit 1
fi

echo "export SURREALDB_PROTOCOL_DIR=\"$(cd "$DIR" && pwd)\""
