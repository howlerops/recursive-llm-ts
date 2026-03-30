#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== Docker Integration Tests ==="
echo "Testing dual CJS/ESM module format in Docker containers"
echo ""

# Build from repo root context
cd "$REPO_ROOT"

echo "── Test 1: CJS Consumer ──"
docker build -f test/docker/Dockerfile.cjs-consumer -t rlm-test-cjs . 2>&1 | tail -5
docker run --rm rlm-test-cjs
echo ""

echo "── Test 2: ESM Consumer ──"
docker build -f test/docker/Dockerfile.esm-consumer -t rlm-test-esm . 2>&1 | tail -5
docker run --rm rlm-test-esm
echo ""

echo "=== All Docker tests passed ==="
