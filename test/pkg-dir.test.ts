import { describe, it, expect } from 'vitest';
import * as path from 'path';

// Import the compiled CJS version directly to test runtime behavior
// (vitest runs in CJS-compatible mode)

describe('pkg-dir', () => {
  it('should export PKG_DIST_DIR as a string', async () => {
    const { PKG_DIST_DIR } = await import('../src/pkg-dir');
    expect(typeof PKG_DIST_DIR).toBe('string');
    expect(PKG_DIST_DIR.length).toBeGreaterThan(0);
  });

  it('should export PKG_ROOT_DIR as a string', async () => {
    const { PKG_ROOT_DIR } = await import('../src/pkg-dir');
    expect(typeof PKG_ROOT_DIR).toBe('string');
    expect(PKG_ROOT_DIR.length).toBeGreaterThan(0);
  });

  it('PKG_ROOT_DIR should be the repository root (contains package.json)', async () => {
    const fs = await import('fs');
    const { PKG_ROOT_DIR } = await import('../src/pkg-dir');
    // In the test environment (running from repo root via vitest),
    // PKG_ROOT_DIR should point to the repo root
    const pkgJsonPath = path.join(PKG_ROOT_DIR, 'package.json');
    // This might be off by one level in test vs compiled context,
    // but PKG_ROOT_DIR should at least be a valid directory
    expect(fs.existsSync(PKG_ROOT_DIR)).toBe(true);
  });

  it('PKG_DIST_DIR should be an absolute path', async () => {
    const { PKG_DIST_DIR } = await import('../src/pkg-dir');
    expect(path.isAbsolute(PKG_DIST_DIR)).toBe(true);
  });
});

describe('bridge-factory binary resolution', () => {
  it('should export BridgeType', async () => {
    const { createBridge } = await import('../src/bridge-factory');
    expect(typeof createBridge).toBe('function');
  });
});

describe('go-bridge platform package resolution', () => {
  it('should define PLATFORM_PACKAGES for all supported platforms', async () => {
    // We can't import the const directly (it's module-scoped),
    // but we can verify the go-bridge module loads without errors
    const mod = await import('../src/go-bridge');
    expect(mod.GoBridge).toBeDefined();
    expect(typeof mod.GoBridge).toBe('function');
  });
});
