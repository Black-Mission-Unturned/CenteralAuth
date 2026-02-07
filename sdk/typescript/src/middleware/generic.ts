import type { CentralAuthClient } from '../client.js';
import type { UserInfo } from '../types.js';

/**
 * Framework-agnostic callback handler. Pass the exchange code and get back user info.
 *
 * Usage:
 * ```
 * const handler = createGenericHandler(auth);
 * const user = await handler(code);
 * ```
 */
export function createGenericHandler(
  client: CentralAuthClient
): (code: string) => Promise<UserInfo> {
  return (code: string) => client.exchange(code);
}
