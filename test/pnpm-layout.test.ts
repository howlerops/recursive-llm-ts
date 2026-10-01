import { describe, it, expect, beforeAll, afterAll } from 'vitest';
import { execFileSync } from 'child_process';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import { pathToFileURL } from 'url';

// Simulates a pnpm install of the built package:
//
//   <fixture>/node_modules/recursive-llm-ts -> .pnpm/recursive-llm-ts@x/node_modules/recursive-llm-ts
//   <fixture>/node_modules/.pnpm/recursive-llm-ts@x/node_modules/
//     recursive-llm-ts/            (dist/ + package.json)
//     @recursive-llm/<platform> -> ../../../@recursive-llm+<platform>@x/node_modules/@recursive-llm/<platform>
//   <fixture>/node_modules/.pnpm/@recursive-llm+<platform>@x/node_modules/@recursive-llm/<platform>/bin/rlm-go
//
// and runs createBridge() from an unrelated cwd with RLM_GO_BINARY unset.

const repoRoot = path.resolve(__dirname, '..');
const hasBuild =
  fs.existsSync(path.join(repoRoot, 'dist', 'esm', 'bridge-factory.js')) &&
  fs.existsSync(path.join(repoRoot, 'dist', 'cjs', 'bridge-factory.js'));

const PLATFORM_PACKAGES: Record<string, string> = {
  'darwin-arm64': '@recursive-llm/darwin-arm64',
  'darwin-x64': '@recursive-llm/darwin-x64',
  'linux-x64': '@recursive-llm/linux-x64',
  'linux-arm64': '@recursive-llm/linux-arm64',
  'win32-x64': '@recursive-llm/win32-x64',
};
const platformPkg = PLATFORM_PACKAGES[`${process.platform}-${process.arch}`];
const binaryName = process.platform === 'win32' ? 'rlm-go.exe' : 'rlm-go';
const VERSION = '0.0.0-fixture';

// Compare canonical paths so Windows 8.3 short names in the temp dir don't cause false failures.
const canonical = (p: string) => fs.realpathSync.native(p);

function linkDir(target: string, linkPath: string): void {
  fs.mkdirSync(path.dirname(linkPath), { recursive: true });
  // Junctions need no special privileges on Windows; the type is ignored elsewhere.
  fs.symlinkSync(target, linkPath, 'junction');
}

function makeFixture(opts: { linkPlatformPackage: boolean }): { root: string; pkgDir: string; binary: string } {
  const root = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), 'rlm-pnpm-')));
  const store = path.join(root, 'node_modules', '.pnpm');

  const pkgDir = path.join(store, `recursive-llm-ts@${VERSION}`, 'node_modules', 'recursive-llm-ts');
  fs.mkdirSync(pkgDir, { recursive: true });
  fs.cpSync(path.join(repoRoot, 'dist'), path.join(pkgDir, 'dist'), { recursive: true });
  fs.writeFileSync(path.join(pkgDir, 'package.json'), JSON.stringify({ name: 'recursive-llm-ts', version: VERSION }));
  linkDir(pkgDir, path.join(root, 'node_modules', 'recursive-llm-ts'));

  const platformDir = path.join(store, `${platformPkg.replace('/', '+')}@${VERSION}`, 'node_modules', platformPkg);
  fs.mkdirSync(path.join(platformDir, 'bin'), { recursive: true });
  fs.writeFileSync(path.join(platformDir, 'package.json'), JSON.stringify({ name: platformPkg, version: VERSION }));
  const binary = path.join(platformDir, 'bin', binaryName);
  fs.writeFileSync(binary, '');

  if (opts.linkPlatformPackage) {
    linkDir(platformDir, path.join(store, `recursive-llm-ts@${VERSION}`, 'node_modules', platformPkg));
  }
  return { root, pkgDir, binary };
}

function run(args: string[]): { path: string; source: string; pkgRootDir: string } {
  const cwd = fs.mkdtempSync(path.join(os.tmpdir(), 'rlm-cwd-'));
  const env = { ...process.env };
  delete env.RLM_GO_BINARY;
  try {
    const out = execFileSync(process.execPath, args, { cwd, env, encoding: 'utf8' });
    return JSON.parse(out.trim().split('\n').pop()!);
  } finally {
    fs.rmSync(cwd, { recursive: true, force: true });
  }
}

function esmScript(entry: string): string[] {
  const url = (f: string) => JSON.stringify(pathToFileURL(path.join(entry, 'dist', 'esm', f)).href);
  return [
    '--input-type=module',
    '-e',
    `const { createBridge } = await import(${url('bridge-factory.js')});
     const { resolveGoBinary } = await import(${url('binary-resolver.js')});
     await createBridge();
     console.log(JSON.stringify(resolveGoBinary()));`,
  ];
}

function cjsScript(entry: string): string[] {
  const file = (f: string) => JSON.stringify(path.join(entry, 'dist', 'cjs', f));
  return [
    '-e',
    `const { createBridge } = require(${file('bridge-factory.js')});
     const { resolveGoBinary } = require(${file('binary-resolver.js')});
     createBridge().then(() => console.log(JSON.stringify(resolveGoBinary())), (e) => { console.error(e.message); process.exit(1); });`,
  ];
}

describe.skipIf(!hasBuild || !platformPkg)('pnpm layout (built output)', () => {
  const fixtures: string[] = [];
  let linked: ReturnType<typeof makeFixture>;
  let storeOnly: ReturnType<typeof makeFixture>;

  beforeAll(() => {
    linked = makeFixture({ linkPlatformPackage: true });
    storeOnly = makeFixture({ linkPlatformPackage: false });
    fixtures.push(linked.root, storeOnly.root);
  });

  afterAll(() => {
    for (const f of fixtures) fs.rmSync(f, { recursive: true, force: true });
  });

  for (const [format, script] of [['ESM', esmScript], ['CJS', cjsScript]] as const) {
    it(`${format}: resolves the platform package linked next to recursive-llm-ts`, () => {
      const entry = path.join(linked.root, 'node_modules', 'recursive-llm-ts');
      const result = run(script(entry));
      expect(canonical(result.pkgRootDir)).toBe(canonical(linked.pkgDir));
      expect(canonical(result.path)).toBe(canonical(linked.binary));
      expect(result.source).toBe('platform-package');
    });

    it(`${format}: finds the platform package in the .pnpm store when it is not linked`, () => {
      const entry = path.join(storeOnly.root, 'node_modules', 'recursive-llm-ts');
      const result = run(script(entry));
      expect(canonical(result.pkgRootDir)).toBe(canonical(storeOnly.pkgDir));
      expect(canonical(result.path)).toBe(canonical(storeOnly.binary));
      expect(result.source).toBe('platform-package');
    });
  }
});
