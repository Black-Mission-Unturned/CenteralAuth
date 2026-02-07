import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { CentralAuthClient } from '../src/client.js';
import { createCallbackHandler } from '../src/middleware/express.js';

const mockFetch = vi.fn();

describe('Express Middleware', () => {
  let client: CentralAuthClient;

  beforeEach(() => {
    vi.stubGlobal('fetch', mockFetch);
    client = new CentralAuthClient({
      baseURL: 'https://auth.example.com',
      clientID: 'website',
      apiKey: 'test-api-key',
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('calls onSuccess with user on successful exchange', async () => {
    const mockUser = {
      provider: 'discord',
      provider_id: '123',
      username: 'testuser',
      display_name: 'Test User',
      avatar_url: 'https://example.com/avatar.png',
    };

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ user: mockUser }),
    });

    const onSuccess = vi.fn();
    const onError = vi.fn();

    const handler = createCallbackHandler(client, { onSuccess, onError });

    const req = { query: { code: 'test-code' } };
    const res = {};

    await handler(req, res);

    expect(onSuccess).toHaveBeenCalledWith(mockUser, req, res);
    expect(onError).not.toHaveBeenCalled();
  });

  it('calls onError when exchange fails', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 401,
      statusText: 'Unauthorized',
      json: () => Promise.resolve({ error: 'invalid API key' }),
    });

    const onSuccess = vi.fn();
    const onError = vi.fn();

    const handler = createCallbackHandler(client, { onSuccess, onError });

    const req = { query: { code: 'bad-code' } };
    const res = {};

    await handler(req, res);

    expect(onError).toHaveBeenCalled();
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it('calls onError when code is missing', async () => {
    const onSuccess = vi.fn();
    const onError = vi.fn();

    const handler = createCallbackHandler(client, { onSuccess, onError });

    const req = { query: {} };
    const res = {};

    await handler(req, res);

    expect(onError).toHaveBeenCalled();
    expect(onSuccess).not.toHaveBeenCalled();
  });
});
