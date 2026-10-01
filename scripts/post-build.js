#!/usr/bin/env node
/**
 * Post-build script:
 * 1. Writes package.json marker files into dist/cjs and dist/esm
 * 2. Adds .js extensions to ESM import/export paths (required by Node ESM)
 * 3. Gives the ESM pkg-dir a real `import.meta` so it resolves its own location
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

// ── Step 3: Real import.meta in the ESM pkg-dir ────────────────────────
// src/pkg-dir.ts must compile for CJS too, so it reaches import.meta via
// `new Function('return import.meta')()`. That can never work: Function bodies
// are parsed as scripts, so it throws and pkg-dir falls back to process.cwd().
// In ESM consumers (e.g. an app started from /app) PKG_ROOT_DIR then resolves to
// '/', and the platform Go binary is never found. Use import.meta directly here.
const esmPkgDir = path.join(esmDir, 'pkg-dir.js');
const indirectImportMeta = "new Function('return import.meta')()";
const pkgDirSource = fs.readFileSync(esmPkgDir, 'utf8');
const occurrences = pkgDirSource.split(indirectImportMeta).length - 1;
if (occurrences !== 1) {
  throw new Error(
    `[recursive-llm-ts] expected exactly one indirect import.meta in ${esmPkgDir}, found ${occurrences}`
  );
}
fs.writeFileSync(esmPkgDir, pkgDirSource.replace(indirectImportMeta, 'import.meta'));
console.log('[recursive-llm-ts] ✓ ESM pkg-dir uses import.meta');
