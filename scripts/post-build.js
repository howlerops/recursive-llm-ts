#!/usr/bin/env node
/**
 * Post-build script:
 * 1. Writes package.json marker files into dist/cjs and dist/esm
 * 2. Adds .js extensions to ESM import/export paths (required by Node ESM)
 */
const fs = require('fs');
const path = require('path');

const distDir = path.join(__dirname, '..', 'dist');

// ── Step 1: Module type markers ────────────────────────────────────────
fs.writeFileSync(
  path.join(distDir, 'cjs', 'package.json'),
  JSON.stringify({ type: 'commonjs' }, null, 2) + '\n'
);

fs.writeFileSync(
  path.join(distDir, 'esm', 'package.json'),
  JSON.stringify({ type: 'module' }, null, 2) + '\n'
);

console.log('[recursive-llm-ts] ✓ Module type markers written');

// ── Step 2: Fix ESM import paths ───────────────────────────────────────
// Node ESM requires explicit .js extensions in import specifiers.
// TypeScript does NOT add them, so we do it here.

const esmDir = path.join(distDir, 'esm');

function fixEsmImports(dir) {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      fixEsmImports(fullPath);
      continue;
    }
    if (!entry.name.endsWith('.js')) continue;

    let content = fs.readFileSync(fullPath, 'utf8');
    let changed = false;

    // Match: import ... from './foo'  or  export ... from './foo'
    // Add .js extension to relative specifiers that don't already have one
    const updated = content.replace(
      /((?:import|export)\s+(?:(?:\{[^}]*\}|[^;'"]*)\s+from\s+)?['"])(\.\.?\/[^'"]+)(['"])/g,
      (match, prefix, specifier, quote) => {
        // Skip if already has an extension
        if (/\.\w+$/.test(specifier)) return match;
        changed = true;
        return `${prefix}${specifier}.js${quote}`;
      }
    );

    // Also fix dynamic imports: import('./foo')
    const updated2 = updated.replace(
      /(import\s*\(\s*['"])(\.\.?\/[^'"]+)(['"]\s*\))/g,
      (match, prefix, specifier, suffix) => {
        if (/\.\w+$/.test(specifier)) return match;
        changed = true;
        return `${prefix}${specifier}.js${suffix}`;
      }
    );

    if (changed) {
      fs.writeFileSync(fullPath, updated2);
    }
  }
}

fixEsmImports(esmDir);
console.log('[recursive-llm-ts] ✓ ESM import paths fixed (.js extensions added)');
