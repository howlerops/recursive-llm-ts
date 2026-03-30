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

function isPlatformBinaryAvailable(): boolean {
  const key = `${process.platform}-${process.arch}`;
  const pkgName = PLATFORM_PACKAGES[key];
  if (!pkgName) return false;
  try {
    const pkgDir = path.dirname(require.resolve(`${pkgName}/package.json`));
    return fs.existsSync(path.join(pkgDir, 'bin', DEFAULT_GO_BINARY));
  } catch {
    return false;
  }
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
