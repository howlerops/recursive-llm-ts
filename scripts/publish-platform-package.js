#!/usr/bin/env node
/**
 * Build and publish a single platform-specific npm package.
 *
 * Usage: node scripts/publish-platform-package.js <platform> <arch> <version> <binary-path>
 *
 * Example: node scripts/publish-platform-package.js darwin arm64 5.2.5 bin/rlm-go
 *
 * This creates a temp package directory, copies the binary in, and runs npm publish.
 * Designed to be called from CI where each platform gets its own job (for OIDC provenance).
 */
const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const [platform, arch, version, binaryPath] = process.argv.slice(2);

if (!platform || !arch || !version || !binaryPath) {
  console.error('Usage: publish-platform-package.js <platform> <arch> <version> <binary-path>');
  process.exit(1);
}

const PLATFORM_MAP = {
  'darwin-arm64': '@recursive-llm/darwin-arm64',
  'darwin-x64':   '@recursive-llm/darwin-x64',
  'linux-x64':    '@recursive-llm/linux-x64',
  'linux-arm64':  '@recursive-llm/linux-arm64',
  'win32-x64':    '@recursive-llm/win32-x64',
};

const key = `${platform}-${arch}`;
const pkgName = PLATFORM_MAP[key];
if (!pkgName) {
  console.error(`Unknown platform: ${key}`);
  console.error(`Valid platforms: ${Object.keys(PLATFORM_MAP).join(', ')}`);
  process.exit(1);
}

if (!fs.existsSync(binaryPath)) {
  console.error(`Binary not found: ${binaryPath}`);
  process.exit(1);
}

const binaryName = path.basename(binaryPath);

// Create package directory
const pkgDir = path.join('_platform-pkg');
const binDir = path.join(pkgDir, 'bin');
fs.mkdirSync(binDir, { recursive: true });

// Copy binary
fs.copyFileSync(binaryPath, path.join(binDir, binaryName));
if (platform !== 'win32') {
  fs.chmodSync(path.join(binDir, binaryName), 0o755);
}

// Write package.json
const pkgJson = {
  name: pkgName,
  version,
  description: `Pre-built recursive-llm-ts Go binary for ${platform}-${arch}`,
  os: [platform],
  cpu: [arch],
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

fs.writeFileSync(path.join(pkgDir, 'package.json'), JSON.stringify(pkgJson, null, 2) + '\n');

console.log(`Publishing ${pkgName}@${version}...`);
console.log(`  Binary: ${binaryPath} (${(fs.statSync(binaryPath).size / 1024 / 1024).toFixed(1)} MB)`);

try {
  execSync('npm publish --access public', {
    cwd: pkgDir,
    stdio: 'inherit',
  });
  console.log(`✓ Published ${pkgName}@${version}`);
} catch (err) {
  console.error(`✗ Failed to publish ${pkgName}@${version}`);
  process.exit(1);
}
