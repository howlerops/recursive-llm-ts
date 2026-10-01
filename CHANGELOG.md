# Changelog

## [5.2.11] - 2026-10-01

### Fixes
- **ESM package root** — The ESM build located the package via `new Function('return import.meta')`, which always throws, so it fell back to `process.cwd()` and looked for the Go binary relative to the caller's working directory. Per-format code now lives in `module-context` (a CJS implementation using `__dirname` and an ESM implementation using `import.meta.url`), so no output text is rewritten.
- **`require` in the ESM build** — Platform-package lookup and the optional `@aws-sdk/client-s3` loader now use `createRequire(import.meta.url)` in ESM instead of the missing global `require`.
- **One binary resolver** — `createBridge()` and `GoBridge` share `resolveGoBinary()`: `go_binary_path`, then `RLM_GO_BINARY`, then the platform package (`require.resolve`, then a `node_modules`/`.pnpm` walk), then `<root>/bin`, then `<root>/go`. `go_binary_path` now works without the env var, and an explicit path that does not exist is an error instead of a silent fallback.
- **Binary-not-found diagnostics** — `RLMBinaryError` now lists the package root, cwd, platform key and every path checked (also available as `error.searched`). `resolveGoBinary()` is exported for diagnostics.
- **Langfuse export** — `langfuse_enabled` previously only wrote a debug log. The Go engine now sends traces, generations (with token usage) and events to Langfuse's ingestion API, flushed at the end of each call. This is a Go change: it takes effect with a binary built from this source. The `@recursive-llm/*` optional dependencies stay pinned to 5.2.10 until new platform packages are published.
- **Go CLI** — Observability is flushed before the binary exits on an error.

### Internal
- Tests for the built ESM/CJS package root, a pnpm layout fixture, config vs env resolution, and Langfuse ingestion.
- CI and the publish workflow no longer mask `npm test` failures.

## [5.1.0] - 2026-03-17

### Breaking Changes
- **Removed Python bridges** — pythonia and bunpy bridges have been fully removed. The Go backend is now the only bridge
- **Removed `pythonia_timeout` config** — This config option no longer exists
- **`BridgeType` simplified** — Now only accepts `'go'` (was `'go' | 'pythonia' | 'bunpy' | 'auto'`)
- **`PythonBridge` interface renamed** — Now exported as `Bridge`

### Improvements
- **Prominent Go missing warning** — postinstall now shows a clear boxed warning when Go is not installed, explaining that the package will not work at runtime
- **`@aws-sdk/client-s3` peer dependency** — Declared as optional peer dependency so package managers can warn users who need S3 file storage
- **`exports` field in package.json** — Proper conditional exports for better ESM/CJS compatibility
- **Cleaner npm bundle** — Removed 2 unused source files (bunpy-bridge, rlm-bridge) from the package

## [4.6.0] - 2026-03-04

### Context Overflow Detection & Recovery
- **Automatic context overflow detection** - Classifies OpenAI, Azure, vLLM, and generic `context_length_exceeded` errors
- **6 reduction strategies** for automatic context recovery:
  - `mapreduce` (default) -- Parallel LLM summarization of chunks, then merge
  - `truncate` -- Zero-cost token dropping from the end
  - `chunked` -- Sequential extraction from each chunk
  - `tfidf` -- Pure Go TF-IDF extractive compression, zero API calls
  - `textrank` -- Graph-based PageRank sentence ranking with cosine similarity
  - `refine` -- Sequential iterative LLM refinement (LlamaIndex-style)
- **`ContextOverflowConfig`** -- New config section: `strategy`, `max_model_tokens`, `safety_margin`, `max_reduction_attempts`
- **`RLMContextOverflowError`** -- Typed error with `modelLimit`, `requestTokens`, and `suggestion`
- **`RLMBuilder.withContextOverflow()`** -- Fluent builder method for overflow config

### New Go Packages
- **`go/rlm/tfidf.go`** -- TF-IDF extractive compression: sentence splitting, tokenization, stop-word filtering, IDF scoring, budget-fitting selection
- **`go/rlm/textrank.go`** -- TextRank graph-based ranking: TF-IDF vector construction, cosine similarity graph, PageRank iteration with configurable damping/convergence
- **`go/rlm/context_overflow.go`** -- Core overflow handling: error detection, strategy dispatch, chunking, break-point detection

### Testing
- 150 Go tests: context overflow detection, all 6 strategies, TF-IDF sentence/token/score tests, TextRank graph/PageRank/compression tests
- 150 TypeScript vitest tests: error classification, config types, builder integration, serialization

## [4.5.0] - 2026-02-20

### Improvements
- Streaming, caching, retry, events, and error handling documentation updates
- GitHub Pages documentation improvements

## [4.4.0] - 2026-02-09

### ✨ New Features
- **File Storage Context** - Process local directories or S3 buckets as LLM context
  - `completionFromFiles()` and `structuredCompletionFromFiles()` methods on RLM class
  - `FileContextBuilder` for building structured context from file trees
  - `LocalFileStorage` for local filesystem traversal with recursive directory support
  - `S3FileStorage` for AWS S3, MinIO, LocalStack, DigitalOcean Spaces, Backblaze B2
  - Configurable filters: extensions, glob include/exclude patterns, max file/total size, max files
  - Per-file error recovery (failed reads are skipped, not fatal)
- **S3 Credential Chain** - Flexible credential resolution for S3 storage
  - Explicit credentials > environment variables > AWS SDK default chain
  - Environment variable support: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`
  - `AWS_DEFAULT_REGION` fallback alongside `AWS_REGION`
- **S3StorageError** - Typed error class with actionable messages
  - Error codes: `AUTH_FAILED`, `ACCESS_DENIED`, `BUCKET_NOT_FOUND`, `KEY_NOT_FOUND`, `NETWORK_ERROR`, `REGION_MISMATCH`
  - User-friendly remediation hints for each error type
- **S3-Compatible Services** - First-class support for non-AWS providers
  - Custom endpoint configuration with automatic `forcePathStyle`
  - Tested with MinIO, LocalStack, DigitalOcean Spaces, Backblaze B2

### 🔧 Fixed
- **Parallel structured output** - Rewrote JSON extraction with balanced-brace parser (replaces shallow regex)
  - Per-field schema wrapping eliminates ambiguity for non-object types
  - Automatic fallback from parallel to direct single-call on failure
  - Better error collection captures all parallel failures
- **Type exports** - Added missing exports: `SubTask`, `CoordinatorConfig`, `SchemaDecomposition`, `S3StorageError`
- **FileStorageConfig** - Consolidated to single canonical definition (was duplicated)
- **Debug leak** - Removed leftover `console.error('[RLM_DEBUG]...')` from production code

### 🧪 Testing
- 117 file storage tests: local operations, S3 construction, credential resolution, region resolution, error wrapping, endpoint configs, mock error scenarios, error recovery, forcePathStyle
- 44 Go tests for structured output: balanced JSON extraction, schema wrapping, validation, parallel merging

## [4.3.0] - 2026-01-23

### ✨ Major Improvements
- **Instructor-style retry logic**: Enhanced validation feedback with detailed error messages showing exactly what's missing
- **100% reliability**: Parallel execution now passes all reliability tests with nested objects
- **Comprehensive JSON Schema support**: Added support for all major constraint types

### 🔧 Fixed
- **Retry loop**: Now properly accumulates conversation history including previous attempts and validation feedback
- **Nested object extraction**: LLM now reliably returns all required fields in nested objects during parallel execution

### 📦 Added JSON Schema Support
- Number constraints: `minimum`, `maximum`, `multipleOf`
- String constraints: `minLength`, `maxLength`, `pattern`, `format`
- Array constraints: `minItems`, `maxItems`, `uniqueItems`
- Object constraints: `additionalProperties`
- Union/Intersection: `anyOf`, `allOf`

### 🎯 Enhanced Features
- **Example JSON generation**: Automatically generates example structures for nested objects
- **Detailed constraint messages**: Shows actual min/max values from schema in prompts
- **Better validation feedback**: Provides specific guidance on what fields are missing and their types

### 🧪 Testing
- Added comprehensive reliability test suite (`examples/test-reliability.ts`)
- Tests run 5 consecutive parallel extractions with 100% success rate
- Validates nested object extraction with required fields

## [4.2.0] - 2026-01-23

### Added
- Initial validation feedback in retry logic
- Basic constraint generation for nested objects

## [4.1.0] - Previous

### Features
- Zod schema to JSON Schema conversion
- Parallel execution for large schemas
- Recursive extraction service integration
