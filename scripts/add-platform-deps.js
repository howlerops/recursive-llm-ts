#!/usr/bin/env node
/**
 * Add @recursive-llm/* platform packages as optionalDependencies.
 *
 * Run this after the platform packages have been published to npm for the first time.
 * After that, the release.sh script will keep the versions in sync automatically.
 *
 * Usage: node scripts/add-platform-deps.js [version]
 *        If version is omitted, uses the current package.json version.
 */
const fs = require('fs');
const path = require('path');

const pkgPath = path.join(__dirname, '..', 'package.json');
const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));
const version = process.argv[2] || pkg.version;

const PLATFORMS = [
  '@recursive-llm/darwin-arm64',
  '@recursive-llm/darwin-x64',
  '@recursive-llm/linux-x64',
  '@recursive-llm/linux-arm64',
  '@recursive-llm/win32-x64',
];

pkg.optionalDependencies = pkg.optionalDependencies || {};
for (const p of PLATFORMS) {
  pkg.optionalDependencies[p] = version;
}

fs.writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + '\n');
console.log(`Added optionalDependencies for ${PLATFORMS.length} platform packages at v${version}`);
