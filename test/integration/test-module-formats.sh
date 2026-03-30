#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== Module Format Integration Tests ==="
echo ""

# Ensure build is fresh
echo "Building package..."
cd "$REPO_ROOT"
npm run build > /dev/null 2>&1

# Create a temp directory for test projects
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

# Pack the package for local install
echo "Packing package..."
TARBALL=$(npm pack --pack-destination "$TMPDIR" 2>/dev/null | tail -1)
echo "  Packed: $TARBALL"
echo ""

# ── Test 1: CJS Consumer ──────────────────────────────────────────────
echo "── Test 1: CJS Consumer (no type field) ──"
CJS_DIR="$TMPDIR/cjs-test"
mkdir -p "$CJS_DIR"
cd "$CJS_DIR"

cat > package.json << 'EOF'
{"name":"cjs-test","private":true}
EOF

npm install "$TMPDIR/$TARBALL" --ignore-scripts > /dev/null 2>&1

cat > test.cjs << 'TESTEOF'
const { RLM, RLMError, classifyError, RLMCache, withRetry } = require('recursive-llm-ts');
const assert = (cond, msg) => { if (!cond) { console.error('FAIL:', msg); process.exit(1); } };

assert(typeof RLM === 'function', 'RLM should be a function');
assert(typeof RLMError === 'function', 'RLMError should be a function');
assert(typeof classifyError === 'function', 'classifyError should be a function');
assert(typeof RLMCache === 'function', 'RLMCache should be a function');
assert(typeof withRetry === 'function', 'withRetry should be a function');

console.log('  ✓ All CJS imports work correctly');
TESTEOF

node test.cjs
echo ""

# ── Test 2: ESM Consumer ──────────────────────────────────────────────
echo "── Test 2: ESM Consumer (type: module) ──"
ESM_DIR="$TMPDIR/esm-test"
mkdir -p "$ESM_DIR"
cd "$ESM_DIR"

cat > package.json << 'EOF'
{"name":"esm-test","private":true,"type":"module"}
EOF

npm install "$TMPDIR/$TARBALL" --ignore-scripts > /dev/null 2>&1

cat > test.mjs << 'TESTEOF'
import { RLM, RLMError, classifyError, RLMCache, withRetry } from 'recursive-llm-ts';
const assert = (cond, msg) => { if (!cond) { console.error('FAIL:', msg); process.exit(1); } };

assert(typeof RLM === 'function', 'RLM should be a function');
assert(typeof RLMError === 'function', 'RLMError should be a function');
assert(typeof classifyError === 'function', 'classifyError should be a function');
assert(typeof RLMCache === 'function', 'RLMCache should be a function');
assert(typeof withRetry === 'function', 'withRetry should be a function');

console.log('  ✓ All ESM imports work correctly');
TESTEOF

node test.mjs
echo ""

# ── Test 3: ESM with dynamic import ───────────────────────────────────
echo "── Test 3: ESM dynamic import ──"
cat > test-dynamic.mjs << 'TESTEOF'
const mod = await import('recursive-llm-ts');
const assert = (cond, msg) => { if (!cond) { console.error('FAIL:', msg); process.exit(1); } };

assert(typeof mod.RLM === 'function', 'Dynamic import: RLM should be a function');
assert(typeof mod.default === 'undefined' || typeof mod.RLM === 'function', 'Named exports should work');

console.log('  ✓ Dynamic ESM import works correctly');
TESTEOF

node test-dynamic.mjs
echo ""

# ── Test 4: Binary resolution paths exist in dist ─────────────────────
echo "── Test 4: Verify dist structure ──"
cd "$REPO_ROOT"

assert_file() {
  if [ ! -f "$1" ]; then
    echo "  ✗ Missing: $1"
    exit 1
  fi
  echo "  ✓ $1"
}

assert_file "dist/cjs/index.js"
assert_file "dist/cjs/index.d.ts"
assert_file "dist/cjs/pkg-dir.js"
assert_file "dist/cjs/go-bridge.js"
assert_file "dist/cjs/bridge-factory.js"
assert_file "dist/cjs/package.json"
assert_file "dist/esm/index.js"
assert_file "dist/esm/index.d.ts"
assert_file "dist/esm/pkg-dir.js"
assert_file "dist/esm/go-bridge.js"
assert_file "dist/esm/bridge-factory.js"
assert_file "dist/esm/package.json"

echo ""

# ── Test 5: Verify module type markers ────────────────────────────────
echo "── Test 5: Module type markers ──"
CJS_TYPE=$(node -p "require('$REPO_ROOT/dist/cjs/package.json').type")
ESM_TYPE=$(node -p "require('$REPO_ROOT/dist/esm/package.json').type")

if [ "$CJS_TYPE" = "commonjs" ]; then
  echo "  ✓ dist/cjs/package.json has type: commonjs"
else
  echo "  ✗ dist/cjs/package.json has wrong type: $CJS_TYPE"
  exit 1
fi

if [ "$ESM_TYPE" = "module" ]; then
  echo "  ✓ dist/esm/package.json has type: module"
else
  echo "  ✗ dist/esm/package.json has wrong type: $ESM_TYPE"
  exit 1
fi

echo ""

# ── Test 6: No __dirname in bridge files ──────────────────────────────
echo "── Test 6: Verify __dirname removed from bridge files ──"
if grep -q '__dirname' dist/cjs/go-bridge.js; then
  echo "  ✗ dist/cjs/go-bridge.js still contains __dirname"
  exit 1
fi
echo "  ✓ dist/cjs/go-bridge.js: no __dirname"

if grep -q '__dirname' dist/cjs/bridge-factory.js; then
  echo "  ✗ dist/cjs/bridge-factory.js still contains __dirname"
  exit 1
fi
echo "  ✓ dist/cjs/bridge-factory.js: no __dirname"

# pkg-dir.js SHOULD have __dirname (it's the one place that uses it safely)
if grep -q '__dirname' dist/cjs/pkg-dir.js; then
  echo "  ✓ dist/cjs/pkg-dir.js: uses __dirname (expected, with typeof guard)"
fi
echo ""

echo "=== All module format integration tests passed ==="
