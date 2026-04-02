import * as fs from 'fs';
import * as path from 'path';
import { spawn } from 'child_process';
import { Bridge, RLMConfig, RLMResult } from './bridge-interface';
import { PKG_ROOT_DIR } from './pkg-dir';

const DEFAULT_BINARY_NAME = process.platform === 'win32' ? 'rlm-go.exe' : 'rlm-go';

/** Platform-specific npm package names for pre-built binaries */
const PLATFORM_PACKAGES: Record<string, string> = {
  'darwin-arm64': '@recursive-llm/darwin-arm64',
  'darwin-x64': '@recursive-llm/darwin-x64',
  'linux-x64': '@recursive-llm/linux-x64',
  'linux-arm64': '@recursive-llm/linux-arm64',
  'win32-x64': '@recursive-llm/win32-x64',
};

/**
 * Walk up the directory tree from a starting point, looking for the platform
 * package binary inside node_modules directories. This handles pnpm's strict
 * isolation where require.resolve() cannot find optional dependencies.
 */
function findBinaryInNodeModules(pkgName: string): string | null {
  let dir = PKG_ROOT_DIR;
  const seenDirs = new Set<string>();
  while (dir && !seenDirs.has(dir)) {
    seenDirs.add(dir);

    // Standard node_modules layout (npm, yarn)
    const candidate = path.join(dir, 'node_modules', pkgName, 'bin', DEFAULT_BINARY_NAME);
    if (fs.existsSync(candidate)) return candidate;

    // pnpm hoisted node_modules (the package may be hoisted even under pnpm)
    // Also check .pnpm directory where pnpm stores the actual package contents
    const pnpmDir = path.join(dir, 'node_modules', '.pnpm');
    if (fs.existsSync(pnpmDir)) {
      // pnpm stores packages as: .pnpm/<name>@<version>/node_modules/<name>/
      // where scoped packages use + instead of / in the directory name
      const pnpmName = pkgName.replace('/', '+');
      try {
        const entries = fs.readdirSync(pnpmDir);
        for (const entry of entries) {
          if (entry.startsWith(pnpmName + '@')) {
            const pnpmCandidate = path.join(
              pnpmDir, entry, 'node_modules', pkgName, 'bin', DEFAULT_BINARY_NAME
            );
            if (fs.existsSync(pnpmCandidate)) return pnpmCandidate;
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
  return null;
}

function resolvePlatformBinary(): string | null {
  const platform = process.platform;
  const arch = process.arch;
  const key = `${platform}-${arch}`;
  const pkgName = PLATFORM_PACKAGES[key];
  if (!pkgName) return null;

  // 1. Try require.resolve (works with npm, yarn, and non-strict pnpm)
  try {
    const pkgDir = path.dirname(require.resolve(`${pkgName}/package.json`));
    const binPath = path.join(pkgDir, 'bin', DEFAULT_BINARY_NAME);
    if (fs.existsSync(binPath)) return binPath;
  } catch {
    // Package not resolvable via require — fall through to filesystem search
  }

  // 2. Walk up directory tree looking in node_modules (handles pnpm strict isolation)
  return findBinaryInNodeModules(pkgName);
}

function resolveBinaryPath(rlmConfig: RLMConfig): string {
  const configuredPath = rlmConfig.go_binary_path || process.env.RLM_GO_BINARY;
  if (configuredPath) {
    return configuredPath;
  }

  // 1. Try platform-specific npm package (pre-built binary)
  const platformBin = resolvePlatformBinary();
  if (platformBin) return platformBin;

  // 2. Try local locations
  const possiblePaths = [
    path.join(PKG_ROOT_DIR, 'bin', DEFAULT_BINARY_NAME),  // NPM package (primary)
    path.join(PKG_ROOT_DIR, 'go', DEFAULT_BINARY_NAME),   // Development fallback
  ];

  for (const p of possiblePaths) {
    if (fs.existsSync(p)) {
      return p;
    }
  }

  return possiblePaths[0]; // Return first path, error will be caught later
}

function assertBinaryExists(binaryPath: string): void {
  if (!fs.existsSync(binaryPath)) {
    throw new Error(
      `Go RLM binary not found at ${binaryPath}.\n` +
      'Build it with: node scripts/build-go-binary.js'
    );
  }
}

function sanitizeConfig(config: RLMConfig): { config: Record<string, unknown>, structured?: any } {
  const { go_binary_path, structured, ...sanitized } = config;
  return { config: sanitized, structured };
}

export class GoBridge implements Bridge {
  public async completion(
    model: string,
    query: string,
    context: string,
    rlmConfig: RLMConfig = {}
  ): Promise<RLMResult> {
    const binaryPath = resolveBinaryPath(rlmConfig);
    assertBinaryExists(binaryPath);

    const { config, structured } = sanitizeConfig(rlmConfig);
    const payload = JSON.stringify({
      model,
      query,
      context,
      config,
      structured
    });

    return new Promise<RLMResult>((resolve, reject) => {
      const child = spawn(binaryPath, [], { stdio: ['pipe', 'pipe', 'pipe'] });
      let stdout = '';
      let stderr = '';

      child.stdout.on('data', (data) => {
        stdout += data.toString();
      });

      child.stderr.on('data', (data) => {
        stderr += data.toString();
      });

      child.on('error', (error) => {
        reject(new Error(`Failed to start Go binary: ${error.message}`));
      });

      child.on('close', (code) => {
        if (code !== 0) {
          reject(new Error(stderr || `Go binary exited with code ${code}`));
          return;
        }

        try {
          const parsed = JSON.parse(stdout) as RLMResult;
          resolve(parsed);
        } catch (error: any) {
          reject(new Error(`Failed to parse Go response: ${error.message || error}`));
        }
      });

      child.stdin.write(payload);
      child.stdin.end();
    });
  }

  public async cleanup(): Promise<void> {
    // No persistent processes to clean up.
  }
}
