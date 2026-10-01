/**
 * Structured error hierarchy for recursive-llm-ts.
 *
 * All library errors extend {@link RLMError}, which adds:
 * - `code` – machine-readable error identifier
 * - `retryable` – whether the caller should retry
 * - `suggestion` – human-readable remediation hint
 */

// ─── Base Error ──────────────────────────────────────────────────────────────

export class RLMError extends Error {
  /** Machine-readable error code (e.g. "RATE_LIMIT", "VALIDATION") */
  public readonly code: string;
  /** Whether this error is likely to succeed on retry */
  public readonly retryable: boolean;
  /** Human-readable suggestion for resolving the error */
  public readonly suggestion?: string;

  constructor(message: string, opts: { code: string; retryable?: boolean; suggestion?: string }) {
    super(message);
    this.name = 'RLMError';
    this.code = opts.code;
    this.retryable = opts.retryable ?? false;
    this.suggestion = opts.suggestion;
  }
}

// ─── Validation Errors ───────────────────────────────────────────────────────

/** Thrown when a structured output fails schema validation. */
export class RLMValidationError extends RLMError {
  public readonly expected: unknown;
  public readonly received: unknown;
  public readonly zodErrors?: unknown;

  constructor(opts: { message: string; expected: unknown; received: unknown; zodErrors?: unknown }) {
    super(opts.message, {
      code: 'VALIDATION',
      retryable: true,
      suggestion: 'The LLM returned output that did not match the Zod schema. Try a more capable model, simplify your schema, or increase maxRetries.',
    });
    this.name = 'RLMValidationError';
    this.expected = opts.expected;
    this.received = opts.received;
    this.zodErrors = opts.zodErrors;
  }
}

// ─── Rate Limit ──────────────────────────────────────────────────────────────

/** Thrown when the LLM provider returns a 429 rate limit response. */
export class RLMRateLimitError extends RLMError {
  public readonly retryAfter?: number;

  constructor(opts: { message: string; retryAfter?: number }) {
    super(opts.message, {
      code: 'RATE_LIMIT',
      retryable: true,
      suggestion: opts.retryAfter
        ? `Rate limited. Retry after ${opts.retryAfter}s.`
        : 'Rate limited. Implement exponential backoff or upgrade your API tier.',
    });
    this.name = 'RLMRateLimitError';
    this.retryAfter = opts.retryAfter;
  }
}

// ─── Timeout ─────────────────────────────────────────────────────────────────

/** Thrown when an operation exceeds its time limit. */
export class RLMTimeoutError extends RLMError {
  public readonly elapsed: number;
  public readonly limit: number;

  constructor(opts: { message: string; elapsed: number; limit: number }) {
    super(opts.message, {
      code: 'TIMEOUT',
      retryable: true,
      suggestion: `Operation timed out after ${opts.elapsed}ms (limit: ${opts.limit}ms). Increase the timeout or reduce context size.`,
    });
    this.name = 'RLMTimeoutError';
    this.elapsed = opts.elapsed;
    this.limit = opts.limit;
  }
}

// ─── Provider / API ──────────────────────────────────────────────────────────

/** Thrown when the LLM provider returns an HTTP error. */
export class RLMProviderError extends RLMError {
  public readonly statusCode: number;
  public readonly provider: string;
  public readonly responseBody?: string;

  constructor(opts: { message: string; statusCode: number; provider: string; responseBody?: string }) {
    const retryable = opts.statusCode >= 500 || opts.statusCode === 429;
    super(opts.message, {
      code: 'PROVIDER',
      retryable,
      suggestion: retryable
        ? `Provider "${opts.provider}" returned ${opts.statusCode}. This is likely transient – retry with backoff.`
        : `Provider "${opts.provider}" returned ${opts.statusCode}. Check your API key, model name, and request parameters.`,
    });
    this.name = 'RLMProviderError';
    this.statusCode = opts.statusCode;
    this.provider = opts.provider;
    this.responseBody = opts.responseBody;
  }
}

// ─── Binary / Bridge ─────────────────────────────────────────────────────────

/** Thrown when the Go binary cannot be found or fails to start. */
export class RLMBinaryError extends RLMError {
  public readonly binaryPath: string;
  /** Every location checked while resolving the binary (when known) */
  public readonly searched?: string[];

  constructor(opts: { message: string; binaryPath: string; searched?: string[] }) {
    super(opts.message, {
      code: 'BINARY',
      retryable: false,
      suggestion: `Go binary not found at "${opts.binaryPath}". Build it with: npm run build:go (or node scripts/build-go-binary.js)`,
    });
    this.name = 'RLMBinaryError';
    this.binaryPath = opts.binaryPath;
    this.searched = opts.searched;
  }
}

// ─── Configuration ───────────────────────────────────────────────────────────

/** Thrown when the RLM configuration is invalid. */
export class RLMConfigError extends RLMError {
  public readonly field: string;
  public readonly value: unknown;

  constructor(opts: { message: string; field: string; value: unknown }) {
    super(opts.message, {
      code: 'CONFIG',
      retryable: false,
      suggestion: `Invalid configuration for "${opts.field}". Check the RLMConfig documentation.`,
    });
    this.name = 'RLMConfigError';
    this.field = opts.field;
    this.value = opts.value;
  }
}

// ─── Schema ──────────────────────────────────────────────────────────────────

/** Thrown when a JSON Schema is malformed or unsupported. */
export class RLMSchemaError extends RLMError {
  public readonly path: string;
  public readonly constraint: string;

  constructor(opts: { message: string; path: string; constraint: string }) {
    super(opts.message, {
      code: 'SCHEMA',
      retryable: false,
      suggestion: `Schema issue at "${opts.path}": ${opts.constraint}. Simplify your Zod schema or check for unsupported types.`,
    });
    this.name = 'RLMSchemaError';
    this.path = opts.path;
    this.constraint = opts.constraint;
  }
}

// ─── Context Overflow ─────────────────────────────────────────────────────────

/** Thrown when the request exceeds the model's context window. */
export class RLMContextOverflowError extends RLMError {
  public readonly modelLimit: number;
  public readonly requestTokens: number;

  constructor(opts: { message: string; modelLimit: number; requestTokens: number }) {
    super(opts.message, {
      code: 'CONTEXT_OVERFLOW',
      retryable: true,
      suggestion: `Request has ${opts.requestTokens} tokens but model limit is ${opts.modelLimit}. ` +
        'Enable context_overflow handling or reduce your context size.',
    });
    this.name = 'RLMContextOverflowError';
    this.modelLimit = opts.modelLimit;
    this.requestTokens = opts.requestTokens;
  }
}

// ─── Abort ───────────────────────────────────────────────────────────────────

/** Thrown when an operation is aborted via AbortController. */
export class RLMAbortError extends RLMError {
  constructor(message = 'Operation was aborted') {
    super(message, {
      code: 'ABORTED',
      retryable: false,
      suggestion: 'The operation was cancelled. This is expected when using AbortController.',
    });
    this.name = 'RLMAbortError';
  }
}

// ─── Error Classification Helper ─────────────────────────────────────────────

/**
 * Classify a raw Error into the appropriate RLM error type.
 * Used internally by the bridge layer to wrap Go binary errors.
 */
export function classifyError(err: Error | string, context?: { binaryPath?: string; provider?: string }): RLMError {
  const msg = typeof err === 'string' ? err : err.message;

  // Context overflow
  if (msg.toLowerCase().includes('maximum context length') || msg.toLowerCase().includes('context_length_exceeded') || msg.toLowerCase().includes('too many input tokens')) {
    const limitMatch = msg.match(/maximum context length is (\d[\d,]*)/i);
    const requestMatch = msg.match(/(?:has|requested) (\d[\d,]*)\s*(?:input )?tokens/i);
    const modelLimit = limitMatch ? parseInt(limitMatch[1].replace(/,/g, ''), 10) : 0;
    const requestTokens = requestMatch ? parseInt(requestMatch[1].replace(/,/g, ''), 10) : 0;
    return new RLMContextOverflowError({ message: msg, modelLimit, requestTokens });
  }

  // Rate limit
  if (msg.includes('429') || msg.toLowerCase().includes('rate limit') || msg.toLowerCase().includes('too many requests')) {
    const retryMatch = msg.match(/retry.after[:\s]+(\d+)/i);
    return new RLMRateLimitError({
      message: msg,
      retryAfter: retryMatch ? parseInt(retryMatch[1], 10) : undefined,
    });
  }

  // Timeout
  if (msg.toLowerCase().includes('timeout') || msg.includes('ETIMEDOUT') || msg.includes('ESOCKETTIMEDOUT')) {
    return new RLMTimeoutError({ message: msg, elapsed: 0, limit: 0 });
  }

  // Binary not found
  if ((msg.includes('not found') && msg.includes('binary')) || (msg.includes('ENOENT') && context?.binaryPath)) {
    return new RLMBinaryError({ message: msg, binaryPath: context?.binaryPath ?? 'unknown' });
  }

  // Provider errors with HTTP codes
  const statusMatch = msg.match(/(?:status|code)[:\s]+(\d{3})/i);
  if (statusMatch) {
    const statusCode = parseInt(statusMatch[1], 10);
    if (statusCode === 429) {
      return new RLMRateLimitError({ message: msg });
    }
    return new RLMProviderError({
      message: msg,
      statusCode,
      provider: context?.provider || 'unknown',
    });
  }

  // Validation
  if (msg.toLowerCase().includes('validation') || msg.toLowerCase().includes('schema') || msg.toLowerCase().includes('parse')) {
    return new RLMValidationError({
      message: msg,
      expected: undefined,
      received: undefined,
    });
  }

  // Fallback
  return new RLMError(msg, { code: 'UNKNOWN', retryable: false });
}
