# CentralAuth

A stateless OAuth broker service for the BlackMission ecosystem. CentralAuth centralizes all OAuth flows (Discord, Steam) into a single service — client applications redirect users here, CentralAuth handles the provider interaction, and returns a short-lived exchange code. The client's backend then calls CentralAuth with their API key to retrieve the user info as plain JSON.

**No user data is stored.** All state is encoded in cryptographic tokens (HMAC-signed state tokens and AES-GCM encrypted exchange codes).

## Quick Start

### 1. Configure

Copy the example env file and fill in your secrets:

```bash
cp .env.example .env
```

### 2. Run

```bash
# Load env and run
export $(grep -v '^#' .env | xargs) && go run main.go
```

### 3. Verify

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

## Configuration

All configuration is done through environment variables. No config file is needed.

### Server

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `PORT` | No | `8080` | HTTP port |
| `HOST` | No | `0.0.0.0` | Bind address |
| `BASE_URL` | No | | Public URL of this service |

### Secrets

| Variable | Required | Description |
|----------|----------|-------------|
| `STATE_SIGNING_KEY` | Yes | HMAC-SHA256 key for state tokens |
| `EXCHANGE_ENCRYPTION_KEY` | Yes | AES-256 key (must be exactly 32 bytes) |

### Providers

Providers are enabled by the presence of their key env var.

**Discord** (enabled when `DISCORD_CLIENT_ID` is set):

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DISCORD_CLIENT_ID` | Yes | | Discord application ID |
| `DISCORD_CLIENT_SECRET` | No | | Discord application secret |
| `DISCORD_SCOPES` | No | `identify,email` | Comma-separated OAuth scopes |

**Steam** (enabled when `STEAM_API_KEY` is set):

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `STEAM_API_KEY` | Yes | | Steam Web API key |
| `STEAM_REALM` | No | `BASE_URL` value | OpenID realm |

### Clients

Clients are auto-discovered by scanning env vars for the `CLIENT_<ID>_API_KEY` pattern. The client ID is derived from the prefix: `CLIENT_WEBSITE_*` becomes `website`, `CLIENT_ADMIN_PANEL_*` becomes `admin-panel` (uppercase underscores to lowercase hyphens).

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `CLIENT_<ID>_API_KEY` | Yes | | API key for this client |
| `CLIENT_<ID>_NAME` | No | ID value | Display name |
| `CLIENT_<ID>_ALLOWED_CALLBACKS` | No | | Comma-separated callback URLs |
| `CLIENT_<ID>_ALLOWED_PROVIDERS` | No | | Comma-separated provider names |

Example:

```bash
CLIENT_WEBSITE_API_KEY=secret-key
CLIENT_WEBSITE_NAME=BlackMission Website
CLIENT_WEBSITE_ALLOWED_CALLBACKS=https://blackmission.com/auth/callback,http://localhost:3000/auth/callback
CLIENT_WEBSITE_ALLOWED_PROVIDERS=discord,steam
```

## API Reference

### `GET /health`

Health check endpoint.

**Response:** `200 OK`
```json
{"status": "ok"}
```

---

### `GET /auth/{provider}`

Initiate an OAuth flow. Redirects the user's browser to the provider's authorization page.

**Query Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `client_id` | string | Yes | Registered client application ID |
| `redirect_uri` | string | Yes | URL to redirect back to after auth (must be in allowlist) |

**Response:** `302 Found` → Provider's auth page

**Error Responses:**
| Status | Condition |
|--------|-----------|
| 400 | Missing `client_id` or `redirect_uri` |
| 400 | Unknown client or provider |
| 400 | `redirect_uri` not in allowlist |
| 403 | Provider not allowed for this client |

**Example:**
```bash
# Redirect user's browser to:
https://auth.blackmission.com/auth/discord?client_id=website&redirect_uri=https://blackmission.com/auth/callback
```

---

### `GET /callback/{provider}`

Handles the OAuth provider's callback. Validates the state token, exchanges credentials with the provider, encrypts the result into an exchange code, and redirects back to the client.

This endpoint is called by the provider (browser redirect), not directly by client applications.

**Response:** `302 Found` → `{redirect_uri}?code={exchange_code}`

**Error Responses:**
| Status | Condition |
|--------|-----------|
| 400 | Missing or invalid state token |
| 400 | State token expired (5-minute window) |
| 502 | Provider exchange or user fetch failed |

---

### `GET /exchange`

Server-to-server endpoint. Exchange an authorization code for user info. Requires API key authentication.

**Query Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `code` | string | Yes | Exchange code from the callback redirect |

**Headers:**

| Name | Value | Required |
|------|-------|----------|
| `Authorization` | `Bearer {api_key}` | Yes |

**Response:** `200 OK`
```json
{
  "user": {
    "provider": "discord",
    "provider_id": "123456789",
    "username": "tactical",
    "display_name": "Tactical Commander",
    "avatar_url": "https://cdn.discordapp.com/avatars/123456789/abc.png",
    "email": "user@example.com"
  }
}
```

**Error Responses:**
| Status | Condition |
|--------|-----------|
| 400 | Missing code, invalid code, or expired code (30-second window) |
| 401 | Missing or invalid API key |
| 403 | API key doesn't match the client that initiated the auth flow |

**Example:**
```bash
curl -H "Authorization: Bearer your-api-key" \
  "https://auth.blackmission.com/exchange?code=BASE64_EXCHANGE_CODE"
```

---

### `GET /providers`

List all registered provider names.

**Response:** `200 OK`
```json
["discord", "steam"]
```

## OAuth Flow

```
Client App                   CentralAuth                  OAuth Provider
    |                            |                              |
    | 1. GET /auth/discord       |                              |
    |    ?client_id=website      |                              |
    |    &redirect_uri=...       |                              |
    |--------------------------->|                              |
    |                            |                              |
    |   2. Validate client,      |                              |
    |      redirect_uri,         |                              |
    |      provider              |                              |
    |                            |                              |
    |   3. Generate HMAC-signed  |                              |
    |      state token           |                              |
    |                            |                              |
    | 4. 302 → Provider auth URL |                              |
    |<---------------------------|                              |
    |                            |                              |
    |  (browser at provider)     |  5. User authenticates       |
    |                            |                              |
    |                            |  6. Provider callback         |
    |                            |<-----------------------------|
    |                            |                              |
    |   7. Validate state token  |                              |
    |   8. Exchange code/tokens  |                              |
    |                            |----------------------------->|
    |   9. Fetch user profile    |<-----------------------------|
    |                            |                              |
    |  10. Encrypt result as     |                              |
    |      exchange code         |                              |
    |                            |                              |
    | 11. 302 → redirect_uri     |                              |
    |     ?code={exchange_code}  |                              |
    |<---------------------------|                              |
    |                            |                              |
    | 12. Backend: GET /exchange |                              |
    |     Authorization: Bearer  |                              |
    |     {api_key}              |                              |
    |--------------------------->|                              |
    |                            |                              |
    |  13. Decrypt, validate,    |                              |
    |      return user JSON      |                              |
    |<---------------------------|                              |
```

## Client Integration Guide

### TypeScript/Node.js (with SDK)

Install the SDK:

```bash
npm install @blackmission/centralauth-sdk
```

```typescript
import { CentralAuthClient, createCallbackHandler } from '@blackmission/centralauth-sdk';

const auth = new CentralAuthClient({
  baseURL: 'https://auth.blackmission.com',
  clientID: 'website',
  apiKey: process.env.CENTRALAUTH_API_KEY!,
});

// Redirect user to auth
app.get('/login/discord', (req, res) => {
  res.redirect(auth.getAuthorizeURL('discord', 'https://mysite.com/auth/callback'));
});

// Handle callback
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

### Manual Integration (any language)

1. **Redirect user** to `https://auth.blackmission.com/auth/{provider}?client_id={your_id}&redirect_uri={your_callback}`

2. **Handle callback** — your callback URL receives `?code={exchange_code}`

3. **Exchange code** — make a server-to-server request:
```bash
curl -H "Authorization: Bearer YOUR_API_KEY" \
  "https://auth.blackmission.com/exchange?code=THE_EXCHANGE_CODE"
```

4. **Use the user info** returned in the JSON response.

## Adding a New Provider

1. Create a new package under `internal/providers/{name}/`
2. Implement the `auth.Provider` interface:

```go
type Provider interface {
    Name() string
    AuthURL(stateToken string) (string, error)
    Exchange(ctx context.Context, params map[string]string) (*domain.AuthResult, error)
}
```

3. Register the provider in `main.go`:

```go
if cfg, ok := cfg.Providers["newprovider"]; ok {
    p := newprovider.New(newprovider.Config{...})
    providers.Register(p)
}
```

4. Add the provider name to client `CLIENT_<ID>_ALLOWED_PROVIDERS` env vars.

## Adding a New Client App

Add new env vars with the client prefix:

```bash
CLIENT_NEW_APP_API_KEY=your-api-key
CLIENT_NEW_APP_NAME=My New App
CLIENT_NEW_APP_ALLOWED_CALLBACKS=https://newapp.example.com/auth/callback,http://localhost:4000/auth/callback
CLIENT_NEW_APP_ALLOWED_PROVIDERS=discord,steam
```

The client will be auto-discovered as `new-app`.

## Security

### Design Decisions

- **Stateless architecture:** No database or session store. All context is encoded in cryptographic tokens, making the service horizontally scalable and simple to operate.
- **User data never in the browser:** Exchange codes are opaque AES-GCM ciphertext. Actual user info is only returned via the server-to-server `/exchange` endpoint.
- **Short-lived tokens:** State tokens expire in 5 minutes, exchange codes in 30 seconds.

### Cryptographic Details

- **State tokens:** HMAC-SHA256 signed, base64url-encoded JSON payload with 5-minute expiry and random nonce. Verified with constant-time comparison (`crypto/hmac.Equal`).
- **Exchange codes:** AES-256-GCM authenticated encryption. Format: `base64url(nonce || ciphertext || tag)`. 30-second embedded expiry.
- **API key validation:** Constant-time comparison via `crypto/hmac.Equal`.
- **Callback allowlist:** Exact string match only, no wildcards or pattern matching.

## Docker

### Build

```bash
docker build -t centralauth .
```

### Run

```bash
docker run -p 8080:8080 \
  -e BASE_URL=https://auth.blackmission.com \
  -e STATE_SIGNING_KEY=your-hmac-key \
  -e EXCHANGE_ENCRYPTION_KEY=your-32-byte-aes-key-here!! \
  -e DISCORD_CLIENT_ID=your-discord-id \
  -e DISCORD_CLIENT_SECRET=your-discord-secret \
  -e STEAM_API_KEY=your-steam-key \
  -e CLIENT_WEBSITE_API_KEY=your-client-key \
  -e CLIENT_WEBSITE_ALLOWED_CALLBACKS=https://example.com/cb \
  -e CLIENT_WEBSITE_ALLOWED_PROVIDERS=discord,steam \
  centralauth
```

Or with an env file:

```bash
docker run -p 8080:8080 --env-file .env centralauth
```

### Docker Compose

```yaml
services:
  centralauth:
    build: .
    ports:
      - "8080:8080"
    env_file:
      - .env
```

## Development

### Project Structure

```
CenteralAuth/
├── main.go                          # Entrypoint
├── .env.example                     # Example environment variables
├── Dockerfile                       # Multi-stage Docker build
├── internal/
│   ├── config/                      # Env var config loading
│   ├── domain/                      # Models and sentinel errors
│   ├── auth/                        # Provider interface + registry
│   ├── providers/
│   │   ├── discord/                 # Discord OAuth2
│   │   └── steam/                   # Steam OpenID 2.0
│   ├── state/                       # HMAC-signed state tokens
│   ├── exchange/                    # AES-GCM exchange codes
│   ├── client/                      # Client app registry
│   ├── handler/                     # HTTP handlers
│   └── server/                      # Router + middleware
├── pkg/testutil/                    # Shared test helpers
└── sdk/                             # TypeScript SDK
```

### Run Tests

```bash
# Go tests
go test ./...

# SDK tests
cd sdk && npm test
```

### Dependencies

- Standard library only (`crypto/*`, `net/http`, `encoding/*`)
- Zero runtime dependencies in the TypeScript SDK
