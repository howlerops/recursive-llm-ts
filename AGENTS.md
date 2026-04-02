# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

## Build Commands

```bash
# TypeScript (dual CJS + ESM output)
npm run build          # Compile src/ → dist/cjs/ + dist/esm/ + module markers
npm run build:cjs      # CJS only
npm run build:esm      # ESM only
npm install            # Install deps + auto-build Go binary via postinstall

# Go binary (manual)
cd go && go build -ldflags="-s -w" -o ../bin/rlm-go ./cmd/rlm

# Platform packages (cross-compile Go binaries for npm distribution)
node scripts/build-platform-packages.js                    # Build all platforms (requires Go)
node scripts/build-platform-packages.js --version 5.2.0    # Explicit version
node scripts/build-platform-packages.js --skip-build --binaries-dir ./bins  # Use pre-built binaries

# Release
./scripts/release.sh   # Interactive version bump, build, tag, push
```

### Module Format

The package ships dual CJS + ESM builds:
- `dist/cjs/` — CommonJS output (with `{"type":"commonjs"}` marker)
- `dist/esm/` — ES Module output (with `{"type":"module"}` marker)

Consumers with `"type": "module"` in their package.json get ESM automatically via the `"import"` export condition. CJS consumers get CommonJS via `"require"`.

### Platform Binary Distribution

Pre-built Go binaries are distributed via platform-specific npm packages:
- `@recursive-llm/darwin-arm64` — macOS Apple Silicon
- `@recursive-llm/darwin-x64` — macOS Intel
- `@recursive-llm/linux-x64` — Linux x86_64
- `@recursive-llm/linux-arm64` — Linux ARM64
- `@recursive-llm/win32-x64` — Windows x86_64

These are listed as `optionalDependencies` — npm installs only the matching platform. The `postinstall` script skips Go compilation if a pre-built binary is found.

## Testing

```bash
# TypeScript tests (no npm test configured - run directly)
ts-node test/test.ts                    # Main integration test
ts-node test/test-go.ts                 # Go bridge tests
ts-node test/structured-test.ts         # Structured output tests
ts-node test/zod-schema-converter-test.ts  # Schema conversion tests
ts-node test/new-features-test.ts       # Meta-agent, observability, config tests
ts-node test/file-storage-test.ts       # File storage tests (local + S3, 117 tests)

# Go tests
cd go
go test ./rlm/... -v                    # All tests verbose
go test ./rlm -run TestParser -v        # Single test
go test ./rlm -run TestSchema -v        # Schema validation tests
go test ./rlm -run TestObserver -v      # Observability tests
go test ./rlm -run TestMetaAgent -v     # Meta-agent tests
go test ./rlm -run TestExtract -v       # Balanced JSON extraction tests
go test ./rlm -run "TestToken|TestRLMStats_Token|TestFormatStats|TestEstimate" -v  # Token tracking & efficiency tests
go test ./rlm -run "TestTiktoken|TestHeuristic|TestCached|TestSetDefault|TestResetDefault" -v  # Tokenizer tests (BPE, heuristic, caching)
go test ./rlm -run "TestMemoryBackend|TestSQLiteBackend|TestStoreBackend" -v  # Store backend tests (memory + SQLite)
go test ./rlm -run "TestEpisodeManager|TestBuildEpisode" -v  # Episodic memory tests
go test ./rlm -run "TestContextSavings_" -v  # Context savings benchmarks (reproducible, no LLM calls)
go test ./rlm -run "TestLCM|TestDelegation|TestExpand|TestAgenticMap|TestIsTotalDelegation" -v  # LCM tests (store, DAG, delegation, expand restriction)
go test ./rlm/... -cover                # With coverage
go test ./rlm/... -bench=. -benchmem    # Benchmarks

# Go integration tests (require built binary)
./go/test_simple.sh
./go/integration_test.sh
```

## Linting

```bash
cd go && golangci-lint run ./...        # Go linting (CI uses this)
```

## Architecture

This is a TypeScript/Go hybrid package implementing Recursive Language Models (RLM). The TypeScript layer provides the public API; the Go binary does the heavy lifting.

### IPC Bridge Pattern

```
TypeScript (RLM class)
    ↓ spawn binary, JSON via stdin
Go Binary (bin/rlm-go)
    ↓ meta-agent optimizes query (optional)
    ↓ calls OpenAI-compatible API
    ↓ executes JS code in REPL (goja)
    ↓ recursive decomposition
    ↓ observability: OTEL traces, Langfuse events, debug logs
JSON response via stdout (+ trace_events)
    ↑
TypeScript (parses result, exposes trace events)
```

### Key Files

**TypeScript:**
- `src/rlm.ts` - Main RLM class, Zod->JSON Schema conversion, trace event access, file-based completions
- `src/bridge-interface.ts` - Config types (RLMConfig, MetaAgentConfig, ObservabilityConfig, ContextOverflowConfig, LCMConfig, LLMMapConfig, AgenticMapConfig, DelegationRequest, TraceEvent, FileStorageConfig)
- `src/errors.ts` - Error hierarchy including RLMContextOverflowError, classifyError()
- `src/file-storage.ts` - File storage providers (LocalFileStorage, S3FileStorage), FileContextBuilder, S3StorageError
- `src/pkg-dir.ts` - Portable package directory resolution (works in both CJS and ESM)
- `src/go-bridge.ts` - Spawns Go binary, handles stdin/stdout JSON IPC, platform package resolution
- `src/bridge-factory.ts` - Runtime Go binary detection (platform pkg → local → env), bridge creation
- `src/structured-types.ts` - TypeScript interfaces for structured output (SubTask, CoordinatorConfig, SchemaDecomposition)

**Go:**
- `go/cmd/rlm/main.go` - CLI entry, JSON I/O handler
- `go/rlm/rlm.go` - Core RLM engine, LLM orchestration, observer/meta-agent integration
- `go/rlm/structured.go` - Structured output with schema validation, parallel execution
- `go/rlm/schema.go` - Google jsonschema-go bridge, SchemaValidator, schema inference
- `go/rlm/meta_agent.go` - Meta-agent for query optimization
- `go/rlm/observability.go` - OTEL tracing, Langfuse integration, debug logging
- `go/rlm/repl.go` - JavaScript REPL executor using goja engine
- `go/rlm/types.go` - Type definitions (Config, MetaAgentConfig, ObservabilityConfig)
- `go/rlm/context_overflow.go` - Context overflow detection, 6 reduction strategies (mapreduce, truncate, chunked, tfidf, textrank, refine)
- `go/rlm/tfidf.go` - TF-IDF extractive compression: sentence splitting, tokenization, stop-word filtering, scoring
- `go/rlm/textrank.go` - TextRank graph-based ranking: cosine similarity, PageRank iteration
- `go/rlm/openai.go` - OpenAI-compatible API client, ChatCompletionResult with TokenUsage parsing
- `go/rlm/tokenizer.go` - BPE tokenizer via tiktoken-go, model-specific encoding selection, cached counting, heuristic fallback
- `go/rlm/tokenizer_test.go` - Tokenizer tests (BPE accuracy, encoding selection, CJK, caching, global default)
- `go/rlm/token_tracking_test.go` - 22 tests for token tracking and context reduction efficiency
- `go/rlm/lcm_store.go` - LCM Immutable Store and Hierarchical Summary DAG, grep/expand/describe tools
- `go/rlm/lcm_summarizer.go` - Five-Level Summarization Escalation (normal → aggressive → TF-IDF → TextRank → deterministic)
- `go/rlm/lcm_context_loop.go` - LCM Context Control Loop with soft/hard token thresholds
- `go/rlm/lcm_map.go` - LLM-Map operator: parallel batch processing with schema validation
- `go/rlm/lcm_agentic_map.go` - Agentic-Map operator: full sub-agent sessions per item
- `go/rlm/lcm_delegation.go` - Infinite delegation guard with scope-reduction invariant
- `go/rlm/lcm_files.go` - Large file handling with type-aware exploration summaries
- `go/rlm/lcm_episodes.go` - Episodic memory layer: episode lifecycle, auto-rotation, compaction, budget-based context retrieval
- `go/rlm/lcm_test.go` - Comprehensive LCM tests (store, DAG, summarizer, control loop, delegation, expand restriction)
- `go/rlm/lcm_episodes_test.go` - Episodic memory tests (episode rotation, compaction, parent chaining, context budget)
- `go/rlm/json_extraction.go` - Shared JSON extraction: balanced-brace parsing, markdown stripping (used by structured.go + lcm_map.go)
- `go/rlm/compression.go` - Shared text truncation: "keep start + end" strategy (used by context_overflow.go + lcm_summarizer.go)
- `go/rlm/store_backend.go` - StoreBackend interface and MemoryBackend implementation for LCM persistence abstraction
- `go/rlm/store_sqlite.go` - SQLiteBackend: pure-Go SQLite persistence with FTS5 full-text search, WAL mode, transactional writes
- `go/rlm/store_backend_test.go` - Store backend tests (MemoryBackend + SQLiteBackend with :memory:)
- `go/rlm/context_savings_test.go` - Reproducible context savings benchmarks (tokenizer accuracy, 5-level escalation, episodic memory, strategy comparison, combined pipeline)
### Binary Resolution

The Go bridge looks for the binary in this order:
1. `RLMConfig.go_binary_path` parameter
2. `RLM_GO_BINARY` environment variable
3. Platform-specific npm package (e.g., `@recursive-llm/darwin-arm64/bin/rlm-go`)
4. `<pkg-root>/bin/rlm-go` (npm package location)
5. `<pkg-root>/go/rlm-go` (development location)

Package root is resolved portably via `src/pkg-dir.ts` (handles both CJS `__dirname` and ESM `import.meta.url`).

### Structured Output Flow

When `structuredCompletion()` is called:
1. TypeScript converts Zod schema -> JSON Schema
2. Meta-agent optimizes query for extraction (if enabled)
3. Go validates schema with Google jsonschema-go
4. Go decomposes complex schemas into subtasks
5. Parallel execution via goroutines (if enabled)
6. Validation with retry logic (instructor-style feedback)
7. Results merged and returned as typed object
8. Trace events returned alongside results (if observability enabled)

### Meta-Agent Flow

When meta-agent is enabled:
1. Query is analyzed for specificity
2. If vague/short, meta-agent rewrites it
3. For structured queries, schema fields are referenced explicitly
4. Optimized query is passed to the RLM engine
5. Falls back to original query on error

### Context Overflow Flow

When context overflow is enabled (default):
1. Go engine attempts LLM call
2. If API returns context_length_exceeded error, `classifyError()` detects it
3. Error is parsed to extract `modelLimit` and `requestTokens`
4. `ContextReducer` applies configured strategy:
   - `mapreduce`: Split into chunks → parallel LLM summarization → merge
   - `truncate`: Drop tokens from end to fit budget
   - `chunked`: Sequential extraction from each chunk
   - `tfidf`: Pure Go extractive compression via TF-IDF sentence scoring
   - `textrank`: Graph-based PageRank over cosine-similarity of TF-IDF vectors
   - `refine`: Sequential iterative LLM refinement across chunks
5. Reduced context retried (up to `max_reduction_attempts`)

### File Storage Flow

When file-based completions are used:
1. FileContextBuilder creates appropriate provider (LocalFileStorage or S3FileStorage)
2. Provider lists all files recursively (traverses nested directories)
3. Filters applied: extensions, include/exclude globs, max file size, max total size, max files
4. Files read and formatted with clear delimiters for LLM consumption
5. Per-file errors caught gracefully (skipped with reason, other files still included)
6. Built context string passed to normal completion/structuredCompletion flow

S3 credential resolution order:
1. Explicit `config.credentials`
2. Environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`
3. AWS SDK default credential chain (IAM role, `~/.aws/credentials`, ECS task role, etc.)

S3-compatible services supported: AWS S3, MinIO, LocalStack, DigitalOcean Spaces, Backblaze B2

### Observability Flow

When observability is configured:
1. Observer is created with debug/OTEL/Langfuse config
2. Root trace span is created per completion call
3. Child spans track: LLM calls, REPL execution, meta-agent, validation
4. Events are collected and returned in response JSON
5. OTEL spans are exported to configured endpoint
6. Debug mode logs all operations to stderr/stdout/file

### LCM (Lossless Context Management) Architecture

LCM is an optional, opt-in context management layer inspired by the [LCM paper](https://papers.voltropy.com/LCM) (Voltropy, 2026). It replaces the legacy message pruning with a deterministic, DAG-based architecture that provides lossless retrievability of all prior context.

**Enable via config:**
```go
engine := rlm.New("gpt-4o", rlm.Config{
    LCM: &rlm.LCMConfig{Enabled: true},
})
```

Or in TypeScript:
```typescript
const rlm = RLM.builder('gpt-4o').build();
// Pass lcm config through RLMConfig
```

#### Dual-State Memory Architecture

```
Immutable Store (source of truth)
  │  Every message persisted verbatim, never modified
  │
  ├── Messages: user, assistant, tool, system
  │     └── Each has: ID, role, content, tokens, timestamp, file_ids
  │
  └── Summary DAG (derived, not source of truth)
        ├── Leaf Summaries: direct summary of message spans
        └── Condensed Summaries: higher-order summary of other summaries
              └── Provenance: parent/child pointers, file ID propagation

Active Context (sent to LLM)
  │  Assembled from recent messages + summary node pointers
  │  Summary nodes annotated with IDs for deterministic retrievability
  └── BuildMessages() → []Message for LLM calls
```

#### Context Control Loop (Soft/Hard Thresholds)

```
                    ┌─────────────────────────────────────┐
 tokens < τ_soft    │  Zero-cost continuity: no overhead  │
                    └─────────────────────────────────────┘
                    ┌─────────────────────────────────────┐
 τ_soft ≤ t < τ_hard│  Async compaction between turns     │
                    └─────────────────────────────────────┘
                    ┌─────────────────────────────────────┐
 tokens ≥ τ_hard    │  Blocking compaction before LLM call│
                    └─────────────────────────────────────┘
```

Defaults: τ_soft = 70% of model limit, τ_hard = 90% of model limit.

#### Five-Level Summarization Escalation

Guaranteed convergence — Level 5 is deterministic (no LLM):
1. **Level 1 (Normal):** LLM summarize with detail preservation, target T tokens
2. **Level 2 (Aggressive):** LLM summarize as bullet points, target T/2 tokens
3. **Level 3 (TF-IDF):** Extractive compression via sentence scoring, no LLM
4. **Level 4 (TextRank):** Graph-based PageRank sentence ranking, no LLM
5. **Level 5 (Deterministic):** `TruncateText()` — keep 2/3 start + 1/3 end, no LLM

#### Operator-Level Recursion

**LLM-Map** (`lcm_map.go`): Single tool call → engine manages parallel LLM calls
- JSONL input/output, schema validation per item, retry with feedback
- Worker pool (default 16 concurrent), database-backed status tracking
- Context isolation: inputs/outputs on disk, not in LLM context

**Agentic-Map** (`lcm_agentic_map.go`): Full sub-agent RLM sessions per item
- Each item gets its own RLM instance with tool access
- read_only flag, output validation, configurable depth/iterations
- Lower default concurrency (8) due to heavier per-item cost

#### Infinite Delegation Guard

When a sub-agent (depth > 0) spawns a further sub-agent, it must declare:
- `delegated_scope`: the specific slice of work being handed off
- `kept_work`: the work the caller retains for itself

If `kept_work` is trivial (e.g., "none", "waiting", "collect results"), the call is rejected. This forces each delegation level to represent a strict reduction in responsibility.

**Exemptions:** root agent (depth 0), read-only agents, parallel decomposition (sibling tasks).

#### LCM Tool Interfaces

- `Grep(pattern, maxResults)` — regex search across immutable store, results grouped by covering summary
- `ExpandSummary(id)` — reverse compaction, returns original messages (internal use)
- `ExpandSummaryRestricted(id, depth)` — same but enforces sub-agent-only policy (depth must be > 0)
- `Describe(id)` — metadata for any LCM identifier (message or summary)

#### Episodic Memory Layer

Episodes group related messages into coherent interaction units for compression and retrieval:
- EpisodeManager tracks episode lifecycle: active → compacted → archived
- Auto-rotation when episodes exceed token/message limits (configurable)
- Parent chaining creates episode chains for conversation continuity
- Budget-based retrieval: GetEpisodesForContext selects episodes fitting token budget
- Active episode always included in context; compacted episodes use summary tokens

#### Persistent Storage Backend

StoreBackend interface abstracts LCM persistence with two implementations:
- **MemoryBackend:** Default in-memory storage, zero external dependencies
- **SQLiteBackend:** Pure-Go SQLite (modernc.org/sqlite, no CGO) with:
  - WAL mode for concurrent reads during writes
  - FTS5 virtual tables for full-text message search
  - Transactional writes for crash recovery
  - Indexed lookups by ID, role, timestamp

#### Large File Handling

Files above token threshold (default 25k) are stored externally with exploration summaries:
- **JSON/JSONL:** Schema and shape extraction (keys, types, row counts)
- **CSV/TSV:** Column names, row counts, sample rows
- **Code:** Function/class/struct definitions, import analysis
- **Text:** LLM-generated summary (falls back to deterministic on failure)

File IDs propagate through the summary DAG during compaction.

#### Shared Utilities

#### Context Savings Benchmarks (Reproducible)

All benchmarks use deterministic content and the heuristic tokenizer for reproducibility.
Run with: `go test ./rlm/ -run "TestContextSavings_" -v`

**Tokenizer Accuracy** (BPE vs heuristic ~3.5 chars/token):

| Content Type | Chars | Heuristic Tokens | BPE Tokens | Difference |
|-------------|-------|-----------------|------------|------------|
| English Prose | 4,004 | 1,144 | 546 | +109.5% over-estimate |
| Go Code | 433 | 124 | 110 | +12.7% over-estimate |
| JSON | 411 | 118 | 143 | -17.5% under-estimate |
| CJK Text | 79 | 65 | 51 | +27.5% over-estimate |

**Five-Level Summarization (non-LLM levels, 5K→2K token target):**

| Level | Strategy | Output Tokens | Reduction | Preserves Sentences |
|-------|----------|--------------|-----------|-------------------|
| 3 | TF-IDF | 1,989 | 59.7% | ✅ Yes |
| 4 | TextRank | 1,972 | 60.0% | ✅ Yes |
| 5 | Truncate | 1,723 | 65.1% | ❌ No |

**Episodic Memory Budget Selection (50 messages → 10 episodes, compacted):**

| Token Budget | Episodes Selected | Context Tokens | Raw Tokens | Savings |
|-------------|------------------|---------------|------------|---------|
| 200 | 1 | 550 | 5,500 | 90.0% |
| 500 | 1 | 550 | 5,500 | 90.0% |
| 1,000 | 10 | 991 | 5,500 | 82.0% |
| 2,000 | 10 | 991 | 5,500 | 82.0% |

**All Strategies Comparison (35K→16K token target):**

| Strategy | Output Tokens | Reduction | Preserves Sentences |
|----------|--------------|-----------|-------------------|
| TF-IDF | 15,939 | 53.8% | ✅ Yes |
| TextRank | 15,850 | 54.0% | ✅ Yes |
| Truncate | 13,723 | 60.2% | ❌ No |

**Combined Pipeline (100 messages, episodic grouping + TF-IDF compaction + budget selection):**

| Stage | Tokens | Savings vs Raw |
|-------|--------|---------------|
| Raw messages | 50,491 | — |
| After episodic grouping | 50,491 | 0% |
| After TF-IDF compaction | 7,669 | 84.8% |
| After budget selection (8K) | 7,669 | 84.8% |

#### Accurate Token Management
Token counting uses model-specific BPE tokenization via tiktoken-go:
- SetDefaultTokenizer(model) — called during RLM.New(), selects encoding (o200k_base for GPT-4o, cl100k_base for GPT-4/Codex)
- CachedTokenizer wraps BPE with xxhash-keyed sync.Map cache (10K entries)
- HeuristicTokenizer provides ~3.5 chars/token fallback when encoding unavailable
- EstimateTokens() delegates to the global tokenizer, used throughout the codebase

RLM and LCM share common infrastructure to avoid duplication:
- `json_extraction.go` — `ExtractFirstJSON()`, `ExtractAllBalancedJSON()`, `ExtractBalancedBraces()`, `StripMarkdownCodeBlock()`
- `compression.go` — `TruncateText()` with configurable start/end fractions and marker text
- `tokenizer.go` — Tokenizer interface, TiktokenTokenizer (BPE), HeuristicTokenizer (fallback), CachedTokenizer (xxhash + sync.Map)

## Go Module

The Go code is a public package importable by other Go projects:

```go
import "github.com/howlerops/recursive-llm-ts/go/rlm"

// Standard RLM completion
engine := rlm.New("gpt-4o-mini", rlm.Config{
    APIKey: "...",
    MetaAgent: &rlm.MetaAgentConfig{Enabled: true},
    Observability: &rlm.ObservabilityConfig{Debug: true},
})
result, stats, err := engine.Completion(query, context)

// With LCM enabled (deterministic context management)
lcmEngine := rlm.New("gpt-4o", rlm.Config{
    APIKey: "...",
    LCM: &rlm.LCMConfig{
        Enabled:        true,
        SoftThreshold:  90000,   // 70% of 128k (default)
        HardThreshold:  115000,  // 90% of 128k (default)
    },
})
result, stats, err = lcmEngine.Completion(query, context)

// LLM-Map: parallel batch processing
mapResult, err := engine.LLMMap(rlm.LLMMapConfig{
    InputPath:  "data/items.jsonl",
    OutputPath: "data/results.jsonl",
    Prompt:     "Classify this item: {{item}}",
    OutputSchema: &rlm.JSONSchema{Type: "object", Properties: map[string]*rlm.JSONSchema{
        "category": {Type: "string"},
    }},
    Concurrency: 16,
})

// Agentic-Map: full sub-agent sessions per item
agenticResult, err := engine.AgenticMap(rlm.AgenticMapConfig{
    InputPath:  "data/repos.jsonl",
    OutputPath: "data/analyses.jsonl",
    Prompt:     "Analyze this repository: {{item}}",
    Concurrency: 4,
    ReadOnly:    true,
})

// Task delegation with recursion guard
result, stats, err = engine.DelegateTask(rlm.DelegationRequest{
    Prompt:         "Parse the config files",
    DelegatedScope: "Parse config/ directory files",
    KeptWork:       "Validate and apply extracted settings",
})
```

## Environment

Required: `OPENAI_API_KEY` (or pass via config)

Optional: `AZURE_API_KEY`, `RLM_GO_BINARY`, `RLM_DEBUG`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `LANGFUSE_PUBLIC_KEY`, `LANGFUSE_SECRET_KEY`, `LANGFUSE_HOST`

S3 File Storage: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, `AWS_REGION`, `AWS_DEFAULT_REGION`

## CI/CD

- `.github/workflows/ci.yml` - Go tests + linting, then Node.js matrix (Ubuntu/macOS/Windows x Node 18/20)
- `.github/workflows/go-release.yml` - Cross-platform binary builds on tag push
- `.github/workflows/publish.yml` - NPM publish on release
