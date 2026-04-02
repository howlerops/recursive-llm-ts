import * as fs from 'fs';
import * as path from 'path';
import { Bridge } from './bridge-interface';
import { PKG_ROOT_DIR } from './pkg-dir';

export type BridgeType = 'go';

const DEFAULT_GO_BINARY = process.platform === 'win32' ? 'rlm-go.exe' : 'rlm-go';

function resolveDefaultGoBinary(): string {
  return path.join(PKG_ROOT_DIR, 'bin', DEFAULT_GO_BINARY);
}

/** Platform-specific npm package names for pre-built binaries */
const PLATFORM_PACKAGES: Record<string, string> = {
  'darwin-arm64': '@recursive-llm/darwin-arm64',
  'darwin-x64': '@recursive-llm/darwin-x64',
  'linux-x64': '@recursive-llm/linux-x64',
  'linux-arm64': '@recursive-llm/linux-arm64',
  'win32-x64': '@recursive-llm/win32-x64',
};

/**
 * Walk up the directory tree from PKG_ROOT_DIR, looking for the platform
 * package binary inside node_modules directories. This handles pnpm's strict
 * isolation where require.resolve() cannot find optional dependencies.
 */
function findBinaryInNodeModules(pkgName: string): boolean {
  let dir = PKG_ROOT_DIR;
  const seenDirs = new Set<string>();
  while (dir && !seenDirs.has(dir)) {
    seenDirs.add(dir);

    // Standard node_modules layout (npm, yarn)
    const candidate = path.join(dir, 'node_modules', pkgName, 'bin', DEFAULT_GO_BINARY);
    if (fs.existsSync(candidate)) return true;

    // pnpm .pnpm directory
    const pnpmDir = path.join(dir, 'node_modules', '.pnpm');
    if (fs.existsSync(pnpmDir)) {
      const pnpmName = pkgName.replace('/', '+');
      try {
        const entries = fs.readdirSync(pnpmDir);
        for (const entry of entries) {
          if (entry.startsWith(pnpmName + '@')) {
            const pnpmCandidate = path.join(
              pnpmDir, entry, 'node_modules', pkgName, 'bin', DEFAULT_GO_BINARY
            );
            if (fs.existsSync(pnpmCandidate)) return true;
          }
        }
      } catch {
        // Permission error or similar — skip
      }
    }

    const parent = path.dirname(dir);
    if (parent === dir) break;
    dir = parent;
  }
  return false;
}

function isPlatformBinaryAvailable(): boolean {
  const key = `${process.platform}-${process.arch}`;
  const pkgName = PLATFORM_PACKAGES[key];
  if (!pkgName) return false;

  // 1. Try require.resolve (works with npm, yarn, and non-strict pnpm)
  try {
    const pkgDir = path.dirname(require.resolve(`${pkgName}/package.json`));
    if (fs.existsSync(path.join(pkgDir, 'bin', DEFAULT_GO_BINARY))) return true;
  } catch {
    // Fall through to filesystem search
  }

  // 2. Walk up directory tree looking in node_modules (handles pnpm strict isolation)
  return findBinaryInNodeModules(pkgName);
}

function isGoBinaryAvailable(): boolean {
  const envPath = process.env.RLM_GO_BINARY;
  if (envPath && fs.existsSync(envPath)) {
    return true;
  }
  if (isPlatformBinaryAvailable()) return true;
  return fs.existsSync(resolveDefaultGoBinary());
}

/**
 * Create the Go bridge for RLM communication.
 * Throws if the Go binary is not available.
 */
export async function createBridge(bridgeType: BridgeType = 'go'): Promise<Bridge> {
  if (!isGoBinaryAvailable()) {
    throw new Error(
      'Go RLM binary not found. Build it with: node scripts/build-go-binary.js\n' +
      'Ensure Go 1.25+ is installed: https://go.dev/dl/'
    );
  }

  const { GoBridge } = await import('./go-bridge');
  return new GoBridge();
}
