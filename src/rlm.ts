/**
 * Main RLM (Recursive Language Model) class.
 *
 * Provides the primary API for recursive completions, structured output,
 * streaming, file-based context, caching, retry/resilience, and events.
 *
 * @example
 * ```typescript
 * import { RLM } from 'recursive-llm-ts';
 *
 * const rlm = new RLM('gpt-4o-mini', { api_key: process.env.OPENAI_API_KEY });
 * const result = await rlm.completion('Summarize this', longDocument);
 * console.log(result.result);
 * ```
 */

import { RLMConfig, RLMResult, RLMStats, TraceEvent, FileStorageConfig, ContextOverflowConfig, Bridge } from './bridge-interface';
import { createBridge, BridgeType } from './bridge-factory';
import { z } from 'zod';
import { StructuredRLMResult } from './structured-types';
import { FileContextBuilder, FileStorageResult } from './file-storage';
import { RLMEventEmitter, RLMEventType, RLMEventMap } from './events';
import { RLMCache, CacheConfig } from './cache';
import { withRetry, withFallback, RetryConfig, FallbackConfig } from './retry';
import { RLMStream, StreamOptions, createSimulatedStream } from './streaming';
import { RLMExtendedConfig, validateConfig, assertValidConfig, ValidationResult } from './config';
import { RLMError, RLMValidationError, RLMBinaryError, classifyError } from './errors';

// ─── Enhanced Result Types ───────────────────────────────────────────────────

/** Extended result with cache information */
export interface RLMCompletionResult extends RLMResult {
  /** Whether this result was served from cache */
  cached: boolean;
  /** Model that was actually used (relevant with fallback) */
  model: string;
}

/** Pretty-printable result wrapper */
export class RLMResultFormatter {
  constructor(
    public readonly result: string,
    public readonly stats: RLMStats,
    public readonly cached: boolean,
    public readonly model: string,
    public readonly trace_events?: TraceEvent[]
  ) {}

  /** Format stats as a concise one-liner */
  prettyStats(): string {
    const parts = [
      `LLM Calls: ${this.stats.llm_calls}`,
      `Iterations: ${this.stats.iterations}`,
      `Depth: ${this.stats.depth}`,
    ];
    if (this.stats.parsing_retries) {
      parts.push(`Retries: ${this.stats.parsing_retries}`);
    }
    if (this.stats.total_tokens) {
      parts.push(`Tokens: ${this.stats.total_tokens} (prompt: ${this.stats.prompt_tokens ?? 0}, completion: ${this.stats.completion_tokens ?? 0})`);
    }
    if (this.cached) {
      parts.push('(cached)');
    }
    return parts.join(' | ');
  }

  /** Serialize to a JSON-safe object */
  toJSON(): Record<string, unknown> {
    return {
      result: this.result,
      stats: this.stats,
      cached: this.cached,
      model: this.model,
      trace_events: this.trace_events,
    };
  }

  /** Format as Markdown */
  toMarkdown(): string {
    const lines = [
      '## Result',
      '',
      this.result,
      '',
      '## Stats',
      '',
      `| Metric | Value |`,
      `|--------|-------|`,
      `| LLM Calls | ${this.stats.llm_calls} |`,
      `| Iterations | ${this.stats.iterations} |`,
      `| Depth | ${this.stats.depth} |`,
    ];
    if (this.stats.parsing_retries) {
      lines.push(`| Parsing Retries | ${this.stats.parsing_retries} |`);
    }
    if (this.stats.total_tokens) {
      lines.push(`| Total Tokens | ${this.stats.total_tokens} |`);
      lines.push(`| Prompt Tokens | ${this.stats.prompt_tokens ?? 0} |`);
      lines.push(`| Completion Tokens | ${this.stats.completion_tokens ?? 0} |`);
    }
    lines.push(`| Cached | ${this.cached} |`);
    lines.push(`| Model | ${this.model} |`);
    return lines.join('\n');
  }
}

// ─── Builder ─────────────────────────────────────────────────────────────────

/**
 * Fluent builder for configuring RLM instances.
 *
 * @example
 * ```typescript
 * const rlm = RLM.builder('gpt-4o-mini')
 *   .maxDepth(10)
 *   .withMetaAgent()
 *   .withDebug()
 *   .withCache({ strategy: 'exact' })
 *   .withRetry({ maxRetries: 3 })
 *   .build();
 * ```
 */
export class RLMBuilder {
  private model: string;
  private config: RLMExtendedConfig = {};
  private bridgeType: BridgeType = 'go';

  constructor(model: string) {
    this.model = model;
  }

  /** Set the API key */
  apiKey(key: string): this {
    this.config.api_key = key;
    return this;
  }

  /** Set the API base URL */
  apiBase(url: string): this {
    this.config.api_base = url;
    return this;
  }

  /** Set maximum recursion depth */
  maxDepth(depth: number): this {
    this.config.max_depth = depth;
    return this;
  }

  /** Set maximum iterations */
  maxIterations(iterations: number): this {
    this.config.max_iterations = iterations;
    return this;
  }

  /** Enable meta-agent query optimization */
  withMetaAgent(config?: { model?: string; max_optimize_len?: number }): this {
    this.config.meta_agent = { enabled: true, ...config };
    return this;
  }

  /** Enable debug mode */
  withDebug(logOutput?: string): this {
    this.config.debug = true;
    if (logOutput) {
      this.config.observability = { ...this.config.observability, debug: true, log_output: logOutput };
    }
    return this;
  }

  /** Configure observability */
  withObservability(config: RLMConfig['observability']): this {
    this.config.observability = config;
    return this;
  }

  /** Configure caching */
  withCache(config?: CacheConfig): this {
    this.config.cache = { enabled: true, ...config };
    return this;
  }

  /** Configure retry behavior */
  withRetry(config?: RetryConfig): this {
    this.config.retry = config;
    return this;
  }

  /** Configure fallback models */
  withFallback(models: string[]): this {
    this.config.fallback = { models, strategy: 'sequential' };
    return this;
  }

  /** Set the bridge type */
  bridge(type: BridgeType): this {
    this.bridgeType = type;
    return this;
  }

  /** Configure context overflow recovery */
  withContextOverflow(config?: ContextOverflowConfig): this {
    this.config.context_overflow = { enabled: true, ...config };
    return this;
  }

  /** Set the Go binary path */
  binaryPath(path: string): this {
    this.config.go_binary_path = path;
    return this;
  }

  /** Add LiteLLM passthrough parameters */
  litellmParams(params: Record<string, unknown>): this {
    this.config.litellm_params = params;
    return this;
  }

  /** Build the RLM instance */
  build(): RLM {
    return new RLM(this.model, this.config, this.bridgeType);
  }
}

// ─── Main RLM Class ──────────────────────────────────────────────────────────

export class RLM {
  private bridge: Bridge | null = null;
  private model: string;
  private rlmConfig: RLMExtendedConfig;
  private bridgeType: BridgeType;
  private lastTraceEvents: TraceEvent[] = [];
  private events: RLMEventEmitter;
  private cache: RLMCache;

  /**
   * Create a new RLM instance.
   *
   * @param model - The LLM model identifier (e.g., 'gpt-4o-mini', 'claude-sonnet-4-20250514')
   * @param rlmConfig - Configuration options for the RLM engine
   * @param bridgeType - Bridge selection: 'go' (default)
   *
   * @example
   * ```typescript
   * const rlm = new RLM('gpt-4o-mini', {
   *   api_key: process.env.OPENAI_API_KEY,
   *   max_depth: 5,
   *   cache: { enabled: true },
   *   retry: { maxRetries: 3 },
   * });
   * ```
   */
  constructor(model: string, rlmConfig: RLMExtendedConfig = {}, bridgeType: BridgeType = 'go') {
    this.model = model;
    this.rlmConfig = this.normalizeConfig(rlmConfig);
    this.bridgeType = bridgeType;
    this.events = new RLMEventEmitter();
    this.cache = new RLMCache(rlmConfig.cache);
  }

  // ─── Static Factory Methods ──────────────────────────────────────────────

  /**
   * Create an RLM instance using environment variables for configuration.
   *
   * @param model - The LLM model identifier
   * @returns RLM instance configured from environment
   *
   * @example
   * ```typescript
   * // Uses OPENAI_API_KEY from environment
   * const rlm = RLM.fromEnv('gpt-4o-mini');
   * ```
   */
  static fromEnv(model: string): RLM {
    return new RLM(model, {
      api_key: process.env.OPENAI_API_KEY,
      api_base: process.env.OPENAI_API_BASE,
      debug: process.env.RLM_DEBUG === '1' || process.env.RLM_DEBUG === 'true',
    });
  }

  /**
   * Create an RLM instance with debug logging enabled.
   *
   * @param model - The LLM model identifier
   * @param config - Additional configuration options
   * @returns RLM instance with debug mode active
   */
  static withDebug(model: string, config: RLMExtendedConfig = {}): RLM {
    return new RLM(model, { ...config, debug: true });
  }

  /**
   * Create an RLM instance configured for Azure OpenAI.
   *
   * @param deploymentName - Azure deployment name
   * @param config - Azure-specific configuration
   * @returns RLM instance configured for Azure
   */
  static forAzure(deploymentName: string, config: { apiBase: string; apiKey?: string; apiVersion?: string }): RLM {
    return new RLM(deploymentName, {
      api_base: config.apiBase,
      api_key: config.apiKey || process.env.AZURE_API_KEY,
      litellm_params: { api_version: config.apiVersion || '2024-02-15-preview' },
    });
  }

  /**
   * Create a fluent builder for advanced configuration.
   *
   * @param model - The LLM model identifier
   * @returns Builder instance
   *
   * @example
   * ```typescript
   * const rlm = RLM.builder('gpt-4o-mini')
   *   .apiKey(process.env.OPENAI_API_KEY!)
   *   .maxDepth(10)
   *   .withMetaAgent()
   *   .withCache({ strategy: 'exact' })
   *   .build();
   * ```
   */
  static builder(model: string): RLMBuilder {
    return new RLMBuilder(model);
  }

  // ─── Config Normalization ────────────────────────────────────────────────

  private normalizeConfig(config: RLMExtendedConfig): RLMExtendedConfig {
    // Normalize debug shorthand into observability config
    if (config.debug && !config.observability) {
      config.observability = { debug: true };
    } else if (config.debug && config.observability) {
      config.observability.debug = true;
    }
    return config;
  }

  // ─── Bridge Management ───────────────────────────────────────────────────

  private async ensureBridge(): Promise<Bridge> {
    if (!this.bridge) {
      this.bridge = await createBridge(this.bridgeType, this.rlmConfig);
    }
    return this.bridge;
  }

  // ─── Event System ────────────────────────────────────────────────────────

  /**
   * Register an event listener.
   *
   * @param event - Event type to listen for
   * @param listener - Callback function
   *
   * @example
   * ```typescript
   * rlm.on('llm_call', (e) => console.log(`Calling ${e.model}`));
   * rlm.on('error', (e) => reportError(e.error));
   * rlm.on('cache', (e) => console.log(`Cache ${e.action}`));
   * ```
   */
  on<K extends RLMEventType>(event: K, listener: (event: RLMEventMap[K]) => void): this {
    this.events.on(event, listener);
    return this;
  }

  /**
   * Register a one-time event listener.
   *
   * @param event - Event type to listen for
   * @param listener - Callback function (called once then removed)
   */
  once<K extends RLMEventType>(event: K, listener: (event: RLMEventMap[K]) => void): this {
    this.events.once(event, listener);
    return this;
  }

  /**
   * Remove an event listener.
   *
   * @param event - Event type
   * @param listener - The listener function to remove
   */
  off<K extends RLMEventType>(event: K, listener: (event: RLMEventMap[K]) => void): this {
    this.events.off(event, listener);
    return this;
  }

  /** Remove all event listeners */
  removeAllListeners(event?: RLMEventType): this {
    this.events.removeAllListeners(event);
    return this;
  }

  // ─── Core Completions ────────────────────────────────────────────────────

  /**
   * Execute a completion against an LLM with recursive decomposition.
   *
   * @param query - The question or instruction for the LLM
   * @param context - The document or data to process (can be very large)
   * @param options - Optional completion settings
   * @returns The LLM response with execution statistics
   *
   * @example
   * ```typescript
   * const result = await rlm.completion('Summarize the key points', longDocument);
   * console.log(result.result);
   * console.log(`Used ${result.stats.llm_calls} LLM calls`);
   * ```
   */
  public async completion(
    query: string,
    context: string,
    options: { signal?: AbortSignal } = {}
  ): Promise<RLMCompletionResult> {
    const startTime = Date.now();

    this.events.emit('completion_start', {
      timestamp: startTime,
      type: 'completion_start',
      model: this.model,
      query,
      contextLength: context.length,
      structured: false,
    });

    // Check cache
    const cached = this.cache.lookup<RLMResult>(this.model, query, context);
    if (cached.hit && cached.value) {
      this.events.emit('cache', { timestamp: Date.now(), type: 'cache', action: 'hit' });
      this.events.emit('completion_end', {
        timestamp: Date.now(),
        type: 'completion_end',
        model: this.model,
        duration: Date.now() - startTime,
        stats: cached.value.stats,
        cached: true,
      });
      return { ...cached.value, cached: true, model: this.model };
    }
    this.events.emit('cache', { timestamp: Date.now(), type: 'cache', action: 'miss' });

    // Execute with retry
    const execute = async () => {
      const bridge = await this.ensureBridge();

      this.events.emit('llm_call', {
        timestamp: Date.now(),
        type: 'llm_call',
        model: this.model,
        queryLength: query.length,
        contextLength: context.length,
      });

      const result = await bridge.completion(this.model, query, context, this.rlmConfig);

      this.events.emit('llm_response', {
        timestamp: Date.now(),
        type: 'llm_response',
        model: this.model,
        duration: Date.now() - startTime,
      });

      return result;
    };

    try {
      const result = await withRetry(execute, this.rlmConfig.retry, options.signal);

      if (result.trace_events) {
        this.lastTraceEvents = result.trace_events;
      }

      // Store in cache
      this.cache.store(this.model, query, context, result);
      this.events.emit('cache', { timestamp: Date.now(), type: 'cache', action: 'store' });

      this.events.emit('completion_end', {
        timestamp: Date.now(),
        type: 'completion_end',
        model: this.model,
        duration: Date.now() - startTime,
        stats: result.stats,
        cached: false,
      });

      return { ...result, cached: false, model: this.model };
    } catch (err) {
      const error = err instanceof Error ? err : new Error(String(err));
      this.events.emit('error', {
        timestamp: Date.now(),
        type: 'error',
        error,
        operation: 'completion',
      });
      throw err instanceof RLMError ? err : classifyError(error);
    }
  }

  /**
   * Extract structured, typed data from context using a Zod schema.
   *
   * @param query - The extraction task to perform
   * @param context - The document or data to process
   * @param schema - Zod schema defining the expected output structure
   * @param options - Execution options (parallelExecution, maxRetries, signal)
   * @returns Typed result matching your Zod schema
   *
   * @example
   * ```typescript
   * const schema = z.object({
   *   summary: z.string(),
   *   score: z.number().min(1).max(10),
   *   tags: z.array(z.string()),
   * });
   *
   * const result = await rlm.structuredCompletion('Analyze this document', doc, schema);
   * console.log(result.result.summary);  // string
   * console.log(result.result.score);    // number
   * console.log(result.result.tags);     // string[]
   * ```
   */
  public async structuredCompletion<T>(
    query: string,
    context: string,
    schema: z.ZodSchema<T>,
    options: { maxRetries?: number; parallelExecution?: boolean; signal?: AbortSignal } = {}
  ): Promise<StructuredRLMResult<T>> {
    const startTime = Date.now();

    this.events.emit('completion_start', {
      timestamp: startTime,
      type: 'completion_start',
      model: this.model,
      query,
      contextLength: context.length,
      structured: true,
    });

    const jsonSchema = this.zodToJsonSchema(schema);

    const execute = async () => {
      const bridge = await this.ensureBridge();

      const structuredConfig = {
        schema: jsonSchema,
        parallelExecution: options.parallelExecution ?? true,
        maxRetries: options.maxRetries ?? 3,
      };

      this.events.emit('llm_call', {
        timestamp: Date.now(),
        type: 'llm_call',
        model: this.model,
        queryLength: query.length,
        contextLength: context.length,
      });

      const result = await bridge.completion(
        this.model,
        query,
        context,
        { ...this.rlmConfig, structured: structuredConfig }
      );

      return result;
    };

    try {
      const result = await withRetry(execute, this.rlmConfig.retry, options.signal);

      if (result.trace_events) {
        this.lastTraceEvents = result.trace_events;
      }

      // Validate result against Zod schema for type safety
      let validated: T;
      try {
        validated = schema.parse(result.result);
      } catch (zodErr: any) {
        throw new RLMValidationError({
          message: `Structured output failed Zod validation: ${zodErr.message}`,
          expected: jsonSchema,
          received: result.result,
          zodErrors: zodErr.errors || zodErr.issues,
        });
      }

      this.events.emit('completion_end', {
        timestamp: Date.now(),
        type: 'completion_end',
        model: this.model,
        duration: Date.now() - startTime,
        stats: result.stats,
        cached: false,
      });

      return {
        result: validated,
        stats: result.stats,
        trace_events: result.trace_events,
      };
    } catch (err) {
      const error = err instanceof Error ? err : new Error(String(err));
      this.events.emit('error', {
        timestamp: Date.now(),
        type: 'error',
        error,
        operation: 'structuredCompletion',
      });
      throw err;
    }
  }

  // ─── Streaming ───────────────────────────────────────────────────────────

  /**
   * Stream a completion with progressive text output.
   *
   * Returns an async iterable of stream chunks. Supports AbortController
   * for cancellation.
   *
   * Note: Currently simulates streaming by chunking the full response.
   * Full streaming support (from the Go binary) is planned.
   *
   * @param query - The question or instruction for the LLM
   * @param context - The document or data to process
   * @param options - Stream options including AbortController signal
   * @returns Async iterable stream of chunks
   *
   * @example
   * ```typescript
   * const stream = rlm.streamCompletion(query, context);
   * for await (const chunk of stream) {
   *   if (chunk.type === 'text') process.stdout.write(chunk.text);
   * }
   *
   * // Or collect as string
   * const text = await rlm.streamCompletion(query, context).toText();
   *
   * // With abort
   * const controller = new AbortController();
   * const stream = rlm.streamCompletion(query, context, { signal: controller.signal });
   * setTimeout(() => controller.abort(), 5000);
   * ```
   */
  public streamCompletion(
    query: string,
    context: string,
    options: StreamOptions = {}
  ): RLMStream {
    const stream = new RLMStream(options.signal);

    (async () => {
      try {
        const result = await this.completion(query, context, { signal: options.signal });
        const text = typeof result.result === 'string' ? result.result : JSON.stringify(result.result);

        // Simulate streaming by chunking
        const chunkSize = 20;
        for (let i = 0; i < text.length; i += chunkSize) {
          if (options.signal?.aborted) return;
          const chunk = text.slice(i, i + chunkSize);
          stream.push({ type: 'text', text: chunk, timestamp: Date.now() });
          if (options.onChunk) {
            options.onChunk({ type: 'text', text: chunk, timestamp: Date.now() });
          }
          // Yield to event loop
          await new Promise(resolve => setImmediate(resolve));
        }

        stream.complete(result.stats);
      } catch (err) {
        stream.pushError(err instanceof Error ? err : new Error(String(err)));
      }
    })();

    return stream;
  }

  /**
   * Stream a structured completion with partial object updates.
   *
   * @param query - The extraction task to perform
   * @param context - The document or data to process
   * @param schema - Zod schema for the output structure
   * @param options - Stream and execution options
   * @returns Async iterable stream with partial object chunks
   */
  public streamStructuredCompletion<T>(
    query: string,
    context: string,
    schema: z.ZodSchema<T>,
    options: StreamOptions & { maxRetries?: number; parallelExecution?: boolean } = {}
  ): RLMStream<T> {
    const stream = new RLMStream<T>(options.signal);

    (async () => {
      try {
        const result = await this.structuredCompletion(query, context, schema, options);
        stream.push({
          type: 'partial_object',
          object: result.result as Partial<T>,
          timestamp: Date.now(),
        });
        stream.complete(result.stats);
      } catch (err) {
        stream.pushError(err instanceof Error ? err : new Error(String(err)));
      }
    })();

    return stream;
  }

  // ─── Batch Operations ────────────────────────────────────────────────────

  /**
   * Execute multiple completions in parallel with concurrency control.
   *
   * @param queries - Array of query+context pairs to process
   * @param options - Batch options including concurrency limit
   * @returns Array of results in the same order as input
   *
   * @example
   * ```typescript
   * const results = await rlm.batchCompletion([
   *   { query: 'Summarize chapter 1', context: ch1 },
   *   { query: 'Summarize chapter 2', context: ch2 },
   *   { query: 'Summarize chapter 3', context: ch3 },
   * ], { concurrency: 2 });
   * ```
   */
  public async batchCompletion(
    queries: Array<{ query: string; context: string }>,
    options: { concurrency?: number; signal?: AbortSignal } = {}
  ): Promise<Array<RLMCompletionResult | Error>> {
    const concurrency = options.concurrency ?? 3;
    const results: Array<RLMCompletionResult | Error> = new Array(queries.length);
    let index = 0;

    const worker = async () => {
      while (index < queries.length) {
        if (options.signal?.aborted) return;
        const i = index++;
        try {
          results[i] = await this.completion(queries[i].query, queries[i].context, { signal: options.signal });
        } catch (err) {
          results[i] = err instanceof Error ? err : new Error(String(err));
        }
      }
    };

    const workers = Array.from({ length: Math.min(concurrency, queries.length) }, () => worker());
    await Promise.all(workers);
    return results;
  }

  /**
   * Execute multiple structured completions in parallel.
   *
   * @param queries - Array of query+context+schema triples
   * @param options - Batch options including concurrency limit
   * @returns Array of typed results
   */
  public async batchStructuredCompletion<T>(
    queries: Array<{ query: string; context: string; schema: z.ZodSchema<T> }>,
    options: { concurrency?: number; signal?: AbortSignal } = {}
  ): Promise<Array<StructuredRLMResult<T> | Error>> {
    const concurrency = options.concurrency ?? 3;
    const results: Array<StructuredRLMResult<T> | Error> = new Array(queries.length);
    let index = 0;

    const worker = async () => {
      while (index < queries.length) {
        if (options.signal?.aborted) return;
        const i = index++;
        try {
          results[i] = await this.structuredCompletion(
            queries[i].query,
            queries[i].context,
            queries[i].schema,
            { signal: options.signal }
          );
        } catch (err) {
          results[i] = err instanceof Error ? err : new Error(String(err));
        }
      }
    };

    const workers = Array.from({ length: Math.min(concurrency, queries.length) }, () => worker());
    await Promise.all(workers);
    return results;
  }

  // ─── File-Based Completions ──────────────────────────────────────────────

  /**
   * Run a completion using files from a folder (local or S3) as context.
   *
   * @param query - The question or task to perform
   * @param fileConfig - File storage configuration (local path or S3 bucket)
   * @returns Result with fileStorage metadata (files included, skipped, total size)
   *
   * @example
   * ```typescript
   * const result = await rlm.completionFromFiles(
   *   'Summarize the architecture',
   *   { type: 'local', path: './src', extensions: ['.ts'] }
   * );
   * console.log(result.result);
   * console.log(`Processed ${result.fileStorage.files.length} files`);
   * ```
   */
  public async completionFromFiles(
    query: string,
    fileConfig: FileStorageConfig
  ): Promise<RLMCompletionResult & { fileStorage: FileStorageResult }> {
    const builder = new FileContextBuilder(fileConfig);
    const storageResult = await builder.buildContext();
    const result = await this.completion(query, storageResult.context);
    return { ...result, fileStorage: storageResult };
  }

  /**
   * Run a structured completion using files from a folder (local or S3) as context.
   *
   * @param query - The extraction task to perform
   * @param fileConfig - File storage configuration
   * @param schema - Zod schema for the output structure
   * @param options - Execution options
   * @returns Typed result with fileStorage metadata
   */
  public async structuredCompletionFromFiles<T>(
    query: string,
    fileConfig: FileStorageConfig,
    schema: z.ZodSchema<T>,
    options: { maxRetries?: number; parallelExecution?: boolean } = {}
  ): Promise<StructuredRLMResult<T> & { fileStorage: FileStorageResult }> {
    const builder = new FileContextBuilder(fileConfig);
    const storageResult = await builder.buildContext();
    const result: StructuredRLMResult<T> = await this.structuredCompletion(query, storageResult.context, schema, options);
    return {
      result: result.result,
      stats: result.stats,
      trace_events: result.trace_events,
      fileStorage: storageResult,
    };
  }

  /**
   * Preview which files would be included from a file storage config
   * without actually reading them. Useful for dry-runs.
   *
   * @param fileConfig - File storage configuration
   * @returns Array of relative file paths that match the config
   */
  public async previewFiles(fileConfig: FileStorageConfig): Promise<string[]> {
    const builder = new FileContextBuilder(fileConfig);
    return builder.listMatchingFiles();
  }

  /**
   * Build context from a file storage config without running a completion.
   * Useful for inspecting the generated context string.
   *
   * @param fileConfig - File storage configuration
   * @returns Built context with metadata
   */
  public async buildFileContext(fileConfig: FileStorageConfig): Promise<FileStorageResult> {
    const builder = new FileContextBuilder(fileConfig);
    return builder.buildContext();
  }

  // ─── Observability ───────────────────────────────────────────────────────

  /**
   * Returns trace events from the last operation.
   * Only populated when observability is enabled in the config.
   *
   * @returns Array of trace events from the most recent completion
   */
  public getTraceEvents(): TraceEvent[] {
    return this.lastTraceEvents;
  }

  /**
   * Get cache statistics (hits, misses, hit rate).
   *
   * @returns Cache performance statistics
   */
  public getCacheStats() {
    return this.cache.getStats();
  }

  /** Clear the completion cache */
  public clearCache(): void {
    this.cache.clear();
  }

  // ─── Validation ──────────────────────────────────────────────────────────

  /**
   * Validate the current configuration without making any API calls.
   * Checks binary existence, config validity, and connectivity hints.
   *
   * @returns Validation result with issues
   *
   * @example
   * ```typescript
   * const issues = rlm.validate();
   * if (!issues.valid) {
   *   console.error('Config issues:', issues.issues);
   * }
   * ```
   */
  public validate(): ValidationResult {
    return validateConfig(this.rlmConfig);
  }

  // ─── Result Formatting ───────────────────────────────────────────────────

  /**
   * Create a formatted result wrapper from a completion result.
   *
   * @param result - The completion result to format
   * @returns Formatter with prettyStats(), toJSON(), and toMarkdown() methods
   */
  public formatResult(result: RLMCompletionResult): RLMResultFormatter {
    return new RLMResultFormatter(
      typeof result.result === 'string' ? result.result : JSON.stringify(result.result),
      result.stats,
      result.cached,
      result.model,
      result.trace_events
    );
  }

  // ─── Cleanup ─────────────────────────────────────────────────────────────

  /**
   * Clean up the bridge connection and free resources.
   * Call this when you're done using the RLM instance.
   */
  public async cleanup(): Promise<void> {
    if (this.bridge) {
      await this.bridge.cleanup();
      this.bridge = null;
    }
    this.events.removeAllListeners();
  }

  /**
   * Support for `Symbol.asyncDispose` (Node 22+ `await using`).
   */
  async [Symbol.asyncDispose](): Promise<void> {
    await this.cleanup();
  }

  // ─── Zod to JSON Schema Conversion ───────────────────────────────────────

  private zodToJsonSchema(schema: z.ZodSchema<any>): any {
    const def = (schema as any)._def;
    const defType = def.type;

    // Handle wrapped types (optional, nullable, default, catch)
    if (defType === 'optional' || defType === 'nullable' || defType === 'default' || defType === 'catch') {
      const inner = this.zodToJsonSchema(def.innerType);
      if (defType === 'nullable') {
        return { ...inner, nullable: true };
      }
      return inner;
    }

    // Handle effects (refine, transform, preprocess) - unwrap to inner type
    if (defType === 'effects') {
      return this.zodToJsonSchema(def.schema);
    }

    // Handle pipeline (pipe) - use the output schema
    if (defType === 'pipeline') {
      return this.zodToJsonSchema(def.out);
    }

    // Handle lazy schemas - unwrap the getter
    if (defType === 'lazy') {
      try {
        const actualSchema = def.getter();
        return this.zodToJsonSchema(actualSchema);
      } catch (e) {
        return { type: 'object' };
      }
    }

    // Handle branded types - unwrap to base type
    if (defType === 'branded') {
      return this.zodToJsonSchema(def.type);
    }

    // Handle readonly - pass through
    if (defType === 'readonly') {
      return this.zodToJsonSchema(def.innerType);
    }

    // Handle literals
    if (defType === 'literal') {
      if (def.values && def.values.length > 0) {
        const value = def.values[0];
        const valueType = typeof value;
        return {
          type: valueType === 'object' ? 'string' : valueType,
          enum: [value]
        };
      }
      const value = def.value;
      if (value !== undefined) {
        const valueType = typeof value;
        return {
          type: valueType === 'object' ? 'string' : valueType,
          enum: [value]
        };
      }
    }

    // Handle unions
    if (defType === 'union' || defType === 'discriminatedUnion') {
      const options = def.options || Array.from(def.optionsMap?.values() || []);
      if (options.length > 0) {
        return {
          anyOf: options.map((opt: any) => this.zodToJsonSchema(opt))
        };
      }
    }

    // Handle intersections
    if (defType === 'intersection') {
      return {
        allOf: [
          this.zodToJsonSchema(def.left),
          this.zodToJsonSchema(def.right)
        ]
      };
    }

    // Handle object type
    if (def.shape || defType === 'object') {
      const shape = def.shape || {};
      const properties: any = {};
      const required: string[] = [];

      for (const [key, value] of Object.entries(shape)) {
        properties[key] = this.zodToJsonSchema(value as z.ZodSchema<any>);
        const valueDef = (value as any)._def;
        const isOptional = (value as any).isOptional?.() ?? false;
        const hasDefault = valueDef?.type === 'default';
        if (!isOptional && !hasDefault) {
          required.push(key);
        }
      }

      const result: any = { type: 'object', properties };
      if (required.length > 0) result.required = required;

      if (def.catchall) {
        const catchallType = (def.catchall as any)._def?.type;
        if (catchallType === 'unknown') result.additionalProperties = true;
        else if (catchallType === 'never') result.additionalProperties = false;
      }

      if (def.unknownKeys === 'passthrough') result.additionalProperties = true;
      else if (def.unknownKeys === 'strict') result.additionalProperties = false;

      return result;
    }

    // Handle array type
    if (defType === 'array') {
      const itemSchema = def.element;
      const result: any = {
        type: 'array',
        items: this.zodToJsonSchema(itemSchema)
      };

      if (def.checks && Array.isArray(def.checks)) {
        for (const check of def.checks) {
          const checkDef = check._zod?.def || check.def || check;
          switch (checkDef.check) {
            case 'min_length': result.minItems = checkDef.minimum || checkDef.value; break;
            case 'max_length': result.maxItems = checkDef.maximum || checkDef.value; break;
            case 'exact_length': result.minItems = checkDef.value; result.maxItems = checkDef.value; break;
          }
        }
      }

      if (def.minLength) result.minItems = def.minLength.value || def.minLength;
      if (def.maxLength) result.maxItems = def.maxLength.value || def.maxLength;
      if (def.exactLength) {
        const exact = def.exactLength.value || def.exactLength;
        result.minItems = exact;
        result.maxItems = exact;
      }

      return result;
    }

    // Handle tuple
    if (defType === 'tuple') {
      const items = def.items?.map((item: any) => this.zodToJsonSchema(item)) || [];
      const result: any = {
        type: 'array',
        prefixItems: items,
        minItems: items.length,
        maxItems: def.rest ? undefined : items.length
      };
      if (def.rest) result.items = this.zodToJsonSchema(def.rest);
      else result.items = false;
      return result;
    }

    if (defType === 'set') {
      return { type: 'array', uniqueItems: true, items: def.valueType ? this.zodToJsonSchema(def.valueType) : {} };
    }

    if (defType === 'map') {
      return { type: 'object', additionalProperties: def.valueType ? this.zodToJsonSchema(def.valueType) : true };
    }

    if (defType === 'record') {
      return { type: 'object', additionalProperties: def.valueType ? this.zodToJsonSchema(def.valueType) : true };
    }

    if (defType === 'enum' || defType === 'nativeEnum') {
      if (def.values && Array.isArray(def.values)) return { type: 'string', enum: def.values };
      if (def.entries) return { type: 'string', enum: Object.keys(def.entries) };
    }

    if (defType === 'string') {
      const result: any = { type: 'string' };
      if (def.checks && Array.isArray(def.checks)) {
        for (const check of def.checks) {
          const checkDef = check._zod?.def || check.def || check;
          switch (checkDef.check) {
            case 'min_length': result.minLength = checkDef.minimum || checkDef.value; break;
            case 'max_length': result.maxLength = checkDef.maximum || checkDef.value; break;
            case 'length_equals': result.minLength = checkDef.length; result.maxLength = checkDef.length; break;
            case 'string_format':
              switch (checkDef.format) {
                case 'email': result.format = 'email'; break;
                case 'url': result.format = 'uri'; break;
                case 'uuid': result.format = 'uuid'; break;
                case 'regex': if (checkDef.pattern) result.pattern = checkDef.pattern.source || checkDef.pattern; break;
              }
              break;
            case 'regex': result.pattern = checkDef.pattern?.source || checkDef.pattern; break;
          }
          if (check.def && check.def.format) {
            switch (check.def.format) {
              case 'email': result.format = 'email'; break;
              case 'url': result.format = 'uri'; break;
              case 'uuid': result.format = 'uuid'; break;
            }
          }
        }
      }
      return result;
    }

    if (defType === 'number' || defType === 'bigint') {
      const result: any = { type: defType === 'bigint' ? 'integer' : 'number' };
      if (def.checks && Array.isArray(def.checks)) {
        for (const check of def.checks) {
          const checkDef = check._zod?.def || check.def || check;
          switch (checkDef.check) {
            case 'number_format': if (checkDef.format === 'safeint') result.type = 'integer'; break;
            case 'greater_than': result.minimum = checkDef.value; if (!checkDef.inclusive) result.exclusiveMinimum = true; break;
            case 'less_than': result.maximum = checkDef.value; if (!checkDef.inclusive) result.exclusiveMaximum = true; break;
            case 'multiple_of': result.multipleOf = checkDef.value; break;
          }
          if (check.isInt === true) result.type = 'integer';
        }
      }
      return result;
    }

    if (defType === 'boolean') return { type: 'boolean' };
    if (defType === 'date') return { type: 'string', format: 'date-time' };
    if (defType === 'null') return { type: 'null' };
    if (defType === 'undefined') return { type: 'null' };
    if (defType === 'void') return { type: 'null' };
    if (defType === 'any' || defType === 'unknown') return {};
    if (defType === 'never') return { not: {} };
    if (defType === 'promise') return this.zodToJsonSchema(def.innerType || def.type);
    if (defType === 'function') return { type: 'string', description: 'Function (not serializable)' };

    return { type: 'string' };
  }
}
