#!/usr/bin/env bash
set -euo pipefail

image="mllnd/sherlock"

docker buildx build -t "${image}" -f docker/Dockerfile .
