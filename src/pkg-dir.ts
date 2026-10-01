/**
 * Portable package directory resolution.
 *
 * The compiled module's own directory comes from ./module-context, which has
 * a separate implementation per output format (__dirname for CJS,
 * import.meta.url for ESM). The resolved root always points to the package
 * root (parent of the dist/cjs or dist/esm directory).
 */
import * as path from 'path';
import { MODULE_DIR } from './module-context';

/** Directory containing the compiled JS file (dist/cjs or dist/esm or dist) */
export const PKG_DIST_DIR = MODULE_DIR;

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
