import { spawn } from 'child_process';
import { Bridge, RLMConfig, RLMResult } from './bridge-interface';
import { requireGoBinary } from './binary-resolver';

function sanitizeConfig(config: RLMConfig): { config: Record<string, unknown>, structured?: any } {
  const { go_binary_path, structured, ...sanitized } = config;
  return { config: sanitized, structured };
}

export class GoBridge implements Bridge {
  public async completion(
    model: string,
    query: string,
    context: string,
    rlmConfig: RLMConfig = {}
  ): Promise<RLMResult> {
    const binaryPath = requireGoBinary(rlmConfig);

    const { config, structured } = sanitizeConfig(rlmConfig);
    const payload = JSON.stringify({
      model,
      query,
      context,
      config,
      structured
    });

    return new Promise<RLMResult>((resolve, reject) => {
      const child = spawn(binaryPath, [], { stdio: ['pipe', 'pipe', 'pipe'] });
      let stdout = '';
      let stderr = '';

      child.stdout.on('data', (data) => {
        stdout += data.toString();
      });

      child.stderr.on('data', (data) => {
        stderr += data.toString();
      });

      child.on('error', (error) => {
        reject(new Error(`Failed to start Go binary: ${error.message}`));
      });

      child.on('close', (code) => {
        if (code !== 0) {
          reject(new Error(stderr || `Go binary exited with code ${code}`));
          return;
        }

        // On success stderr only carries debug logs and warnings (e.g. trace
        // export failures); pass them through instead of discarding them.
        if (stderr) {
          process.stderr.write(stderr);
        }

        try {
          const parsed = JSON.parse(stdout) as RLMResult;
          resolve(parsed);
        } catch (error: any) {
          reject(new Error(`Failed to parse Go response: ${error.message || error}`));
        }
      });

      child.stdin.write(payload);
      child.stdin.end();
    });
  }

  public async cleanup(): Promise<void> {
    // No persistent processes to clean up.
  }
}
