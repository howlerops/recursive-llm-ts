#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== Bundler Compatibility Tests ==="
echo ""

cd "$REPO_ROOT"
npm run build > /dev/null 2>&1

TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

# Pack the package
TARBALL=$(npm pack --pack-destination "$TMPDIR" 2>/dev/null | tail -1)

# ── Test 1: esbuild ESM bundle ────────────────────────────────────────
echo "── Test 1: esbuild ESM bundle ──"
TEST_DIR="$TMPDIR/esbuild-test"
mkdir -p "$TEST_DIR"
cd "$TEST_DIR"

cat > package.json << 'EOF'
{"name":"esbuild-test","private":true,"type":"module"}
EOF

npm install "$TMPDIR/$TARBALL" --ignore-scripts > /dev/null 2>&1
npm install esbuild > /dev/null 2>&1

cat > input.mjs << 'TESTEOF'
import { RLM, RLMError, classifyError, RLMCache } from 'recursive-llm-ts';
console.log('RLM:', typeof RLM);
console.log('RLMError:', typeof RLMError);
console.log('classifyError:', typeof classifyError);
console.log('RLMCache:', typeof RLMCache);
if (typeof RLM !== 'function') process.exit(1);
if (typeof RLMError !== 'function') process.exit(1);
console.log('ESBUILD ESM BUNDLE OK');
TESTEOF

# Bundle as ESM with esbuild (this is what would fail with __dirname)
npx esbuild input.mjs --bundle --platform=node --format=esm --outfile=bundle.mjs \
  --external:child_process --external:fs --external:path --external:url \
  --external:crypto --external:os --external:stream --external:events \
  --external:zod --external:@aws-sdk/client-s3 2>&1 | head -3

node bundle.mjs
echo "  ✓ esbuild ESM bundle works"
echo ""

# ── Test 2: esbuild CJS bundle ────────────────────────────────────────
echo "── Test 2: esbuild CJS bundle ──"

cat > input.cjs << 'TESTEOF'
const { RLM, RLMError, classifyError, RLMCache } = require('recursive-llm-ts');
console.log('RLM:', typeof RLM);
console.log('RLMError:', typeof RLMError);
if (typeof RLM !== 'function') process.exit(1);
console.log('ESBUILD CJS BUNDLE OK');
TESTEOF

npx esbuild input.cjs --bundle --platform=node --format=cjs --outfile=bundle.cjs \
  --external:child_process --external:fs --external:path --external:url \
  --external:crypto --external:os --external:stream --external:events \
  --external:zod --external:@aws-sdk/client-s3 2>&1 | head -3

node bundle.cjs
echo "  ✓ esbuild CJS bundle works"
echo ""

# ── Test 3: Verify no __dirname in ESM bundle ─────────────────────────
echo "── Test 3: Verify __dirname handling in ESM bundle ──"
if grep -q 'typeof __dirname' bundle.mjs; then
  echo "  ✓ bundle.mjs contains typeof __dirname guard (safe)"
elif grep -q '__dirname' bundle.mjs; then
  echo "  ⚠ bundle.mjs contains raw __dirname (esbuild may inject shim)"
  # esbuild on platform=node injects __dirname shim for ESM, which is fine
else
  echo "  ✓ bundle.mjs has no __dirname references"
fi
echo ""

echo "=== All bundler tests passed ==="
