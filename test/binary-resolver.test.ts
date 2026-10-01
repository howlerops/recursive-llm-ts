import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { resolveGoBinary, requireGoBinary, GO_BINARY_NAME } from '../src/binary-resolver';
import { createBridge } from '../src/bridge-factory';
import { RLMBinaryError } from '../src/errors';
import { PKG_ROOT_DIR } from '../src/pkg-dir';

describe('resolveGoBinary (config vs env)', () => {
  let tmp: string;
  let fakeBinary: string;
  let savedEnv: string | undefined;

  beforeEach(() => {
    tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'rlm-resolver-'));
    fakeBinary = path.join(tmp, GO_BINARY_NAME);
    fs.writeFileSync(fakeBinary, '');
    savedEnv = process.env.RLM_GO_BINARY;
    delete process.env.RLM_GO_BINARY;
  });

  afterEach(() => {
    if (savedEnv === undefined) delete process.env.RLM_GO_BINARY;
    else process.env.RLM_GO_BINARY = savedEnv;
    fs.rmSync(tmp, { recursive: true, force: true });
  });

  it('honors go_binary_path without RLM_GO_BINARY', async () => {
    const resolution = resolveGoBinary({ go_binary_path: fakeBinary });
    expect(resolution.path).toBe(fakeBinary);
    expect(resolution.source).toBe('config');
    await expect(createBridge('go', { go_binary_path: fakeBinary })).resolves.toBeDefined();
  });

  it('honors RLM_GO_BINARY when no config path is given', () => {
    process.env.RLM_GO_BINARY = fakeBinary;
    const resolution = resolveGoBinary();
    expect(resolution.path).toBe(fakeBinary);
    expect(resolution.source).toBe('env');
  });

  it('prefers go_binary_path over RLM_GO_BINARY', () => {
    process.env.RLM_GO_BINARY = path.join(tmp, 'env-binary');
    expect(resolveGoBinary({ go_binary_path: fakeBinary }).source).toBe('config');
  });

  it('treats a missing explicit path as not found instead of falling back', async () => {
    const missing = path.join(tmp, 'does-not-exist');
    const resolution = resolveGoBinary({ go_binary_path: missing });
    expect(resolution.path).toBeNull();
    expect(resolution.explicit).toEqual({ path: missing, via: 'go_binary_path' });
    await expect(createBridge('go', { go_binary_path: missing })).rejects.toBeInstanceOf(RLMBinaryError);
  });

  it('reports package root, cwd, platform and every searched path when not found', () => {
    const missing = path.join(tmp, 'does-not-exist');
    process.env.RLM_GO_BINARY = missing;
    let error: unknown;
    try {
      requireGoBinary();
    } catch (e) {
      error = e;
    }
    expect(error).toBeInstanceOf(RLMBinaryError);
    const err = error as RLMBinaryError;
    expect(err.message).toContain(`Go RLM binary not found at ${missing} (set via RLM_GO_BINARY)`);
    expect(err.message).toContain(`package root: ${PKG_ROOT_DIR}`);
    expect(err.message).toContain(`cwd:          ${process.cwd()}`);
    expect(err.message).toContain(`platform:     ${process.platform}-${process.arch}`);
    expect(err.binaryPath).toBe(missing);
    expect(err.searched).toEqual([`${missing} (RLM_GO_BINARY)`]);
  });

  it('lists the package lookups it tried when nothing is configured', () => {
    const resolution = resolveGoBinary();
    // Whatever is found locally, the package-root locations must be among the candidates
    // unless an earlier step (the platform package) already succeeded.
    if (resolution.source !== 'platform-package') {
      expect(resolution.searched).toContain(path.join(PKG_ROOT_DIR, 'bin', GO_BINARY_NAME));
    }
    expect(resolution.searched.length).toBeGreaterThan(0);
  });
});
