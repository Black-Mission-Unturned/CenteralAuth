# @blackmission/centralauth-sdk

TypeScript SDK for the BlackMission CentralAuth OAuth broker service.

## Installation

```bash
npm install @blackmission/centralauth-sdk
```

**Requirements:** Node.js 18+ (uses native `fetch`)

## Quick Start

```typescript
import { CentralAuthClient } from '@blackmission/centralauth-sdk';

const auth = new CentralAuthClient({
  baseURL: 'https://auth.blackmission.com',
  clientID: 'website',
  apiKey: process.env.CENTRALAUTH_API_KEY!,
});

// 1. Generate authorize URL (redirect user's browser here)
const url = auth.getAuthorizeURL('discord', 'https://mysite.com/auth/callback');

// 2. Exchange code for user info (server-to-server)
const user = await auth.exchange(code);
// → { provider, provider_id, username, display_name, avatar_url, email? }

// 3. List available providers
const providers = await auth.getProviders();
// → ["discord", "steam"]

// 4. Health check
const healthy = await auth.healthCheck();
// → true
```

## Express Middleware

```typescript
import { CentralAuthClient, createCallbackHandler } from '@blackmission/centralauth-sdk';

const auth = new CentralAuthClient({ ... });

app.get('/auth/callback', createCallbackHandler(auth, {
  onSuccess: (user, req, res) => {
    req.session.user = user;
    res.redirect('/');
  },
  onError: (error, req, res) => {
    res.status(401).send('Authentication failed');
  },
}));
```

## Error Handling

```typescript
import {
  ExchangeExpiredError,
  UnauthorizedError,
  ForbiddenError,
  ProviderError,
  CentralAuthError,
} from '@blackmission/centralauth-sdk';

try {
  const user = await auth.exchange(code);
} catch (err) {
  if (err instanceof ExchangeExpiredError) {
    // Code expired (30-second window)
  } else if (err instanceof UnauthorizedError) {
    // Invalid API key
  } else if (err instanceof ForbiddenError) {
    // API key doesn't match the client that initiated the flow
  } else if (err instanceof ProviderError) {
    // Provider (Discord/Steam) returned an error
  } else if (err instanceof CentralAuthError) {
    // Network error, timeout, or other failure
  }
}
```

## API Reference

### `new CentralAuthClient(config)`

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `baseURL` | `string` | Yes | CentralAuth server URL |
| `clientID` | `string` | Yes | Registered client ID |
| `apiKey` | `string` | Yes | API key for `/exchange` calls |
| `timeout` | `number` | No | Request timeout in ms (default: 5000) |

### `client.getAuthorizeURL(provider, redirectURI)`

Returns the full URL to redirect the user's browser to for authentication. No network call.

### `client.exchange(code)`

Exchanges an authorization code for user info. Returns a `UserInfo` object.

### `client.getProviders()`

Returns an array of available provider names (e.g., `["discord", "steam"]`).

### `client.healthCheck()`

Returns `true` if the server is healthy, `false` otherwise. Never throws.

### `createCallbackHandler(client, options)`

Creates an Express-compatible route handler for the OAuth callback.

### `createGenericHandler(client)`

Creates a framework-agnostic handler function `(code: string) => Promise<UserInfo>`.
