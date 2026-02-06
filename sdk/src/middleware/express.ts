import type { CentralAuthClient } from '../client.js';
import type { UserInfo } from '../types.js';

export interface CallbackHandlerOptions {
  onSuccess: (user: UserInfo, req: any, res: any) => void | Promise<void>;
  onError: (error: Error, req: any, res: any) => void | Promise<void>;
}

/**
 * Creates an Express-compatible callback handler for CentralAuth.
 *
 * Usage:
 * ```
 * app.get('/auth/callback', createCallbackHandler(auth, {
 *   onSuccess: (user, req, res) => { req.session.user = user; res.redirect('/'); },
 *   onError: (error, req, res) => { res.status(401).send('Auth failed'); },
 * }));
 * ```
 */
export function createCallbackHandler(
  client: CentralAuthClient,
  options: CallbackHandlerOptions
): (req: any, res: any) => Promise<void> {
  return async (req, res) => {
    const code = req.query?.code;
    if (!code) {
      await options.onError(new Error('Missing code parameter'), req, res);
      return;
    }

    try {
      const user = await client.exchange(code as string);
      await options.onSuccess(user, req, res);
    } catch (error) {
      await options.onError(error instanceof Error ? error : new Error(String(error)), req, res);
    }
  };
}
