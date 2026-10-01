/**
 * Go binary resolution — the single source of truth used by both
 * createBridge() and GoBridge.
 *
 * Lookup order:
 *   1. RLMConfig.go_binary_path
 *   2. RLM_GO_BINARY environment variable
 *   3. Platform package (@recursive-llm/<platform>-<arch>), via require.resolve
 *      and then a node_modules walk up from the package root (pnpm layouts)
 *   4. <pkg-root>/bin/rlm-go (built by postinstall)
 *   5. <pkg-root>/go/rlm-go (development checkout)
 *
 * An explicit path (1 or 2) is authoritative: if it does not exist, resolution
 * fails rather than silently falling back to another binary.
 */
import * as fs from 'fs';
import * as path from 'path';
import { RLMBinaryError } from './errors';
import { moduleRequire } from './module-context';
import { PKG_ROOT_DIR } from './pkg-dir';

export const GO_BINARY_NAME = process.platform === 'win32' ? 'rlm-go.exe' : 'rlm-go';

/** Platform-specific npm package names for pre-built binaries */
export const PLATFORM_PACKAGES: Record<string, string> = {
  'darwin-arm64': '@recursive-llm/darwin-arm64',
  'darwin-x64': '@recursive-llm/darwin-x64',
  'linux-x64': '@recursive-llm/linux-x64',
  'linux-arm64': '@recursive-llm/linux-arm64',
  'win32-x64': '@recursive-llm/win32-x64',
};

export type GoBinarySource = 'config' | 'env' | 'platform-package' | 'package-bin' | 'dev';

export interface GoBinaryResolution {
  /** Resolved binary path, or null if none of the candidates exist */
  path: string | null;
  /** Which lookup step produced `path` (null when not found) */
  source: GoBinarySource | null;
  /** Every location checked, in order */
  searched: string[];
  /** `${process.platform}-${process.arch}` */
  platformKey: string;
  /** Package root used as the base for the package lookups */
  pkgRootDir: string;
  /** Working directory at resolution time (diagnostic only; not used for lookup) */
  cwd: string;
  /** Set when an explicit path (config or env) decided the result */
  explicit?: { path: string; via: 'go_binary_path' | 'RLM_GO_BINARY' };
}

export interface ResolveGoBinaryOptions {
  /** Explicit binary path (RLMConfig.go_binary_path) */
  go_binary_path?: string;
}

/**
 * Walk up from the package root looking for the platform package binary inside
 * node_modules directories, including pnpm's .pnpm store.
 */
function findInNodeModules(pkgName: string, searched: string[]): string | null {
  let dir = PKG_ROOT_DIR;
  const seenDirs = new Set<string>();
  while (dir && !seenDirs.has(dir)) {
    seenDirs.add(dir);

    // Standard node_modules layout (npm, yarn, hoisted pnpm)
    const candidate = path.join(dir, 'node_modules', pkgName, 'bin', GO_BINARY_NAME);
    searched.push(candidate);
    if (fs.existsSync(candidate)) return candidate;

    // pnpm stores packages as: .pnpm/<name>@<version>/node_modules/<name>/
    // where scoped packages use + instead of / in the directory name
    const pnpmDir = path.join(dir, 'node_modules', '.pnpm');
    if (fs.existsSync(pnpmDir)) {
      const pnpmName = pkgName.replace('/', '+');
      try {
        for (const entry of fs.readdirSync(pnpmDir)) {
          if (!entry.startsWith(pnpmName + '@')) continue;
          const pnpmCandidate = path.join(pnpmDir, entry, 'node_modules', pkgName, 'bin', GO_BINARY_NAME);
          searched.push(pnpmCandidate);
          if (fs.existsSync(pnpmCandidate)) return pnpmCandidate;
        }
      } catch {
        // Permission error or similar — skip
      }
    }

    const parent = path.dirname(dir);
    if (parent === dir) break;
    dir = parent;
  }
  return null;
}

function findPlatformBinary(pkgName: string, searched: string[]): string | null {
  // require.resolve follows Node's resolution from this module's real path,
  // which finds the optional dependency under npm, yarn and pnpm.
  try {
    const pkgDir = path.dirname(moduleRequire.resolve(`${pkgName}/package.json`));
    const candidate = path.join(pkgDir, 'bin', GO_BINARY_NAME);
    searched.push(candidate);
    if (fs.existsSync(candidate)) return candidate;
  } catch {
    searched.push(`require.resolve('${pkgName}/package.json') (not resolvable)`);
  }

  return findInNodeModules(pkgName, searched);
}

/** Resolve the Go binary without throwing. */
export function resolveGoBinary(options: ResolveGoBinaryOptions = {}): GoBinaryResolution {
  const platformKey = `${process.platform}-${process.arch}`;
  const searched: string[] = [];
  let explicit: GoBinaryResolution['explicit'];
  const result = (p: string | null, source: GoBinarySource | null): GoBinaryResolution => ({
    path: p,
    source: p ? source : null,
    searched,
    platformKey,
    pkgRootDir: PKG_ROOT_DIR,
    cwd: process.cwd(),
    ...(explicit ? { explicit } : {}),
  });

  const explicitCandidates: Array<[string | undefined, GoBinarySource]> = [
    [options.go_binary_path, 'config'],
    [process.env.RLM_GO_BINARY, 'env'],
  ];
  for (const [candidate, source] of explicitCandidates) {
    if (!candidate) continue;
    explicit = { path: candidate, via: source === 'config' ? 'go_binary_path' : 'RLM_GO_BINARY' };
    searched.push(`${candidate} (${explicit.via})`);
    return result(fs.existsSync(candidate) ? candidate : null, source);
  }

  const pkgName = PLATFORM_PACKAGES[platformKey];
  if (pkgName) {
    const platformBin = findPlatformBinary(pkgName, searched);
    if (platformBin) return result(platformBin, 'platform-package');
  } else {
    searched.push(`(no platform package for ${platformKey})`);
  }

  const local: Array<[string, GoBinarySource]> = [
    [path.join(PKG_ROOT_DIR, 'bin', GO_BINARY_NAME), 'package-bin'],
    [path.join(PKG_ROOT_DIR, 'go', GO_BINARY_NAME), 'dev'],
  ];
  for (const [candidate, source] of local) {
    searched.push(candidate);
    if (fs.existsSync(candidate)) return result(candidate, source);
  }

  return result(null, null);
}

/** Build the diagnostic message for a failed resolution. */
export function formatBinaryNotFound(resolution: GoBinaryResolution): string {
  const headline = resolution.explicit
    ? `Go RLM binary not found at ${resolution.explicit.path} (set via ${resolution.explicit.via}).`
    : 'Go RLM binary not found.';
  return [
    headline,
    `  package root: ${resolution.pkgRootDir}`,
    `  cwd:          ${resolution.cwd}`,
    `  platform:     ${resolution.platformKey}` +
      (PLATFORM_PACKAGES[resolution.platformKey] ? ` (${PLATFORM_PACKAGES[resolution.platformKey]})` : ' (no pre-built package)'),
    '  searched:',
    ...resolution.searched.map(p => `    - ${p}`),
    'Install the platform package, set RLM_GO_BINARY / go_binary_path, or build it with: node scripts/build-go-binary.js (requires Go 1.25+).',
  ].join('\n');
}

/** Resolve the Go binary, throwing an RLMBinaryError that lists every location checked. */
export function requireGoBinary(options: ResolveGoBinaryOptions = {}): string {
  const resolution = resolveGoBinary(options);
  if (resolution.path) return resolution.path;
  throw new RLMBinaryError({
    message: formatBinaryNotFound(resolution),
    binaryPath: resolution.explicit?.path ?? path.join(resolution.pkgRootDir, 'bin', GO_BINARY_NAME),
    searched: resolution.searched,
  });
}
