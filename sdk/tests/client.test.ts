import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { CentralAuthClient } from '../src/client.js';
import {
  CentralAuthError,
  UnauthorizedError,
  ForbiddenError,
  ExchangeExpiredError,
} from '../src/errors.js';

const mockFetch = vi.fn();

describe('CentralAuthClient', () => {
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

  describe('getAuthorizeURL', () => {
    it('generates correct URL with client_id and redirect_uri', () => {
      const url = client.getAuthorizeURL('discord', 'https://mysite.com/auth/callback');

      expect(url).toBe(
        'https://auth.example.com/auth/discord?client_id=website&redirect_uri=https%3A%2F%2Fmysite.com%2Fauth%2Fcallback'
      );
    });

    it('strips trailing slashes from baseURL', () => {
      const c = new CentralAuthClient({
        baseURL: 'https://auth.example.com/',
        clientID: 'website',
        apiKey: 'key',
      });
      const url = c.getAuthorizeURL('steam', 'https://mysite.com/cb');
      expect(url).toContain('https://auth.example.com/auth/steam');
    });
  });

  describe('exchange', () => {
    it('returns UserInfo on successful exchange', async () => {
      const mockUser = {
        provider: 'discord',
        provider_id: '123456789',
        username: 'tactical',
        display_name: 'Tactical Commander',
        avatar_url: 'https://cdn.example.com/avatar.png',
        email: 'tactical@example.com',
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ user: mockUser }),
      });

      const user = await client.exchange('exchange-code');

      expect(user).toEqual(mockUser);
      expect(mockFetch).toHaveBeenCalledWith(
        'https://auth.example.com/exchange?code=exchange-code',
        expect.objectContaining({
          headers: { Authorization: 'Bearer test-api-key' },
        })
      );
    });

    it('throws ExchangeExpiredError for expired code', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 400,
        statusText: 'Bad Request',
        json: () => Promise.resolve({ error: 'exchange code expired' }),
      });

      await expect(client.exchange('expired-code')).rejects.toThrow(ExchangeExpiredError);
    });

    it('throws UnauthorizedError for invalid API key', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 401,
        statusText: 'Unauthorized',
        json: () => Promise.resolve({ error: 'invalid API key' }),
      });

      await expect(client.exchange('code')).rejects.toThrow(UnauthorizedError);
    });

    it('throws ForbiddenError for client mismatch', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 403,
        statusText: 'Forbidden',
        json: () => Promise.resolve({ error: 'API key does not match client' }),
      });

      await expect(client.exchange('code')).rejects.toThrow(ForbiddenError);
    });

    it('throws CentralAuthError on network error', async () => {
      mockFetch.mockRejectedValueOnce(new TypeError('Failed to fetch'));

      await expect(client.exchange('code')).rejects.toThrow(CentralAuthError);
    });

    it('throws CentralAuthError on timeout', async () => {
      const slowClient = new CentralAuthClient({
        baseURL: 'https://auth.example.com',
        clientID: 'website',
        apiKey: 'key',
        timeout: 10,
      });

      mockFetch.mockImplementationOnce((_url: string, init: RequestInit) => {
        return new Promise((_, reject) => {
          init.signal?.addEventListener('abort', () => {
            reject(new DOMException('The operation was aborted', 'AbortError'));
          });
        });
      });

      await expect(slowClient.exchange('code')).rejects.toThrow(CentralAuthError);
    });
  });

  describe('getProviders', () => {
    it('returns provider name array', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(['discord', 'steam']),
      });

      const providers = await client.getProviders();
      expect(providers).toEqual(['discord', 'steam']);
    });
  });

  describe('healthCheck', () => {
    it('returns true when healthy', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ status: 'ok' }),
      });

      const healthy = await client.healthCheck();
      expect(healthy).toBe(true);
    });

    it('returns false when unhealthy', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        status: 500,
      });

      const healthy = await client.healthCheck();
      expect(healthy).toBe(false);
    });

    it('returns false on network error', async () => {
      mockFetch.mockRejectedValueOnce(new TypeError('Network error'));

      const healthy = await client.healthCheck();
      expect(healthy).toBe(false);
    });
  });
});
