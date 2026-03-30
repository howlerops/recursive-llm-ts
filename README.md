# recursive-llm-ts

TypeScript/JavaScript package for [Recursive Language Models (RLM)](https://github.com/alexzhang13/rlm) - process unbounded context lengths with LLMs.

**Based on the paper**: [Recursive Language Models](https://alexzhang13.github.io/blog/2025/rlm/) by Alex Zhang and Omar Khattab (MIT, 2025)

## Features

**Core**
- **Unbounded Context** - Process 10M+ tokens without degradation via recursive decomposition
- **Structured Outputs** - Extract typed data with Zod schemas, parallel execution, and instructor-style retry
- **Streaming** - Progressive text output and partial structured objects via async iterables
- **Batch Operations** - Process multiple queries in parallel with concurrency control

**Performance & Resilience**
- **Pure Go Backend** - 50x faster startup, 3x less memory vs Python
- **Context Overflow Recovery** - Automatic detection and 6 reduction strategies (mapreduce, truncate, chunked, tfidf, textrank, refine)
- **Lossless Context Management** - Episodic memory, 5-level summarization escalation, SQLite persistence with FTS5 search
- **BPE Token Counting** - Accurate model-specific tokenization via tiktoken (o200k, cl100k) with cached counting
- **Caching** - Exact-match caching with in-memory and file-based backends
- **Retry & Fallback** - Exponential backoff, jitter, and multi-provider fallback chains
- **AbortController** - Cancel any operation mid-flight

**Developer Experience**
- **Typed Errors** - Rich error hierarchy with codes, retryable flags, and suggestions
- **Event System** - Monitor LLM calls, cache hits, retries, and errors in real-time
- **Builder API** - Fluent configuration with full IDE discoverability
- **Factory Methods** - `RLM.fromEnv()`, `RLM.withDebug()`, `RLM.forAzure()`
- **Config Validation** - Catches typos and invalid settings at construction time
- **Result Formatters** - `prettyStats()`, `toJSON()`, `toMarkdown()`

**Ecosystem**
- **Provider Agnostic** - Works with OpenAI, Anthropic, Azure, Bedrock, local models
- **Meta-Agent Mode** - Automatically optimize queries for better results
- **Observability** - OpenTelemetry tracing, Langfuse integration, and debug logging
- **File Storage** - Process local directories or S3/MinIO/LocalStack buckets as LLM context
- **200+ Tests** - Comprehensive Vitest + Go test suites

## Installation

```bash
npm install recursive-llm-ts
```

### Prerequisites

- **Node.js 16+**
- **Go 1.25+** (only if you need to build the Go binary from source)

### Go Binary Resolution

On supported platforms, npm installs a matching pre-built binary package automatically:

- `@recursive-llm/darwin-arm64`
- `@recursive-llm/darwin-x64`
- `@recursive-llm/linux-x64`
- `@recursive-llm/linux-arm64`
- `@recursive-llm/win32-x64`

This is the default path for most Linux containers, so Go is usually not required in container images.

The bridge resolves the binary in this order:

1. `config.go_binary_path`
2. `RLM_GO_BINARY`
3. Matching `@recursive-llm/<platform>` package
4. Local `bin/rlm-go` or `go/rlm-go`

### Go Binary (Automatic Fallback)

When no matching pre-built binary is available, the `postinstall` script attempts to build the Go binary during installation. If Go is not available, the script will warn but not fail.

If you need to build manually:

```bash
# From the package directory
cd node_modules/recursive-llm-ts
node scripts/build-go-binary.js

# Or directly with Go
cd go && go build -o ../bin/rlm-go ./cmd/rlm
```

Override the binary path if needed:

```bash
export RLM_GO_BINARY=/custom/path/to/rlm-go
```

## Usage

### Quick Start
```typescript
import { RLM } from 'recursive-llm-ts';

// Initialize RLM with a model
const rlm = new RLM('gpt-4o-mini', {
  max_iterations: 15,
  api_key: process.env.OPENAI_API_KEY
});

// Process a query with unbounded context
const result = await rlm.completion(
  'What are the key points in this document?',
  longDocument
);

console.log(result.result);
console.log('Stats:', result.stats);
```

### Structured Outputs with Zod Schemas

Extract structured, typed data from any context using Zod schemas. Supports complex nested objects, arrays, enums, and automatic parallel execution for performance.

```typescript
import { RLM } from 'recursive-llm-ts';
import { z } from 'zod';

const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY
});

// Define your schema
const sentimentSchema = z.object({
  sentimentValue: z.number().min(1).max(5),
  sentimentExplanation: z.string(),
  keyPhrases: z.array(z.object({
    phrase: z.string(),
    sentiment: z.number()
  })),
  topics: z.array(z.enum(['pricing', 'features', 'support', 'competition']))
});

// Extract structured data
const result = await rlm.structuredCompletion(
  'Analyze the sentiment and extract key information',
  callTranscript,
  sentimentSchema
);

// result.result is fully typed!
console.log(result.result.sentimentValue); // number
console.log(result.result.keyPhrases); // Array<{phrase: string, sentiment: number}>
```

**Key Benefits:**
- ✅ **Type-safe** - Full TypeScript types from your Zod schema
- ✅ **Automatic validation** - Retries with error feedback if schema doesn't match
- ✅ **Parallel execution** - Complex schemas processed in parallel with goroutines (3-5x faster)
- ✅ **Deep nesting** - Supports arbitrarily nested objects and arrays
- ✅ **Enum support** - Validates enum values automatically

**Performance Options:**
```typescript
// Enable/disable parallel execution
const result = await rlm.structuredCompletion(
  query,
  context,
  schema,
  { 
    parallelExecution: true,  // default: true for complex schemas
    maxRetries: 3              // default: 3
  }
);
```

### Streaming

Get progressive output with async iterables and AbortController support:

```typescript
// Stream text output
const stream = rlm.streamCompletion('Summarize', longDocument);
for await (const chunk of stream) {
  if (chunk.type === 'text') process.stdout.write(chunk.text);
}

// Collect all text
const text = await rlm.streamCompletion('Summarize', doc).toText();

// Cancel mid-stream
const controller = new AbortController();
const stream = rlm.streamCompletion('Summarize', doc, { signal: controller.signal });
setTimeout(() => controller.abort(), 5000);
```

### Caching

Avoid redundant API calls with exact-match caching:

```typescript
const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY,
  cache: {
    enabled: true,
    strategy: 'exact',   // 'exact' | 'none'
    maxEntries: 1000,
    ttl: 3600,            // seconds
    storage: 'memory',    // 'memory' | 'file'
  }
});

const r1 = await rlm.completion('Summarize', doc); // API call
const r2 = await rlm.completion('Summarize', doc); // Cache hit!
console.log(r2.cached); // true
console.log(rlm.getCacheStats()); // { hits: 1, misses: 1, hitRate: 0.5, ... }
```

### Retry & Resilience

Automatic retry with exponential backoff and provider fallback:

```typescript
const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY,
  retry: {
    maxRetries: 3,
    backoff: 'exponential', // 1s, 2s, 4s with jitter
    onRetry: (attempt, error, delay) => {
      console.log(`Retry ${attempt} after ${delay}ms: ${error.message}`);
    },
  },
});

// Or use standalone retry/fallback utilities:
import { withRetry, withFallback } from 'recursive-llm-ts';

const result = await withFallback(
  (model) => rlm.completion(query, context),
  { models: ['gpt-4o', 'claude-sonnet-4-20250514', 'gemini-2.0-flash'] }
);
```

### Event System

Monitor operations in real-time:

```typescript
const rlm = new RLM('gpt-4o-mini', { api_key: process.env.OPENAI_API_KEY });

rlm.on('llm_call', (e) => console.log(`Calling ${e.model}...`));
rlm.on('llm_response', (e) => console.log(`Response in ${e.duration}ms`));
rlm.on('cache', (e) => console.log(`Cache ${e.action}`));
rlm.on('error', (e) => reportToSentry(e.error));
rlm.on('completion_start', (e) => showSpinner());
rlm.on('completion_end', (e) => hideSpinner());
```

### Builder API

Fluent configuration with full IDE discoverability:

```typescript
const rlm = RLM.builder('gpt-4o-mini')
  .apiKey(process.env.OPENAI_API_KEY!)
  .maxDepth(10)
  .maxIterations(30)
  .withMetaAgent({ model: 'gpt-4o' })
  .withDebug()
  .withCache({ strategy: 'exact' })
  .withRetry({ maxRetries: 3 })
  .withFallback(['gpt-4o', 'claude-sonnet-4-20250514'])
  .build();
```

### Factory Methods

Quick setup for common configurations:

```typescript
// From environment variables
const rlm = RLM.fromEnv('gpt-4o-mini');

// Debug mode
const rlm = RLM.withDebug('gpt-4o-mini');

// Azure OpenAI
const rlm = RLM.forAzure('my-deployment', {
  apiBase: 'https://myresource.openai.azure.com',
  apiVersion: '2024-02-15-preview',
});
```

### Batch Operations

Process multiple queries in parallel:

```typescript
const results = await rlm.batchCompletion([
  { query: 'Summarize chapter 1', context: ch1 },
  { query: 'Summarize chapter 2', context: ch2 },
  { query: 'Summarize chapter 3', context: ch3 },
], { concurrency: 2 });
```

### Error Handling

Rich error hierarchy with actionable information:

```typescript
import {
  RLMRateLimitError, RLMValidationError,
  RLMTimeoutError, RLMContextOverflowError
} from 'recursive-llm-ts';

try {
  const result = await rlm.completion(query, context);
} catch (err) {
  if (err instanceof RLMContextOverflowError) {
    console.log(`Context overflow: ${err.requestTokens} tokens > ${err.modelLimit} limit`);
    // Enable context_overflow config to auto-recover from this
  } else if (err instanceof RLMRateLimitError) {
    console.log(`Rate limited. Retry after: ${err.retryAfter}s`);
  } else if (err instanceof RLMValidationError) {
    console.log(`Schema mismatch:`, err.zodErrors);
  } else if (err instanceof RLMTimeoutError) {
    console.log(`Timed out after ${err.elapsed}ms`);
  }
  // All RLM errors have: err.code, err.retryable, err.suggestion
}
```

### Context Overflow Handling

Automatically detect and recover from context window overflows. When your input exceeds the model's token limit, RLM catches the error and applies a reduction strategy to fit the context within bounds.

```typescript
const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY,
  context_overflow: {
    enabled: true,           // Enable overflow recovery (default: true)
    strategy: 'tfidf',       // Reduction strategy (see table below)
    max_model_tokens: 32768, // Override auto-detected limit (optional)
    safety_margin: 0.15,     // Reserve 15% for prompts/overhead (default: 0.15)
    max_reduction_attempts: 3, // Max retry attempts (default: 3)
  }
});

// Process a document that may exceed the model's context window
const result = await rlm.completion(
  'Summarize the key findings',
  veryLargeDocument  // If too large, auto-reduces and retries
);
```

**Builder API:**
```typescript
const rlm = RLM.builder('gpt-4o-mini')
  .apiKey(process.env.OPENAI_API_KEY!)
  .withContextOverflow({ strategy: 'textrank', max_model_tokens: 32768 })
  .build();
```

**Strategy Comparison:**

| Strategy | API Calls | Speed | Quality | Best For |
|----------|-----------|-------|---------|----------|
| `mapreduce` | Many (parallel) | Medium | High | General-purpose, large documents |
| `truncate` | 0 | Fastest | Low | Quick-and-dirty, when beginning of doc matters |
| `chunked` | Many (sequential) | Slow | High | Detailed extraction from specific sections |
| `tfidf` | 0 | Fast | Medium | Fast first pass, keyword-rich documents |
| `textrank` | 0 | Fast | Medium-High | Documents with clear sentence structure |
| `refine` | Many (sequential) | Slow | Highest | When quality matters most, iterative refinement |

**Strategy Details:**

- **`mapreduce`** (default) - Splits context into chunks, summarizes each in parallel via LLM calls, then merges summaries. Good balance of quality and speed.
- **`truncate`** - Drops tokens from the end to fit the budget. Zero API calls, but loses information. Best when the beginning of the document is most important.
- **`chunked`** - Processes chunks sequentially, extracting relevant content from each. Higher quality than mapreduce for targeted extraction.
- **`tfidf`** - Pure Go, zero API calls. Uses TF-IDF scoring to select the most informative sentences. Preserves original document order. Great for a fast, no-cost first pass.
- **`textrank`** - Pure Go, zero API calls. Graph-based sentence ranking using PageRank over cosine-similarity of TF-IDF vectors. Better at identifying structurally important sentences than plain TF-IDF.
- **`refine`** - Sequential iterative refinement. Processes chunks one at a time, building and refining an answer progressively. Highest quality but slowest, as each chunk sees the accumulated context.

### Config Validation

Catch configuration issues at construction time:

```typescript
const rlm = new RLM('gpt-4o-mini', { max_detph: 5 }); // typo!
const result = rlm.validate();
// result.issues: [{ level: 'warning', field: 'max_detph', message: 'Unknown config key...' }]
```

### Result Formatting

```typescript
const result = await rlm.completion(query, context);
const formatted = rlm.formatResult(result);

console.log(formatted.prettyStats());
// "LLM Calls: 3 | Iterations: 12 | Depth: 2"

console.log(formatted.toMarkdown());
// Full markdown-formatted result with stats table
```

### Agent Coordinator (Advanced)

For complex multi-field schemas, use the coordinator API:

```typescript
import { RLMAgentCoordinator } from 'recursive-llm-ts';

const coordinator = new RLMAgentCoordinator(
  'gpt-4o-mini',
  { api_key: process.env.OPENAI_API_KEY },
  'auto',
  { parallelExecution: true }
);

const result = await coordinator.processComplex(
  'Extract comprehensive call analysis',
  transcript,
  complexSchema
);
```

### Bridge Selection

The package automatically uses the Go binary by default (if available). You can explicitly specify a bridge if needed:

```typescript
import { RLM } from 'recursive-llm-ts';

// Default: Auto-detection (prefers Go if available)
const rlm = new RLM('gpt-4o-mini', {
  max_iterations: 15,
  api_key: process.env.OPENAI_API_KEY
});

// Explicit: Force Go binary
const rlmGo = new RLM('gpt-4o-mini', {
  max_iterations: 15,
  api_key: process.env.OPENAI_API_KEY
}, 'go');

```


### Meta-Agent Mode

The meta-agent automatically optimizes queries before they are processed by the RLM engine. This is useful when queries are vague, non-specific, or not optimized for recursive processing.

```typescript
import { RLM } from 'recursive-llm-ts';

const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY,
  meta_agent: {
    enabled: true,
    model: 'gpt-4o',           // Optional: model for optimization (defaults to main model)
    max_optimize_len: 10000    // Optional: skip optimization for short contexts
  }
});

// The meta-agent will automatically optimize this vague query
const result = await rlm.completion(
  'what happened?',
  longCallTranscript
);

// Also works with structured completions
const structured = await rlm.structuredCompletion(
  'analyze this',
  callTranscript,
  sentimentSchema
);
```

The meta-agent:
- Rewrites vague queries to be specific and actionable
- Adds format specifications and extraction hints
- Optimizes for recursive decomposition patterns
- Falls back to the original query if optimization fails

### Observability and Debugging

Built-in support for OpenTelemetry tracing, Langfuse integration, and debug logging.

#### Debug Mode

Enable verbose logging of all internal operations:

```typescript
const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY,
  debug: true  // Shorthand for observability.debug
});

// Or with full config:
const rlm2 = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY,
  observability: {
    debug: true,
    log_output: 'stderr'  // "stderr" (default), "stdout", or a file path
  }
});
```

#### OpenTelemetry Tracing

```typescript
const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY,
  observability: {
    trace_enabled: true,
    trace_endpoint: 'localhost:4317',  // OTLP endpoint
    service_name: 'my-rlm-service'
  }
});

const result = await rlm.completion('Summarize', document);

// Access trace events programmatically
const events = rlm.getTraceEvents();
console.log('Trace events:', events);
```

#### Langfuse Integration

```typescript
const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY,
  observability: {
    langfuse_enabled: true,
    langfuse_public_key: process.env.LANGFUSE_PUBLIC_KEY,
    langfuse_secret_key: process.env.LANGFUSE_SECRET_KEY,
    langfuse_host: 'https://cloud.langfuse.com'  // Optional, defaults to cloud
  }
});
```

#### Environment Variable Configuration

Observability can also be configured via environment variables:

```bash
RLM_DEBUG=1                           # Enable debug mode
OTEL_EXPORTER_OTLP_ENDPOINT=...      # Auto-enable OTEL tracing
LANGFUSE_PUBLIC_KEY=pk-...            # Auto-enable Langfuse
LANGFUSE_SECRET_KEY=sk-...
LANGFUSE_HOST=https://cloud.langfuse.com
```

### File Storage Context

Process files from local directories or S3-compatible storage as LLM context. Supports recursive directory traversal, filtering, and automatic context formatting.

#### Local Files

```typescript
import { RLM } from 'recursive-llm-ts';

const rlm = new RLM('gpt-4o-mini', {
  api_key: process.env.OPENAI_API_KEY
});

// Process all TypeScript files in a directory
const result = await rlm.completionFromFiles(
  'Summarize the architecture of this codebase',
  {
    type: 'local',
    path: '/path/to/project/src',
    extensions: ['.ts', '.tsx'],
    excludePatterns: ['*.test.ts', '*.spec.ts'],
    maxFileSize: 100_000,  // Skip files over 100KB
  }
);

console.log(result.result);
console.log(`Files processed: ${result.fileStorage?.files.length}`);
```

#### S3 / Object Storage

Works with AWS S3, MinIO, LocalStack, DigitalOcean Spaces, and Backblaze B2.

```typescript
// AWS S3 with explicit credentials
const result = await rlm.completionFromFiles(
  'What are the key findings in these reports?',
  {
    type: 's3',
    path: 'my-bucket',          // Bucket name
    prefix: 'reports/2024/',    // Folder prefix
    extensions: ['.md', '.txt'],
    credentials: {
      accessKeyId: 'AKIA...',
      secretAccessKey: '...',
    },
    region: 'us-west-2',
  }
);

// S3 with environment variable credentials
// Uses AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN
const result2 = await rlm.completionFromFiles(
  'Summarize these documents',
  { type: 's3', path: 'my-bucket', prefix: 'docs/' }
);

// MinIO / LocalStack
const result3 = await rlm.completionFromFiles(
  'Analyze the data',
  {
    type: 's3',
    path: 'local-bucket',
    endpoint: 'http://localhost:9000',  // MinIO
    credentials: { accessKeyId: 'minioadmin', secretAccessKey: 'minioadmin' },
  }
);
```

**Credential Resolution Order:**
1. Explicit `credentials` in config
2. Environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`
3. AWS SDK default credential chain (IAM roles, `~/.aws/credentials`, ECS task role, etc.)

#### Structured Extraction from Files

```typescript
import { z } from 'zod';

const schema = z.object({
  summary: z.string(),
  mainTopics: z.array(z.string()),
  sentiment: z.enum(['positive', 'negative', 'neutral']),
});

const result = await rlm.structuredCompletionFromFiles(
  'Extract a summary, main topics, and overall sentiment',
  { type: 'local', path: './docs', extensions: ['.md'] },
  schema
);

console.log(result.result.mainTopics); // string[]
```

#### Preview Files Before Processing

```typescript
import { FileContextBuilder } from 'recursive-llm-ts';

const builder = new FileContextBuilder({
  type: 'local',
  path: './src',
  extensions: ['.ts'],
  excludePatterns: ['node_modules/**'],
});

// List matching files without reading content
const files = await builder.listMatchingFiles();
console.log('Would process:', files);

// Build full context
const ctx = await builder.buildContext();
console.log(`Total size: ${ctx.totalSize} bytes`);
console.log(`Files included: ${ctx.files.length}`);
console.log(`Files skipped: ${ctx.skipped.length}`);
```

## API

### `RLM`

Main class for recursive language model completions.

**Constructor:**
```typescript
constructor(model: string, rlmConfig?: RLMConfig)
```

- `model`: The LLM model to use (e.g., 'gpt-4o-mini', 'claude-3-sonnet')
- `rlmConfig`: Optional configuration object

**Methods:**

#### `completion(query: string, context: string): Promise<RLMResult>`

Process a query with the given context using recursive language models.

**Parameters:**
- `query`: The question or task to perform
- `context`: The context/document to process (can be arbitrarily large)

**Returns:**
- `Promise<RLMResult>`: Result containing the answer and statistics

#### `structuredCompletion<T>(query: string, context: string, schema: ZodSchema<T>, options?): Promise<StructuredRLMResult<T>>`

Extract structured, typed data from context using a Zod schema.

**Parameters:**
- `query`: The extraction task to perform
- `context`: The context/document to process
- `schema`: Zod schema defining the output structure
- `options`: Optional configuration
  - `parallelExecution?: boolean` - Enable parallel processing (default: true)
  - `maxRetries?: number` - Max validation retries (default: 3)

**Returns:**
- `Promise<StructuredRLMResult<T>>`: Typed result matching your schema

**Example:**
```typescript
const schema = z.object({ score: z.number(), summary: z.string() });
const result = await rlm.structuredCompletion('Analyze', doc, schema);
// result.result is typed as { score: number, summary: string }
```

#### `completionFromFiles(query: string, fileConfig: FileStorageConfig): Promise<RLMResult & { fileStorage?: FileStorageResult }>`

Process a query using files from local or S3 storage as context.

**Parameters:**
- `query`: The question or task to perform
- `fileConfig`: File storage configuration (local path or S3 bucket)

**Returns:**
- Result with `fileStorage` metadata (files included, skipped, total size)

#### `structuredCompletionFromFiles<T>(query: string, fileConfig: FileStorageConfig, schema: ZodSchema<T>, options?): Promise<StructuredRLMResult<T> & { fileStorage?: FileStorageResult }>`

Extract structured data from file-based context.

#### `streamCompletion(query, context, options?): RLMStream`

Stream a completion with progressive text output. Returns an async iterable.

#### `streamStructuredCompletion<T>(query, context, schema, options?): RLMStream<T>`

Stream a structured completion with partial object updates.

#### `batchCompletion(queries, options?): Promise<Array<RLMCompletionResult | Error>>`

Execute multiple completions in parallel with concurrency control.

#### `batchStructuredCompletion<T>(queries, options?): Promise<Array<StructuredRLMResult<T> | Error>>`

Execute multiple structured completions in parallel.

#### `validate(): ValidationResult`

Validate the current configuration without making API calls.

#### `getCacheStats(): CacheStats`

Get cache performance statistics (hits, misses, hit rate).

#### `clearCache(): void`

Clear the completion cache.

#### `formatResult(result): RLMResultFormatter`

Create a formatted result with `prettyStats()`, `toJSON()`, `toMarkdown()`.

#### `on(event, listener) / off(event, listener) / once(event, listener)`

Register/remove event listeners. Events: `llm_call`, `llm_response`, `error`, `cache`, `completion_start`, `completion_end`, `retry`, `validation_retry`, `meta_agent`, `recursion`.

#### `cleanup(): Promise<void>`

Clean up the bridge and free resources.

```typescript
await rlm.cleanup();
```

#### `getTraceEvents(): TraceEvent[]`

Returns trace events from the last operation (when observability is enabled).

```typescript
const events = rlm.getTraceEvents();
for (const event of events) {
  console.log(`${event.type}: ${event.name}`, event.attributes);
}
```

### Types

```typescript
interface RLMConfig {
  // Model configuration
  recursive_model?: string;      // Model to use for recursive calls (defaults to main model)

  // API configuration
  api_base?: string;             // Custom API base URL (for Azure, Bedrock, etc.)
  api_key?: string;              // API key for authentication

  // Execution limits
  max_depth?: number;            // Maximum recursion depth (default: 5)
  max_iterations?: number;       // Maximum REPL iterations per call (default: 30)
  go_binary_path?: string;       // Override path for Go binary (optional)

  // Meta-agent configuration
  meta_agent?: MetaAgentConfig;

  // Observability configuration
  observability?: ObservabilityConfig;

  // Shorthand for observability.debug
  debug?: boolean;

  // LiteLLM parameters
  api_version?: string;          // API version (e.g., for Azure)
  timeout?: number;              // Request timeout in seconds
  temperature?: number;          // Sampling temperature
  max_tokens?: number;           // Maximum tokens in response

  // Context overflow recovery
  context_overflow?: ContextOverflowConfig;

  // Caching, retry, fallback
  cache?: CacheConfig;           // Cache configuration
  retry?: RetryConfig;           // Retry configuration
  fallback?: FallbackConfig;     // Fallback model configuration
}

interface CacheConfig {
  enabled?: boolean;             // Enable caching (default: false)
  strategy?: 'exact' | 'none';  // Cache strategy (default: 'exact')
  maxEntries?: number;           // Max cached entries (default: 1000)
  ttl?: number;                  // Time-to-live in seconds (default: 3600)
  storage?: 'memory' | 'file';  // Storage backend (default: 'memory')
  cacheDir?: string;             // Dir for file cache (default: '.rlm-cache')
}

interface RetryConfig {
  maxRetries?: number;           // Max retries (default: 3)
  backoff?: 'exponential' | 'linear' | 'fixed';
  baseDelay?: number;            // Base delay ms (default: 1000)
  maxDelay?: number;             // Max delay ms (default: 30000)
  jitter?: boolean;              // Add jitter (default: true)
  onRetry?: (attempt: number, error: Error, delay: number) => void;
}

interface FallbackConfig {
  models?: string[];             // Ordered fallback models
  strategy?: 'sequential';      // Fallback strategy
}

interface ContextOverflowConfig {
  enabled?: boolean;             // Enable overflow recovery (default: true)
  max_model_tokens?: number;     // Override auto-detected model limit (0 = auto-detect)
  strategy?: 'mapreduce' | 'truncate' | 'chunked' | 'tfidf' | 'textrank' | 'refine';
  safety_margin?: number;        // Fraction to reserve for overhead (default: 0.15)
  max_reduction_attempts?: number; // Max reduction retries (default: 3)
}

interface MetaAgentConfig {
  enabled: boolean;              // Enable the meta-agent
  model?: string;                // Model for query optimization (defaults to main model)
  max_optimize_len?: number;     // Max context length before optimization (0 = always)
}

interface ObservabilityConfig {
  debug?: boolean;               // Enable verbose debug logging
  trace_enabled?: boolean;       // Enable OpenTelemetry tracing
  trace_endpoint?: string;       // OTLP endpoint (e.g., "localhost:4317")
  service_name?: string;         // Service name for traces (default: "rlm")
  log_output?: string;           // Log destination: "stderr", "stdout", or file path
  langfuse_enabled?: boolean;    // Enable Langfuse integration
  langfuse_public_key?: string;  // Langfuse public key
  langfuse_secret_key?: string;  // Langfuse secret key
  langfuse_host?: string;        // Langfuse API host
}

interface RLMResult {
  result: string;
  stats: RLMStats;
  trace_events?: TraceEvent[];   // Observability events (when enabled)
}

interface RLMStats {
  llm_calls: number;
  iterations: number;
  depth: number;
  parsing_retries?: number;
}

interface TraceEvent {
  timestamp: string;
  type: string;                  // "span_start", "span_end", "llm_call", "error", "event"
  name: string;
  attributes: Record<string, string>;
  duration?: number;
  trace_id?: string;
  span_id?: string;
}

interface FileStorageConfig {
  type: 'local' | 's3';         // Storage provider type
  path: string;                  // Local directory path or S3 bucket name
  prefix?: string;               // S3 key prefix (folder path)
  region?: string;               // AWS region (default: AWS_REGION env or 'us-east-1')
  credentials?: {                // S3 explicit credentials (optional)
    accessKeyId: string;
    secretAccessKey: string;
    sessionToken?: string;
  };
  endpoint?: string;             // Custom S3 endpoint (MinIO, LocalStack, etc.)
  forcePathStyle?: boolean;      // Force path-style S3 URLs (auto-enabled with endpoint)
  extensions?: string[];         // File extensions to include (e.g., ['.ts', '.md'])
  includePatterns?: string[];    // Glob patterns to include
  excludePatterns?: string[];    // Glob patterns to exclude
  maxFileSize?: number;          // Max individual file size in bytes (default: 1MB)
  maxTotalSize?: number;         // Max total context size in bytes (default: 10MB)
  maxFiles?: number;             // Max number of files (default: 1000)
}

interface FileStorageResult {
  context: string;               // Built context string with file contents
  files: Array<{ relativePath: string; size: number }>;
  totalSize: number;
  skipped: Array<{ relativePath: string; reason: string }>;
}

// Error hierarchy - all extend RLMError
class RLMError extends Error {
  code: string;                  // Machine-readable: "RATE_LIMIT", "VALIDATION", etc.
  retryable: boolean;            // Whether caller should retry
  suggestion?: string;           // Human-readable fix suggestion
}
class RLMValidationError extends RLMError { expected; received; zodErrors; }
class RLMRateLimitError extends RLMError { retryAfter?: number; }
class RLMTimeoutError extends RLMError { elapsed; limit; }
class RLMProviderError extends RLMError { statusCode; provider; }
class RLMBinaryError extends RLMError { binaryPath; }
class RLMConfigError extends RLMError { field; value; }
class RLMContextOverflowError extends RLMError { modelLimit; requestTokens; }
class RLMSchemaError extends RLMError { path; constraint; }
class RLMAbortError extends RLMError {}
```

## Environment Variables

Set your API key as an environment variable:

```bash
export OPENAI_API_KEY='your-api-key-here'
```

Or pass it in the configuration:

```typescript
const rlm = new RLM('gpt-4o-mini', {
  api_key: 'your-api-key-here'
});
```

### S3 File Storage

When using S3 file storage without explicit credentials, these environment variables are checked:

```bash
export AWS_ACCESS_KEY_ID='your-access-key'
export AWS_SECRET_ACCESS_KEY='your-secret-key'
export AWS_SESSION_TOKEN='optional-session-token'
export AWS_REGION='us-east-1'
```

## Custom Providers

The Go binary uses an **OpenAI-compatible chat completion API** and works seamlessly with
[LiteLLM proxy](https://docs.litellm.ai/docs/simple_proxy) or any provider that supports the
OpenAI `/chat/completions` schema. This keeps the implementation provider-agnostic.

### Quick Reference

The Go binary speaks the **OpenAI chat completion schema**, so you can:

- Use OpenAI directly with `api_key`
- Use an OpenAI-compatible endpoint (Azure OpenAI, vLLM, Ollama)
- Use a LiteLLM proxy to reach providers like Anthropic, Bedrock, or Cohere

### Amazon Bedrock (via LiteLLM proxy)

```typescript
import { RLM } from 'recursive-llm-ts';

const rlm = new RLM('bedrock/anthropic.claude-3-sonnet-20240229-v1:0', {
  api_base: 'http://localhost:4000', // LiteLLM proxy URL
  api_key: process.env.LITELLM_API_KEY,
  max_iterations: 15
});
```

### Azure OpenAI

```typescript
const rlm = new RLM('gpt-4o', {
  api_base: 'https://your-resource.openai.azure.com/openai/deployments/your-deployment',
  api_key: process.env.AZURE_API_KEY,
  api_version: '2024-02-15-preview' // Passed through to the OpenAI-compatible API
});
```

### Custom OpenAI-Compatible APIs

For providers with OpenAI-compatible APIs (e.g., local models, vLLM, Ollama):

```typescript
const rlm = new RLM('openai/your-model', {
  api_base: 'https://your-custom-endpoint.com/v1',
  api_key: 'your-key-here'
});
```

### Long-Running Processes

For large documents or queue-based processing that may take longer than the default 100s timeout:

```typescript
const rlm = new RLM('gpt-4o-mini', {
  max_iterations: 50,           // Allow more iterations for complex processing
  timeout: 300                  // 5 minutes timeout for LLM API calls
});

// Process very large document
const result = await rlm.completion(
  'Summarize all key points from this document',
  veryLargeDocument
);
```

### Other Providers

See the [LiteLLM documentation](https://docs.litellm.ai/docs/providers) for the complete list of supported providers and their configuration.

## Docker Deployment

### Option 1: Use the Published Linux Binary Package

On `linux-x64` and `linux-arm64`, `npm ci --omit=dev` installs the matching `@recursive-llm/linux-*` package automatically. This is the simplest container setup and does not require Go or `RLM_GO_BINARY`.

```dockerfile
FROM node:22-alpine
WORKDIR /app

COPY package*.json ./
RUN npm ci --omit=dev

COPY . .

ENV OPENAI_API_KEY=""
ENV NODE_ENV=production

CMD ["node", "your-app.js"]
```

### Option 2: Copy or Mount a Custom Binary

Use this when you build the Go binary elsewhere and want to point the package at an explicit path.

```dockerfile
FROM node:22-alpine
WORKDIR /app

COPY package*.json ./
RUN npm ci --omit=dev --ignore-scripts

COPY . .
COPY ./bin/rlm-go /app/bin/rlm-go
RUN chmod +x /app/bin/rlm-go

ENV OPENAI_API_KEY=""
ENV NODE_ENV=production
ENV RLM_GO_BINARY=/app/bin/rlm-go

CMD ["node", "your-app.js"]
```

### Option 3: Build from Source in a Multi-Stage Image

Use this for unsupported targets or when you need a custom-compiled binary.

```dockerfile
# Stage 1: Build the Go binary
FROM golang:1.25-alpine AS go-builder
WORKDIR /build
COPY go/go.mod go/go.sum ./
RUN go mod download
COPY go/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o rlm-go ./cmd/rlm

# Stage 2: Build Node.js dependencies
FROM node:22-alpine AS node-builder
WORKDIR /app
COPY package*.json ./
RUN npm ci --omit=dev --ignore-scripts

# Stage 3: Final runtime image
FROM node:22-alpine
WORKDIR /app

COPY --from=node-builder /app/node_modules ./node_modules
COPY --from=go-builder /build/rlm-go ./bin/rlm-go
RUN chmod +x ./bin/rlm-go

COPY . .

ENV NODE_ENV=production
ENV RLM_GO_BINARY=/app/bin/rlm-go
ENV OPENAI_API_KEY=""

CMD ["node", "your-app.js"]
```

**Benefits:** Small runtime image, deterministic binary path, and no Go toolchain in the final image.

### Docker Compose

```yaml
version: '3.8'
services:
  app:
    build: .
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - NODE_ENV=production
    ports:
      - "3000:3000"
```

### Installing Go in Different Base Images

```dockerfile
# Alpine
RUN apk add --no-cache go

# Debian/Ubuntu
RUN apt-get update && apt-get install -y golang-1.25

# Or use the published platform package (no Go required)
# npm ci --omit=dev installs @recursive-llm/linux-x64 or @recursive-llm/linux-arm64 automatically

# Or use a release binary explicitly
# Download from GitHub Releases and set RLM_GO_BINARY=/app/bin/rlm-go
```

## Using the Go Module Directly

The Go implementation can be used as a standalone library in Go projects.

### Installation

```bash
go get github.com/howlerops/recursive-llm-ts/go
```

### Usage

```go
package main

import (
    "fmt"
    "os"

    "github.com/howlerops/recursive-llm-ts/go/rlm"
)

func main() {
    config := rlm.Config{
        MaxDepth:      5,
        MaxIterations: 30,
        APIKey:        os.Getenv("OPENAI_API_KEY"),
    }

    engine := rlm.New("gpt-4o-mini", config)

    answer, stats, err := engine.Completion(
        "What are the key points?",
        "Your long document here...",
    )
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    fmt.Printf("Answer: %s\n", answer)
    fmt.Printf("Stats: %d LLM calls, %d iterations\n",
        stats.LlmCalls, stats.Iterations)
}
```

### Structured Output

```go
schema := &rlm.JSONSchema{
    Type: "object",
    Properties: map[string]*rlm.JSONSchema{
        "summary": {Type: "string"},
        "score":   {Type: "number"},
    },
    Required: []string{"summary", "score"},
}

config := &rlm.StructuredConfig{
    Schema:     schema,
    MaxRetries: 3,
}

result, stats, err := engine.StructuredCompletion(
    "Summarize and score",
    document,
    config,
)
```

### Building from Source

```bash
cd go

# Standard build
go build -o rlm-go ./cmd/rlm

# Optimized (smaller binary)
go build -ldflags="-s -w" -o rlm-go ./cmd/rlm

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o rlm-linux-amd64 ./cmd/rlm
GOOS=darwin GOARCH=arm64 go build -o rlm-darwin-arm64 ./cmd/rlm
```

### Running Tests

```bash
cd go
go test -v ./rlm/...
```

## How It Works

This package provides a TypeScript wrapper around a Go implementation of Recursive-LLM, enabling seamless integration into Node.js/TypeScript applications without Python dependencies. The Go binary is built locally (or supplied via `RLM_GO_BINARY`) and invoked for completions.

The recursive-llm approach breaks down large contexts into manageable chunks and processes them recursively, allowing you to work with documents of any size without hitting token limits.

### Key Features

- ✅ **No Python dependency** - Go binary handles the full recursive loop
- ✅ **Provider-agnostic** - Works with OpenAI-compatible APIs or LiteLLM proxy
- ✅ **Type-safe** - Full TypeScript type definitions
- ✅ **Simple API** - Just `npm install` and start using

## Testing

```bash
# Run all tests (Vitest)
npm test

# Watch mode
npm run test:watch

# Coverage
npm run test:coverage

# Type-check
npm run typecheck

# Go tests
cd go && go test ./rlm/... -v
```

## Documentation

- [Quick Start Guide](docs/QUICKSTART.md)
- [Architecture Overview](docs/ARCHITECTURE.md)
- [Contributing Guide](CONTRIBUTING.md)
- [UX/DX Gap Analysis](docs/UX-DX-GAP-ANALYSIS.md)

## Publishing

This package uses automated GitHub Actions workflows to publish to npm. See [RELEASE.md](RELEASE.md) for detailed instructions on publishing new versions.

**Quick start:**
```bash
npm version patch  # Bump version
git push origin main --tags  # Push tag
# Then create a GitHub release to trigger automatic npm publish
```

## License

MIT
