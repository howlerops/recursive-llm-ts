/**
 * Configuration validation for recursive-llm-ts.
 *
 * Validates RLMConfig at construction time with clear error messages.
 */

import { RLMConfigError } from './errors';
import { RLMConfig } from './bridge-interface';
import { CacheConfig } from './cache';
import { RetryConfig, FallbackConfig } from './retry';

// ─── Extended Config (with new features) ─────────────────────────────────────

export interface RLMExtendedConfig extends RLMConfig {
  /** Cache configuration */
  cache?: CacheConfig;
  /** Retry configuration */
  retry?: RetryConfig;
  /** Fallback model configuration */
  fallback?: FallbackConfig;
  /** LiteLLM passthrough parameters */
  litellm_params?: Record<string, unknown>;
}

// ─── Known Config Keys ───────────────────────────────────────────────────────

const KNOWN_CONFIG_KEYS = new Set([
  'recursive_model',
  'api_base',
  'api_key',
  'max_depth',
  'max_iterations',
  'go_binary_path',
  'meta_agent',
  'observability',
  'debug',
  'cache',
  'retry',
  'fallback',
  'litellm_params',
  // Legacy LiteLLM passthrough keys (commonly used)
  'api_version',
  'timeout',
  'temperature',
  'max_tokens',
]);

// ─── Validation Issues ───────────────────────────────────────────────────────

export type ValidationLevel = 'error' | 'warning' | 'info';

export interface ValidationIssue {
  level: ValidationLevel;
  field: string;
  message: string;
}

export interface ValidationResult {
  valid: boolean;
  issues: ValidationIssue[];
}

// ─── Config Validator ────────────────────────────────────────────────────────

/**
 * Validate an RLM configuration.
 * Returns issues rather than throwing, allowing callers to handle gracefully.
 */
export function validateConfig(config: RLMExtendedConfig): ValidationResult {
  const issues: ValidationIssue[] = [];

  // Check for unknown keys (likely typos)
  for (const key of Object.keys(config)) {
    if (!KNOWN_CONFIG_KEYS.has(key)) {
      issues.push({
        level: 'warning',
        field: key,
        message: `Unknown config key "${key}". This may be a typo. Known keys: ${Array.from(KNOWN_CONFIG_KEYS).join(', ')}`,
      });
    }
  }

  // Validate numeric fields
  if (config.max_depth !== undefined) {
    if (typeof config.max_depth !== 'number' || config.max_depth < 1) {
      issues.push({ level: 'error', field: 'max_depth', message: 'max_depth must be a positive integer' });
    }
  }

  if (config.max_iterations !== undefined) {
    if (typeof config.max_iterations !== 'number' || config.max_iterations < 1) {
      issues.push({ level: 'error', field: 'max_iterations', message: 'max_iterations must be a positive integer' });
    }
  }

  // Validate API base URL
  if (config.api_base !== undefined && typeof config.api_base === 'string') {
    try {
      new URL(config.api_base);
    } catch {
      issues.push({ level: 'error', field: 'api_base', message: `api_base "${config.api_base}" is not a valid URL` });
    }
  }

  // Validate meta_agent config
  if (config.meta_agent) {
    if (typeof config.meta_agent.enabled !== 'boolean') {
      issues.push({ level: 'error', field: 'meta_agent.enabled', message: 'meta_agent.enabled must be a boolean' });
    }
    if (config.meta_agent.max_optimize_len !== undefined) {
      if (typeof config.meta_agent.max_optimize_len !== 'number' || config.meta_agent.max_optimize_len < 0) {
        issues.push({ level: 'error', field: 'meta_agent.max_optimize_len', message: 'max_optimize_len must be a non-negative number' });
      }
    }
  }

  // Validate observability config
  if (config.observability) {
    const obs = config.observability;
    if (obs.trace_enabled && !obs.trace_endpoint) {
      issues.push({
        level: 'warning',
        field: 'observability.trace_endpoint',
        message: 'trace_enabled is true but no trace_endpoint is configured. Spans will be printed to stderr (or log_output) instead of exported.',
      });
    }
    if (obs.langfuse_enabled) {
      if (!obs.langfuse_public_key && !process.env.LANGFUSE_PUBLIC_KEY) {
        issues.push({
          level: 'warning',
          field: 'observability.langfuse_public_key',
          message: 'langfuse_enabled is true but no public key is set (config or LANGFUSE_PUBLIC_KEY env var).',
        });
      }
      if (!obs.langfuse_secret_key && !process.env.LANGFUSE_SECRET_KEY) {
        issues.push({
          level: 'warning',
          field: 'observability.langfuse_secret_key',
          message: 'langfuse_enabled is true but no secret key is set (config or LANGFUSE_SECRET_KEY env var).',
        });
      }
    }
  }

  // Validate cache config
  if (config.cache) {
    if (config.cache.maxEntries !== undefined && (typeof config.cache.maxEntries !== 'number' || config.cache.maxEntries < 1)) {
      issues.push({ level: 'error', field: 'cache.maxEntries', message: 'cache.maxEntries must be a positive integer' });
    }
    if (config.cache.ttl !== undefined && (typeof config.cache.ttl !== 'number' || config.cache.ttl < 0)) {
      issues.push({ level: 'error', field: 'cache.ttl', message: 'cache.ttl must be a non-negative number (seconds)' });
    }
  }

  // Validate retry config
  if (config.retry) {
    if (config.retry.maxRetries !== undefined && (typeof config.retry.maxRetries !== 'number' || config.retry.maxRetries < 0)) {
      issues.push({ level: 'error', field: 'retry.maxRetries', message: 'retry.maxRetries must be a non-negative integer' });
    }
    if (config.retry.baseDelay !== undefined && (typeof config.retry.baseDelay !== 'number' || config.retry.baseDelay < 0)) {
      issues.push({ level: 'error', field: 'retry.baseDelay', message: 'retry.baseDelay must be a non-negative number (ms)' });
    }
  }

  // Check for missing API key
  if (!config.api_key && !process.env.OPENAI_API_KEY) {
    issues.push({
      level: 'info',
      field: 'api_key',
      message: 'No API key configured (api_key or OPENAI_API_KEY env var). Ensure it is set before making completions.',
    });
  }

  return {
    valid: issues.filter(i => i.level === 'error').length === 0,
    issues,
  };
}

/**
 * Validate config and throw on errors. Logs warnings.
 */
export function assertValidConfig(config: RLMExtendedConfig): void {
  const result = validateConfig(config);

  for (const issue of result.issues) {
    if (issue.level === 'warning') {
      console.warn(`[recursive-llm-ts] Warning: ${issue.message}`);
    }
  }

  const errors = result.issues.filter(i => i.level === 'error');
  if (errors.length > 0) {
    throw new RLMConfigError({
      message: `Invalid RLM config: ${errors.map(e => e.message).join('; ')}`,
      field: errors[0].field,
      value: (config as any)[errors[0].field],
    });
  }
}
