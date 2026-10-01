import { Bridge, RLMConfig } from './bridge-interface';
import { requireGoBinary } from './binary-resolver';

export type BridgeType = 'go';

/**
 * Create the Go bridge for RLM communication.
 * Throws an RLMBinaryError listing every location checked if the Go binary
 * cannot be found.
 */
export async function createBridge(
  bridgeType: BridgeType = 'go',
  config: Pick<RLMConfig, 'go_binary_path'> = {}
): Promise<Bridge> {
  requireGoBinary(config);

  const { GoBridge } = await import('./go-bridge');
  return new GoBridge();
}
