import { describe, it, expect } from 'vitest';
import type { UserInfo, CentralAuthConfig, ExchangeResponse } from '../src/types.js';

describe('Type validation', () => {
  it('UserInfo type structure is correct', () => {
    const user: UserInfo = {
      provider: 'discord',
      provider_id: '123456789',
      username: 'testuser',
      display_name: 'Test User',
      avatar_url: 'https://example.com/avatar.png',
    };

    expect(user.provider).toBe('discord');
    expect(user.provider_id).toBe('123456789');
    expect(user.email).toBeUndefined();
  });

  it('UserInfo with optional email', () => {
    const user: UserInfo = {
      provider: 'discord',
      provider_id: '123456789',
      username: 'testuser',
      display_name: 'Test User',
      avatar_url: 'https://example.com/avatar.png',
      email: 'test@example.com',
    };

    expect(user.email).toBe('test@example.com');
  });

  it('CentralAuthConfig has required fields', () => {
    const config: CentralAuthConfig = {
      baseURL: 'https://auth.example.com',
      clientID: 'website',
      apiKey: 'secret-key',
    };

    expect(config.baseURL).toBe('https://auth.example.com');
    expect(config.timeout).toBeUndefined();
  });

  it('CentralAuthConfig with optional timeout', () => {
    const config: CentralAuthConfig = {
      baseURL: 'https://auth.example.com',
      clientID: 'website',
      apiKey: 'secret-key',
      timeout: 10000,
    };

    expect(config.timeout).toBe(10000);
  });

  it('ExchangeResponse wraps UserInfo', () => {
    const response: ExchangeResponse = {
      user: {
        provider: 'steam',
        provider_id: '76561198012345678',
        username: 'gamer',
        display_name: 'Gamer',
        avatar_url: 'https://example.com/avatar.png',
      },
    };

    expect(response.user.provider).toBe('steam');
  });
});
