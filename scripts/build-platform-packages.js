#!/usr/bin/env node
/**
 * Build platform-specific npm packages for pre-built Go binaries.
 *
 * Each package contains only a single pre-compiled binary for one
 * OS+arch combo. The main recursive-llm-ts package lists them as
 * optionalDependencies — npm/pnpm/yarn will install only the one
 * matching the user's platform.
 *
 * Usage:
 *   node scripts/build-platform-packages.js [--version 5.1.1] [--out-dir platform-packages]
 *
 * This script:
 *   1. Cross-compiles the Go binary for all supported platforms
 *   2. Generates a minimal npm package for each platform
 *   3. Outputs them to <out-dir>/<pkg-name>/ ready for `npm publish`
 */
const { execFileSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const PLATFORMS = [
  { os: 'darwin',  arch: 'arm64', node_platform: 'darwin',  node_arch: 'arm64', pkg: '@recursive-llm/darwin-arm64' },
  { os: 'darwin',  arch: 'amd64', node_platform: 'darwin',  node_arch: 'x64',   pkg: '@recursive-llm/darwin-x64' },
  { os: 'linux',   arch: 'amd64', node_platform: 'linux',   node_arch: 'x64',   pkg: '@recursive-llm/linux-x64' },
  { os: 'linux',   arch: 'arm64', node_platform: 'linux',   node_arch: 'arm64', pkg: '@recursive-llm/linux-arm64' },
  { os: 'windows', arch: 'amd64', node_platform: 'win32',   node_arch: 'x64',   pkg: '@recursive-llm/win32-x64' },
];

// ── Parse CLI args ─────────────────────────────────────────────────────────
const args = process.argv.slice(2);
let version = null;
let outDir = 'platform-packages';
let skipBuild = false;
let binariesDir = null;

for (let i = 0; i < args.length; i++) {
  if (args[i] === '--version' && args[i + 1]) version = args[++i];
  else if (args[i] === '--out-dir' && args[i + 1]) outDir = args[++i];
  else if (args[i] === '--skip-build') skipBuild = true;
  else if (args[i] === '--binaries-dir' && args[i + 1]) binariesDir = args[++i];
}

if (!version) {
  const pkgJson = JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'package.json'), 'utf8'));
  version = pkgJson.version;
}

const repoRoot = path.join(__dirname, '..');
const goRoot = path.join(repoRoot, 'go');
const outBase = path.resolve(repoRoot, outDir);

console.log(`Building platform packages v${version} → ${outBase}`);

function goAvailable() {
  try {
    execFileSync('go', ['version'], { stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
}

if (!skipBuild && !goAvailable()) {
  console.error('Go is required to cross-compile binaries. Install from https://go.dev/dl/');
  console.error('Or pass --skip-build --binaries-dir <path> to use pre-built binaries.');
  process.exit(1);
}

// ── Build each platform ────────────────────────────────────────────────────
for (const plat of PLATFORMS) {
  const binaryName = plat.os === 'windows' ? 'rlm-go.exe' : 'rlm-go';
  const pkgDir = path.join(outBase, plat.pkg.replace('/', '-'));
  const binDir = path.join(pkgDir, 'bin');

  console.log(`\n── ${plat.pkg} (${plat.os}/${plat.arch}) ──`);

  // Create package directory structure
  fs.mkdirSync(binDir, { recursive: true });

  // Build or copy binary
  const targetBinary = path.join(binDir, binaryName);

  if (skipBuild && binariesDir) {
    // Copy pre-built binary
    const srcName = plat.os === 'windows'
      ? `rlm-${plat.os}-${plat.arch}.exe`
      : `rlm-${plat.os}-${plat.arch}`;
    const srcPath = path.join(binariesDir, srcName);
    if (!fs.existsSync(srcPath)) {
      console.warn(`  ⚠ Binary not found at ${srcPath}, skipping`);
      continue;
    }
    fs.copyFileSync(srcPath, targetBinary);
    console.log(`  ✓ Copied from ${srcPath}`);
  } else {
    // Cross-compile
    try {
      const goarch = plat.arch;
      const env = {
        ...process.env,
        GOOS: plat.os,
        GOARCH: goarch,
        CGO_ENABLED: '0',
      };
      execFileSync('go', [
        'build',
        '-ldflags=-s -w',
        '-o', targetBinary,
        './cmd/rlm',
      ], { stdio: 'inherit', cwd: goRoot, env });
      console.log(`  ✓ Built ${binaryName}`);
    } catch (err) {
      console.error(`  ✗ Build failed for ${plat.os}/${plat.arch}: ${err.message}`);
      continue;
    }
  }

  // Make binary executable (non-Windows)
  if (plat.os !== 'windows') {
    fs.chmodSync(targetBinary, 0o755);
  }

  // Write package.json
  const pkgJson = {
    name: plat.pkg,
    version,
    description: `Pre-built recursive-llm-ts Go binary for ${plat.node_platform}-${plat.node_arch}`,
    os: [plat.node_platform],
    cpu: [plat.node_arch],
    main: `bin/${binaryName}`,
    files: ['bin'],
    license: 'MIT',
    repository: {
      type: 'git',
      url: 'git+https://github.com/howlerops/recursive-llm-ts.git',
    },
    publishConfig: {
      access: 'public',
    },
  };

  fs.writeFileSync(
    path.join(pkgDir, 'package.json'),
    JSON.stringify(pkgJson, null, 2) + '\n'
  );

  // Write README
  fs.writeFileSync(
    path.join(pkgDir, 'README.md'),
    [
      `# ${plat.pkg}`,
      '',
      `Pre-built Go binary for \`recursive-llm-ts\` on ${plat.node_platform}-${plat.node_arch}.`,
      '',
      'This package is automatically installed as an optional dependency of `recursive-llm-ts`.',
      'You should not need to install it directly.',
      '',
      '## Platform',
      '',
      `- OS: ${plat.node_platform}`,
      `- Architecture: ${plat.node_arch}`,
      '',
    ].join('\n')
  );

  console.log(`  ✓ Package ready at ${pkgDir}`);
}

console.log(`\nAll platform packages written to ${outBase}`);
console.log('To publish: cd into each package directory and run `npm publish`');
