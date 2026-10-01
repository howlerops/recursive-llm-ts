export interface RLMStats {
  llm_calls: number;
  iterations: number;
  depth: number;
  parsing_retries?: number;
  total_tokens?: number;
  prompt_tokens?: number;
  completion_tokens?: number;
}

export interface RLMResult {
  result: string; // Text result for normal completions (structured uses StructuredRLMResult<T>)
  stats: RLMStats;
  structured_result?: boolean; // Flag indicating if result is structured
  trace_events?: TraceEvent[]; // Observability trace events when enabled
}

export interface MetaAgentConfig {
  enabled: boolean;
  model?: string; // Model to use for meta-agent (defaults to main model)
  max_optimize_len?: number; // Max context length before optimization (0 = always optimize)
}

export interface ObservabilityConfig {
  debug?: boolean; // Enable verbose debug logging of all internal operations
  trace_enabled?: boolean; // Enable OpenTelemetry tracing
  trace_endpoint?: string; // OTLP/HTTP endpoint for trace export (e.g., "http://localhost:4318"); "/v1/traces" is appended
  service_name?: string; // Service name for traces (default: "rlm")
  log_output?: string; // Where to write debug logs ("stderr", "stdout", or a file path)
  langfuse_enabled?: boolean; // Send traces to Langfuse (batched to {langfuse_host}/api/public/ingestion at the end of each call)
  langfuse_public_key?: string; // Langfuse public key
  langfuse_secret_key?: string; // Langfuse secret key
  langfuse_host?: string; // Langfuse API host (default: "https://cloud.langfuse.com")
}

export interface TraceEvent {
  timestamp: string;
  type: string; // "span_start", "span_end", "llm_call", "log", "error", "event"
  name: string;
  attributes: Record<string, string>;
  duration?: number;
  trace_id?: string;
  span_id?: string;
  parent_id?: string;
}

export interface ContextOverflowConfig {
  /** Enable automatic context overflow recovery (default: true) */
  enabled?: boolean;
  /** Override detected model token limit (0 = auto-detect from API errors) */
  max_model_tokens?: number;
  /** Strategy: 'mapreduce' (default), 'truncate', 'chunked', 'tfidf', 'textrank', or 'refine' */
  strategy?: 'mapreduce' | 'truncate' | 'chunked' | 'tfidf' | 'textrank' | 'refine';
  /** Fraction of token budget to reserve for prompts/overhead (default: 0.15) */
  safety_margin?: number;
  /** Maximum reduction attempts before giving up (default: 3) */
  max_reduction_attempts?: number;
}

// ─── LCM (Lossless Context Management) ─────────────────────────────────────

export interface LCMConfig {
  /** Enable LCM context management (default: false for backward compat) */
  enabled?: boolean;
  /** Soft token threshold — async compaction begins above this (default: 70% of model limit) */
  soft_threshold?: number;
  /** Hard token threshold — blocking compaction above this (default: 90% of model limit) */
  hard_threshold?: number;
  /** Number of messages to compact at once (default: 10) */
  compaction_block_size?: number;
  /** Target tokens per summary node (default: 500) */
  summary_target_tokens?: number;
  /** Large file handling configuration */
  file_handling?: LCMFileConfig;
  /** Episode-based context grouping configuration */
  episodes?: EpisodeConfig;
  /** Persistence backend configuration (default: in-memory) */
  store_backend?: StoreBackendConfig;
}

export interface LCMFileConfig {
  /** Token count above which files are stored externally with exploration summaries (default: 25000) */
  token_threshold?: number;
}

export interface EpisodeConfig {
  /** Max tokens before auto-closing an episode (default: 2000) */
  max_episode_tokens?: number;
  /** Max messages before auto-closing an episode (default: 20) */
  max_episode_messages?: number;
  /** Topic change sensitivity 0-1 (reserved for future semantic detection) */
  topic_change_threshold?: number;
  /** Auto-generate summary when episode closes (default: true) */
  auto_compact_after_close?: boolean;
}

export interface Episode {
  id: string;
  title: string;
  message_ids: string[];
  start_time: string;
  end_time: string;
  tokens: number;
  summary?: string;
  summary_tokens?: number;
  status: 'active' | 'compacted' | 'archived';
  tags?: string[];
  parent_episode_id?: string;
}

export interface StoreBackendConfig {
  /** Backend type: 'memory' (default) or 'sqlite' */
  type?: 'memory' | 'sqlite';
  /** Path for SQLite database file (required when type is 'sqlite', use ':memory:' for in-memory SQLite) */
  path?: string;
}
export interface LLMMapConfig {
  /** Path to JSONL input file */
  input_path: string;
  /** Path to JSONL output file */
  output_path: string;
  /** Prompt template — use {{item}} as placeholder for each item */
  prompt: string;
  /** JSON Schema for output validation */
  output_schema?: Record<string, any>;
  /** Worker pool concurrency (default: 16) */
  concurrency?: number;
  /** Per-item retry limit (default: 3) */
  max_retries?: number;
  /** Model to use (defaults to engine model) */
  model?: string;
}

export interface LLMMapResult {
  total_items: number;
  completed: number;
  failed: number;
  output_path: string;
  duration_ms: number;
  tokens_used: number;
}

export interface AgenticMapConfig {
  /** Path to JSONL input file */
  input_path: string;
  /** Path to JSONL output file */
  output_path: string;
  /** Prompt template — use {{item}} as placeholder for each item */
  prompt: string;
  /** JSON Schema for output validation */
  output_schema?: Record<string, any>;
  /** Worker pool concurrency (default: 8) */
  concurrency?: number;
  /** Per-item retry limit (default: 2) */
  max_retries?: number;
  /** Model for sub-agents (defaults to engine model) */
  model?: string;
  /** If true, sub-agents cannot modify filesystem */
  read_only?: boolean;
  /** Max recursion depth for sub-agents (default: 3) */
  max_depth?: number;
  /** Max iterations per sub-agent (default: 15) */
  max_iterations?: number;
}

export interface AgenticMapResult {
  total_items: number;
  completed: number;
  failed: number;
  output_path: string;
  duration_ms: number;
  tokens_used: number;
}

export interface DelegationRequest {
  /** Task description for the sub-agent */
  prompt: string;
  /** Specific slice of work being handed off (required for non-root) */
  delegated_scope?: string;
  /** Work the caller retains (required for non-root) */
  kept_work?: string;
  /** Read-only exploration agent (exempt from guard) */
  read_only?: boolean;
  /** Parallel decomposition (exempt from guard) */
  parallel?: boolean;
}

export interface LCMStoreStats {
  total_messages: number;
  total_summaries: number;
  active_context_items: number;
  active_context_tokens: number;
  immutable_store_tokens: number;
  compression_ratio: number;
}

export interface LCMGrepResult {
  message_id: string;
  role: string;
  content: string;
  summary_id?: string;
  match_line: string;
}

export interface LCMDescribeResult {
  type: 'message' | 'summary';
  id: string;
  tokens: number;
  role?: string;
  kind?: 'leaf' | 'condensed';
  level?: number;
  covered_ids?: string[];
  file_ids?: string[];
  content?: string;
}

export interface EpisodeListResult {
  episodes: Episode[];
  active_episode_id?: string;
  total_episodes: number;
}

export interface RLMConfig {
  recursive_model?: string;
  api_base?: string;
  api_key?: string;
  max_depth?: number;
  max_iterations?: number;
  go_binary_path?: string; // Optional override path for Go binary

  // Meta-agent configuration
  meta_agent?: MetaAgentConfig;

  // Observability configuration
  observability?: ObservabilityConfig;

  // Context overflow recovery configuration
  context_overflow?: ContextOverflowConfig;

  // LCM (Lossless Context Management) configuration
  lcm?: LCMConfig;

  // Shorthand for observability.debug
  debug?: boolean;

  // LiteLLM passthrough parameters (commonly used)
  api_version?: string;          // API version (e.g., for Azure)
  timeout?: number;              // Request timeout in seconds
  temperature?: number;          // Sampling temperature
  max_tokens?: number;           // Maximum tokens in response

  // Structured output config (internal - set by structuredCompletion)
  structured?: any;

  // Allow arbitrary passthrough parameters for LiteLLM and other providers
  // (e.g., custom_llm_provider, top_p, frequency_penalty, etc.)
  [key: string]: any;
}

export interface FileStorageConfig {
  /** Storage type: 'local' or 's3' */
  type: 'local' | 's3';
  /** For local: root directory path. For S3: bucket name */
  path: string;
  /** For S3: the prefix (folder path) within the bucket */
  prefix?: string;
  /** For S3: AWS region (falls back to AWS_REGION env var, then 'us-east-1') */
  region?: string;
  /**
   * For S3: explicit credentials.
   * Resolution order:
   * 1. This field (explicit credentials)
   * 2. Environment variables: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN
   * 3. AWS SDK default credential chain (IAM role, ~/.aws/credentials, ECS task role, etc.)
   */
  credentials?: { accessKeyId: string; secretAccessKey: string; sessionToken?: string };
  /**
   * For S3: custom endpoint URL.
   * Use for S3-compatible services: MinIO, LocalStack, DigitalOcean Spaces, Backblaze B2.
   * When set, forcePathStyle is automatically enabled.
   */
  endpoint?: string;
  /**
   * For S3: force path-style addressing (bucket in path, not subdomain).
   * Automatically true when endpoint is set.
   */
  forcePathStyle?: boolean;
  /** Glob patterns to include (e.g. ['*.ts', '*.md']) */
  includePatterns?: string[];
  /** Glob patterns to exclude (e.g. ['node_modules/**']) */
  excludePatterns?: string[];
  /** Maximum file size in bytes to include (default: 1MB) */
  maxFileSize?: number;
  /** Maximum total context size in bytes (default: 10MB) */
  maxTotalSize?: number;
  /** Maximum number of files to include (default: 1000) */
  maxFiles?: number;
  /** File extensions to include (e.g. ['.ts', '.md', '.txt']) */
  extensions?: string[];
}

export interface Bridge {
  completion(
    model: string,
    query: string,
    context: string,
    rlmConfig: RLMConfig
  ): Promise<RLMResult>;

  cleanup(): Promise<void>;
}