#!/usr/bin/env bash
set -euo pipefail

image="mllnd/sherlock"
version="${VERSION:-v0.0.0}"
commit="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}"

docker buildx build \
    --build-arg VERSION="${version}" \
    --build-arg COMMIT="${commit}" \
    -t "${image}:${version}-${commit}" \
    -t "${image}:latest" \
    -f docker/Dockerfile \
    .
