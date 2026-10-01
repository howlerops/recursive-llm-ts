/**
 * Portable package directory resolution.
 *
 * Works in both CommonJS and ESM contexts by detecting the available
 * globals and falling back gracefully. The resolved path always points
 * to the package root (parent of the dist/cjs or dist/esm directory).
 */
import * as path from 'path';
import { fileURLToPath } from 'url';

function resolveCurrentDir(): string {
  // CJS — __dirname is defined natively by Node
  if (typeof __dirname !== 'undefined') {
    return __dirname;
  }

  // ESM — derive from import.meta.url. `new Function` keeps this file compilable for CJS,
  // but cannot see import.meta itself; scripts/post-build.js rewrites it to a real
  // `import.meta` in dist/esm/pkg-dir.js.
  try {
    const meta = new Function('return import.meta')() as { url: string };
    if (meta && typeof meta.url === 'string') {
      return path.dirname(fileURLToPath(meta.url));
    }
  } catch {
    // Not in ESM or import.meta not supported
  }

  // Last resort: use process.cwd()
  return process.cwd();
}

/** Directory containing the compiled JS file (dist/cjs or dist/esm or dist) */
export const PKG_DIST_DIR = resolveCurrentDir();

/**
 * Package root directory.
 * Handles both flat (dist/) and nested (dist/cjs/, dist/esm/) layouts.
 */
export const PKG_ROOT_DIR = (() => {
  const parent = path.dirname(PKG_DIST_DIR);
  const parentBase = path.basename(parent);
  // If parent is 'dist', we're in dist/cjs or dist/esm — go up one more
  if (parentBase === 'dist') {
    return path.dirname(parent);
  }
  return parent;
})();
