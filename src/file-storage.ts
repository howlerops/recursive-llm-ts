import * as fs from 'fs';
import * as path from 'path';
import { FileStorageConfig } from './bridge-interface';
import { moduleRequire } from './module-context';

// Dynamic loader for optional @aws-sdk/client-s3 dependency.
// Uses require() to avoid TypeScript resolving the import at compile time;
// moduleRequire works in both the CJS and ESM builds.
function loadS3SDK(): any {
  try {
    return moduleRequire('@aws-sdk/client-s3');
  } catch {
    throw new Error(
      'S3 file storage requires @aws-sdk/client-s3. Install it with: npm install @aws-sdk/client-s3'
    );
  }
}

// ─── S3 Error Handling ──────────────────────────────────────────────────────

/**
 * Custom error class for S3 storage operations.
 * Provides actionable error messages for common failure modes.
 */
export class S3StorageError extends Error {
  public readonly code: string;
  public readonly bucket: string;
  public readonly key?: string;
  public readonly originalError?: Error;

  constructor(opts: { message: string; code: string; bucket: string; key?: string; originalError?: Error }) {
    super(opts.message);
    this.name = 'S3StorageError';
    this.code = opts.code;
    this.bucket = opts.bucket;
    this.key = opts.key;
    this.originalError = opts.originalError;
  }
}

/**
 * Wraps raw AWS SDK errors with actionable, user-friendly messages.
 */
function wrapS3Error(err: any, bucket: string, key?: string): S3StorageError {
  const errName = err?.name || err?.Code || '';
  const errCode = err?.$metadata?.httpStatusCode || err?.statusCode || 0;
  const errMessage = err?.message || String(err);

  // Authentication / credentials errors
  if (
    errName === 'CredentialsProviderError' ||
    errName === 'InvalidIdentityToken' ||
    errName === 'ExpiredToken' ||
    errName === 'ExpiredTokenException' ||
    errCode === 401 ||
    errMessage.includes('credentials') ||
    errMessage.includes('Could not load credentials')
  ) {
    return new S3StorageError({
      message: `S3 authentication failed for bucket "${bucket}". ` +
        `Ensure credentials are provided via: (1) explicit config.credentials, ` +
        `(2) environment variables AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY, or ` +
        `(3) AWS SDK default credential chain (IAM role, ~/.aws/credentials, etc.). ` +
        `Original error: ${errMessage}`,
      code: 'AUTH_FAILED',
      bucket,
      key,
      originalError: err,
    });
  }

  // Access denied / permissions errors
  if (
    errName === 'AccessDenied' ||
    errName === 'Forbidden' ||
    errCode === 403
  ) {
    return new S3StorageError({
      message: `S3 access denied for bucket "${bucket}"${key ? `, key "${key}"` : ''}. ` +
        `The credentials are valid but lack permission. ` +
        `Required permissions: s3:ListBucket, s3:GetObject, s3:HeadObject. ` +
        `Original error: ${errMessage}`,
      code: 'ACCESS_DENIED',
      bucket,
      key,
      originalError: err,
    });
  }

  // Bucket or key not found
  if (
    errName === 'NoSuchBucket' ||
    errName === 'NotFound' ||
    (errCode === 404 && !key)
  ) {
    return new S3StorageError({
      message: `S3 bucket "${bucket}" not found. ` +
        `Verify the bucket name and region are correct. ` +
        `Original error: ${errMessage}`,
      code: 'BUCKET_NOT_FOUND',
      bucket,
      key,
      originalError: err,
    });
  }

  if (
    errName === 'NoSuchKey' ||
    (errCode === 404 && key)
  ) {
    return new S3StorageError({
      message: `S3 object not found: s3://${bucket}/${key}. ` +
        `The file may have been deleted or the path is incorrect. ` +
        `Original error: ${errMessage}`,
      code: 'KEY_NOT_FOUND',
      bucket,
      key,
      originalError: err,
    });
  }

  // Network / connectivity errors
  if (
    errName === 'NetworkingError' ||
    errName === 'TimeoutError' ||
    errCode === 'ECONNREFUSED' ||
    errMessage.includes('ECONNREFUSED') ||
    errMessage.includes('ENOTFOUND') ||
    errMessage.includes('ETIMEDOUT') ||
    errMessage.includes('fetch failed') ||
    errMessage.includes('getaddrinfo')
  ) {
    return new S3StorageError({
      message: `S3 network error for bucket "${bucket}". ` +
        `Could not connect to the S3 endpoint. ` +
        `If using a custom endpoint (MinIO, LocalStack), verify it is running and accessible. ` +
        `Original error: ${errMessage}`,
      code: 'NETWORK_ERROR',
      bucket,
      key,
      originalError: err,
    });
  }

  // Region mismatch
  if (
    errName === 'PermanentRedirect' ||
    errCode === 301 ||
    errMessage.includes('region') && errMessage.includes('redirect')
  ) {
    return new S3StorageError({
      message: `S3 region mismatch for bucket "${bucket}". ` +
        `The bucket exists in a different region. ` +
        `Set the correct region in FileStorageConfig.region or AWS_REGION env var. ` +
        `Original error: ${errMessage}`,
      code: 'REGION_MISMATCH',
      bucket,
      key,
      originalError: err,
    });
  }

  // Fallback for unknown errors
  return new S3StorageError({
    message: `S3 operation failed for bucket "${bucket}"${key ? `, key "${key}"` : ''}: ${errMessage}`,
    code: 'UNKNOWN',
    bucket,
    key,
    originalError: err,
  });
}

// ─── Credential Resolution ──────────────────────────────────────────────────

/**
 * Credential resolution order:
 * 1. Explicit credentials in FileStorageConfig.credentials
 * 2. Environment variables: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN
 * 3. AWS SDK default credential chain (IAM role, ~/.aws/credentials, ECS task role, etc.)
 */
function resolveCredentials(config: FileStorageConfig): { accessKeyId: string; secretAccessKey: string; sessionToken?: string } | undefined {
  // 1. Explicit credentials from config
  if (config.credentials) {
    return config.credentials;
  }

  // 2. Environment variables
  const accessKeyId = process.env.AWS_ACCESS_KEY_ID;
  const secretAccessKey = process.env.AWS_SECRET_ACCESS_KEY;
  if (accessKeyId && secretAccessKey) {
    return {
      accessKeyId,
      secretAccessKey,
      sessionToken: process.env.AWS_SESSION_TOKEN,
    };
  }

  // 3. Fall through to SDK default credential chain (returns undefined)
  return undefined;
}

// ─── Types ───────────────────────────────────────────────────────────────────

// Re-export FileStorageConfig from the canonical definition in bridge-interface
export { FileStorageConfig } from './bridge-interface';

export interface FileEntry {
  /** Relative path from the root of the storage source */
  relativePath: string;
  /** File content as a string */
  content: string;
  /** Size in bytes */
  size: number;
}

export interface FileStorageResult {
  /** The built context string containing all file contents */
  context: string;
  /** List of files that were included */
  files: Array<{ relativePath: string; size: number }>;
  /** Total size of all file contents */
  totalSize: number;
  /** Files that were skipped and why */
  skipped: Array<{ relativePath: string; reason: string }>;
}

// ─── Glob Matching ───────────────────────────────────────────────────────────

/**
 * Simple glob pattern matcher supporting *, **, and ? wildcards.
 * Does not depend on external libraries.
 */
function globToRegex(pattern: string): RegExp {
  let regex = '';
  let i = 0;
  while (i < pattern.length) {
    const c = pattern[i];
    if (c === '*') {
      if (pattern[i + 1] === '*') {
        // ** matches any number of path segments
        if (pattern[i + 2] === '/') {
          regex += '(?:.+/)?';
          i += 3;
        } else {
          regex += '.*';
          i += 2;
        }
      } else {
        // * matches anything except /
        regex += '[^/]*';
        i += 1;
      }
    } else if (c === '?') {
      regex += '[^/]';
      i += 1;
    } else if (c === '.') {
      regex += '\\.';
      i += 1;
    } else {
      regex += c;
      i += 1;
    }
  }
  return new RegExp('^' + regex + '$');
}

function matchesGlob(filePath: string, pattern: string): boolean {
  const regex = globToRegex(pattern);
  // Match against full relative path
  if (regex.test(filePath)) return true;
  // If pattern has no path separators, also match against just the filename
  if (!pattern.includes('/')) {
    return regex.test(path.basename(filePath));
  }
  return false;
}

function matchesAnyGlob(filePath: string, patterns: string[]): boolean {
  return patterns.some(p => matchesGlob(filePath, p));
}

// ─── File Storage Provider Interface ─────────────────────────────────────────

export interface FileStorageProvider {
  listFiles(): Promise<string[]>;
  readFile(relativePath: string): Promise<string>;
  getFileSize(relativePath: string): Promise<number>;
}

// ─── Local File Storage ──────────────────────────────────────────────────────

export class LocalFileStorage implements FileStorageProvider {
  private rootPath: string;

  constructor(rootPath: string) {
    this.rootPath = path.resolve(rootPath);
  }

  async listFiles(): Promise<string[]> {
    const files: string[] = [];
    await this.walkDir(this.rootPath, files);
    return files.sort();
  }

  private async walkDir(dir: string, files: string[]): Promise<void> {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    for (const entry of entries) {
      const fullPath = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        await this.walkDir(fullPath, files);
      } else if (entry.isFile()) {
        const relativePath = path.relative(this.rootPath, fullPath);
        files.push(relativePath);
      }
    }
  }

  async readFile(relativePath: string): Promise<string> {
    const fullPath = path.join(this.rootPath, relativePath);
    return fs.readFileSync(fullPath, 'utf-8');
  }

  async getFileSize(relativePath: string): Promise<number> {
    const fullPath = path.join(this.rootPath, relativePath);
    const stat = fs.statSync(fullPath);
    return stat.size;
  }
}

// ─── S3 File Storage ─────────────────────────────────────────────────────────

/**
 * S3FileStorage uses the AWS SDK v3 via dynamic import.
 * If @aws-sdk/client-s3 is not installed, it throws a clear error.
 *
 * Supports:
 * - AWS S3
 * - MinIO (set endpoint to MinIO URL)
 * - LocalStack (set endpoint to LocalStack URL, typically http://localhost:4566)
 * - DigitalOcean Spaces, Backblaze B2, and other S3-compatible services
 *
 * Credential resolution order:
 * 1. Explicit credentials in FileStorageConfig.credentials
 * 2. Environment variables: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN
 * 3. AWS SDK default credential chain (IAM role, ~/.aws/credentials, ECS task role, etc.)
 */
export class S3FileStorage implements FileStorageProvider {
  private bucket: string;
  private prefix: string;
  private region: string;
  private credentials?: {
    accessKeyId: string;
    secretAccessKey: string;
    sessionToken?: string;
  };
  private endpoint?: string;
  private forcePathStyle: boolean;
  private s3Client: any = null;

  constructor(config: FileStorageConfig) {
    this.bucket = config.path;
    this.prefix = config.prefix || '';
    this.region = config.region || process.env.AWS_REGION || process.env.AWS_DEFAULT_REGION || 'us-east-1';
    this.credentials = resolveCredentials(config);
    this.endpoint = config.endpoint;
    this.forcePathStyle = config.forcePathStyle ?? !!config.endpoint;
  }

  /**
   * Returns the credential source being used for debugging/logging.
   */
  getCredentialSource(): string {
    if (this.credentials) {
      // Check if they came from env vars or explicit config
      if (
        this.credentials.accessKeyId === process.env.AWS_ACCESS_KEY_ID &&
        this.credentials.secretAccessKey === process.env.AWS_SECRET_ACCESS_KEY
      ) {
        return 'environment';
      }
      return 'explicit';
    }
    return 'default-chain';
  }

  private async getClient(): Promise<any> {
    if (this.s3Client) return this.s3Client;

    const s3Module = loadS3SDK();
    const clientConfig: any = { region: this.region };

    if (this.credentials) {
      clientConfig.credentials = this.credentials;
    }
    if (this.endpoint) {
      clientConfig.endpoint = this.endpoint;
    }
    if (this.forcePathStyle) {
      clientConfig.forcePathStyle = true;
    }

    this.s3Client = new s3Module.S3Client(clientConfig);
    return this.s3Client;
  }

  private buildKey(relativePath: string): string {
    return this.prefix
      ? `${this.prefix.replace(/\/$/, '')}/${relativePath}`
      : relativePath;
  }

  async listFiles(): Promise<string[]> {
    let client: any;
    try {
      client = await this.getClient();
    } catch (err: any) {
      throw wrapS3Error(err, this.bucket);
    }

    const s3Module = loadS3SDK();
    const files: string[] = [];
    let continuationToken: string | undefined;

    try {
      do {
        const command = new s3Module.ListObjectsV2Command({
          Bucket: this.bucket,
          Prefix: this.prefix,
          ContinuationToken: continuationToken,
        });

        const response = await client.send(command);

        if (response.Contents) {
          for (const obj of response.Contents) {
            if (obj.Key && !obj.Key.endsWith('/')) {
              // Get relative path by removing the prefix
              const relativePath = this.prefix
                ? obj.Key.substring(this.prefix.length).replace(/^\//, '')
                : obj.Key;
              if (relativePath) {
                files.push(relativePath);
              }
            }
          }
        }

        continuationToken = response.IsTruncated
          ? response.NextContinuationToken
          : undefined;
      } while (continuationToken);
    } catch (err: any) {
      throw wrapS3Error(err, this.bucket);
    }

    return files.sort();
  }

  async readFile(relativePath: string): Promise<string> {
    const key = this.buildKey(relativePath);
    let client: any;
    try {
      client = await this.getClient();
    } catch (err: any) {
      throw wrapS3Error(err, this.bucket, key);
    }

    const s3Module = loadS3SDK();

    try {
      const command = new s3Module.GetObjectCommand({
        Bucket: this.bucket,
        Key: key,
      });

      const response = await client.send(command);
      const body = response.Body;

      if (!body) {
        throw new S3StorageError({
          message: `Empty response body for s3://${this.bucket}/${key}`,
          code: 'EMPTY_BODY',
          bucket: this.bucket,
          key,
        });
      }

      // Handle different stream types
      if (typeof body.transformToString === 'function') {
        return await body.transformToString('utf-8');
      }

      // Fallback: collect stream chunks
      const chunks: Buffer[] = [];
      for await (const chunk of body as AsyncIterable<Buffer>) {
        chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
      }
      return Buffer.concat(chunks).toString('utf-8');
    } catch (err: any) {
      if (err instanceof S3StorageError) throw err;
      throw wrapS3Error(err, this.bucket, key);
    }
  }

  async getFileSize(relativePath: string): Promise<number> {
    const key = this.buildKey(relativePath);
    let client: any;
    try {
      client = await this.getClient();
    } catch (err: any) {
      throw wrapS3Error(err, this.bucket, key);
    }

    const s3Module = loadS3SDK();

    try {
      const command = new s3Module.HeadObjectCommand({
        Bucket: this.bucket,
        Key: key,
      });

      const response = await client.send(command);
      if (response.ContentLength == null) {
        throw new S3StorageError({
          message: `S3 HeadObject returned no ContentLength for s3://${this.bucket}/${key}. The object may be corrupted.`,
          code: 'NO_CONTENT_LENGTH',
          bucket: this.bucket,
          key,
        });
      }
      return response.ContentLength;
    } catch (err: any) {
      if (err instanceof S3StorageError) throw err;
      throw wrapS3Error(err, this.bucket, key);
    }
  }
}

// ─── File Context Builder ────────────────────────────────────────────────────

const DEFAULT_MAX_FILE_SIZE = 1 * 1024 * 1024; // 1MB
const DEFAULT_MAX_TOTAL_SIZE = 10 * 1024 * 1024; // 10MB
const DEFAULT_MAX_FILES = 1000;

/**
 * FileContextBuilder reads files from a storage provider, applies filters,
 * and builds a structured context string for LLM consumption.
 */
export class FileContextBuilder {
  private provider: FileStorageProvider;
  private config: FileStorageConfig;

  constructor(config: FileStorageConfig) {
    this.config = config;
    this.provider = this.createProvider(config);
  }

  private createProvider(config: FileStorageConfig): FileStorageProvider {
    switch (config.type) {
      case 'local':
        return new LocalFileStorage(config.path);
      case 's3':
        return new S3FileStorage(config);
      default:
        throw new Error(`Unknown file storage type: ${config.type}`);
    }
  }

  /**
   * Build a structured context string from all matching files.
   * Files are formatted with clear delimiters so the LLM can reference them.
   */
  async buildContext(): Promise<FileStorageResult> {
    const maxFileSize = this.config.maxFileSize ?? DEFAULT_MAX_FILE_SIZE;
    const maxTotalSize = this.config.maxTotalSize ?? DEFAULT_MAX_TOTAL_SIZE;
    const maxFiles = this.config.maxFiles ?? DEFAULT_MAX_FILES;

    // 1. List all files
    const allFiles = await this.provider.listFiles();

    // 2. Apply filters
    const matchedFiles: string[] = [];
    const skipped: Array<{ relativePath: string; reason: string }> = [];

    for (const filePath of allFiles) {
      // Extension filter
      if (this.config.extensions && this.config.extensions.length > 0) {
        const ext = path.extname(filePath).toLowerCase();
        if (!this.config.extensions.includes(ext)) {
          skipped.push({ relativePath: filePath, reason: `extension ${ext} not in allowed list` });
          continue;
        }
      }

      // Include patterns
      if (this.config.includePatterns && this.config.includePatterns.length > 0) {
        if (!matchesAnyGlob(filePath, this.config.includePatterns)) {
          skipped.push({ relativePath: filePath, reason: 'did not match include patterns' });
          continue;
        }
      }

      // Exclude patterns
      if (this.config.excludePatterns && this.config.excludePatterns.length > 0) {
        if (matchesAnyGlob(filePath, this.config.excludePatterns)) {
          skipped.push({ relativePath: filePath, reason: 'matched exclude pattern' });
          continue;
        }
      }

      matchedFiles.push(filePath);
    }

    // 3. Read files and build context
    const includedFiles: Array<{ relativePath: string; size: number }> = [];
    const contextParts: string[] = [];
    let totalSize = 0;
    let fileCount = 0;

    // Add file tree header
    const treeHeader = this.buildFileTree(matchedFiles);
    contextParts.push(treeHeader);
    totalSize += Buffer.byteLength(treeHeader, 'utf-8');

    for (const filePath of matchedFiles) {
      if (fileCount >= maxFiles) {
        skipped.push({ relativePath: filePath, reason: `max file count (${maxFiles}) reached` });
        continue;
      }

      try {
        const fileSize = await this.provider.getFileSize(filePath);

        if (fileSize > maxFileSize) {
          skipped.push({ relativePath: filePath, reason: `file size ${fileSize} exceeds max ${maxFileSize}` });
          continue;
        }

        if (totalSize + fileSize > maxTotalSize) {
          skipped.push({ relativePath: filePath, reason: `would exceed total size limit (${maxTotalSize})` });
          continue;
        }

        const content = await this.provider.readFile(filePath);
        const fileBlock = this.formatFileBlock(filePath, content);
        contextParts.push(fileBlock);

        totalSize += Buffer.byteLength(fileBlock, 'utf-8');
        fileCount++;
        includedFiles.push({ relativePath: filePath, size: fileSize });
      } catch (err: any) {
        skipped.push({ relativePath: filePath, reason: `read error: ${err.message}` });
      }
    }

    return {
      context: contextParts.join('\n'),
      files: includedFiles,
      totalSize,
      skipped,
    };
  }

  /**
   * Build a file tree overview for the LLM to understand the folder structure.
   */
  private buildFileTree(files: string[]): string {
    const lines = [
      '=== FILE STRUCTURE ===',
      `Source: ${this.config.type === 's3' ? `s3://${this.config.path}/${this.config.prefix || ''}` : this.config.path}`,
      `Total files: ${files.length}`,
      '',
      'Files:',
    ];

    for (const file of files) {
      const depth = file.split('/').length - 1;
      const indent = '  '.repeat(depth);
      const basename = path.basename(file);
      lines.push(`${indent}${depth > 0 ? '├── ' : ''}${basename} (${file})`);
    }

    lines.push('', '=== END FILE STRUCTURE ===', '');
    return lines.join('\n');
  }

  /**
   * Format a single file's content with clear delimiters.
   */
  private formatFileBlock(relativePath: string, content: string): string {
    const ext = path.extname(relativePath).slice(1);
    return [
      `=== FILE: ${relativePath} ===`,
      `\`\`\`${ext}`,
      content,
      '```',
      `=== END FILE: ${relativePath} ===`,
      '',
    ].join('\n');
  }

  /**
   * Get just the list of files without reading content.
   * Useful for previewing what would be included.
   */
  async listMatchingFiles(): Promise<string[]> {
    const allFiles = await this.provider.listFiles();

    return allFiles.filter(filePath => {
      if (this.config.extensions && this.config.extensions.length > 0) {
        const ext = path.extname(filePath).toLowerCase();
        if (!this.config.extensions.includes(ext)) return false;
      }

      if (this.config.includePatterns && this.config.includePatterns.length > 0) {
        if (!matchesAnyGlob(filePath, this.config.includePatterns)) return false;
      }

      if (this.config.excludePatterns && this.config.excludePatterns.length > 0) {
        if (matchesAnyGlob(filePath, this.config.excludePatterns)) return false;
      }

      return true;
    });
  }
}

/**
 * Convenience function to build context from a file storage config.
 */
export async function buildFileContext(config: FileStorageConfig): Promise<FileStorageResult> {
  const builder = new FileContextBuilder(config);
  return builder.buildContext();
}
