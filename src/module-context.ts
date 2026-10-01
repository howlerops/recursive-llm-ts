/**
 * Per-format module context — CommonJS implementation.
 *
 * This file is compiled into dist/cjs. The ESM build replaces it with
 * src-esm/module-context.ts (compiled by tsconfig.esm-overrides.json), which
 * exposes the same API using import.meta.url. Keep the two files in sync.
 */

/** Directory containing this compiled module (dist/cjs in the published package). */
export const MODULE_DIR: string = __dirname;

/** A require() function bound to this module, for optional/runtime-resolved dependencies. */
export const moduleRequire: NodeJS.Require = require;
