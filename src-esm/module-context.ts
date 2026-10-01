/**
 * Per-format module context — ES module implementation.
 *
 * Compiled by tsconfig.esm-overrides.json into dist/esm/module-context.js,
 * replacing the CommonJS implementation in src/module-context.ts. Keep the
 * exported API identical to that file.
 */
import { createRequire } from 'module';
import * as path from 'path';
import { fileURLToPath } from 'url';

/** Directory containing this compiled module (dist/esm in the published package). */
export const MODULE_DIR: string = path.dirname(fileURLToPath(import.meta.url));

/** A require() function bound to this module, for optional/runtime-resolved dependencies. */
export const moduleRequire: NodeJS.Require = createRequire(import.meta.url);
