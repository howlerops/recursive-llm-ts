import { describe, it, expect } from 'vitest';
import { execFileSync } from 'child_process';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { pathToFileURL } from 'url';

// Exercises the built output the way consumers load it: from an unrelated working
// directory, so a process.cwd() fallback cannot accidentally produce the right answer.
const repoRoot = path.resolve(__dirname, '..');
const esmPkgDir = path.join(repoRoot, 'dist', 'esm', 'pkg-dir.js');
const cjsPkgDir = path.join(repoRoot, 'dist', 'cjs', 'pkg-dir.js');
const hasBuild = fs.existsSync(esmPkgDir) && fs.existsSync(cjsPkgDir);

function rootFrom(script: string, args: string[]): string {
  const cwd = fs.mkdtempSync(path.join(os.tmpdir(), 'rlm-pkg-dir-'));
  try {
    return execFileSync(process.execPath, [...args, script], { cwd, encoding: 'utf8' }).trim();
  } finally {
    fs.rmSync(cwd, { recursive: true, force: true });
  }
}

describe.skipIf(!hasBuild)('pkg-dir (built output)', () => {
  it('resolves the package root from the ESM build regardless of cwd', () => {
    const script = `import(${JSON.stringify(pathToFileURL(esmPkgDir).href)}).then((m) => console.log(m.PKG_ROOT_DIR))`;
    expect(rootFrom(script, ['--input-type=module', '-e'])).toBe(repoRoot);
  });

  it('resolves the package root from the CJS build regardless of cwd', () => {
    const script = `console.log(require(${JSON.stringify(cjsPkgDir)}).PKG_ROOT_DIR)`;
    expect(rootFrom(script, ['-e'])).toBe(repoRoot);
  });
});
