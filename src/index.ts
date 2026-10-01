// ─── Core ────────────────────────────────────────────────────────────────────
export { RLM, RLMBuilder, RLMCompletionResult, RLMResultFormatter } from './rlm';

// ─── Types ───────────────────────────────────────────────────────────────────
export { RLMConfig, RLMResult, RLMStats, MetaAgentConfig, ObservabilityConfig, TraceEvent, FileStorageConfig, ContextOverflowConfig } from './bridge-interface';
export { BridgeType } from './bridge-factory';
export { resolveGoBinary, GoBinaryResolution, GoBinarySource } from './binary-resolver';
export { StructuredRLMResult, SubTask, CoordinatorConfig, SchemaDecomposition } from './structured-types';
export { RLMExtendedConfig, ValidationResult, ValidationIssue, ValidationLevel, validateConfig, assertValidConfig } from './config';

// ─── Errors ──────────────────────────────────────────────────────────────────
export {
  RLMError,
  RLMValidationError,
  RLMRateLimitError,
  RLMTimeoutError,
  RLMProviderError,
  RLMBinaryError,
  RLMConfigError,
  RLMSchemaError,
  RLMContextOverflowError,
  RLMAbortError,
  classifyError,
} from './errors';

// ─── Streaming ───────────────────────────────────────────────────────────────
export {
  RLMStream,
  StreamOptions,
  StreamChunk,
  StreamChunkType,
  TextStreamChunk,
  PartialObjectStreamChunk,
  UsageStreamChunk,
  ErrorStreamChunk,
  DoneStreamChunk,
  createSimulatedStream,
} from './streaming';

// ─── Cache ───────────────────────────────────────────────────────────────────
export { RLMCache, CacheConfig, CacheStats, CacheProvider, MemoryCache, FileCache } from './cache';

// ─── Retry / Resilience ──────────────────────────────────────────────────────
export { RetryConfig, FallbackConfig, withRetry, withFallback } from './retry';

// ─── Events ──────────────────────────────────────────────────────────────────
export {
  RLMEventEmitter,
  RLMEventMap,
  RLMEventType,
  LLMCallEvent,
  LLMResponseEvent,
  ValidationRetryEvent,
  RecursionEvent,
  MetaAgentEvent,
  ErrorEvent,
  CompletionStartEvent,
  CompletionEndEvent,
  CacheEvent,
  RetryEvent,
} from './events';

// ─── Coordinator ─────────────────────────────────────────────────────────────
export { RLMAgentCoordinator } from './coordinator';

// ─── File Storage ────────────────────────────────────────────────────────────
export {
  FileContextBuilder,
  FileStorageProvider,
  FileStorageResult,
  FileEntry,
  LocalFileStorage,
  S3FileStorage,
  S3StorageError,
  buildFileContext,
} from './file-storage';
