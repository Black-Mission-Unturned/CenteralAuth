import type { CentralAuthConfig, UserInfo, ExchangeResponse, HealthResponse } from './types.js';
import {
  CentralAuthError,
  UnauthorizedError,
  ForbiddenError,
  ExchangeExpiredError,
  ProviderError,
} from './errors.js';

const DEFAULT_TIMEOUT = 5000;

export class CentralAuthClient {
  private readonly baseURL: string;
  private readonly clientID: string;
  private readonly apiKey: string;
  private readonly timeout: number;

  constructor(config: CentralAuthConfig) {
    this.baseURL = config.baseURL.replace(/\/+$/, '');
    this.clientID = config.clientID;
    this.apiKey = config.apiKey;
    this.timeout = config.timeout ?? DEFAULT_TIMEOUT;
  }

  /**
   * Generate an authorization URL for redirecting the user to CentralAuth.
   */
  getAuthorizeURL(provider: string, redirectURI: string): string {
    const params = new URLSearchParams({
      client_id: this.clientID,
      redirect_uri: redirectURI,
    });
    return `${this.baseURL}/auth/${encodeURIComponent(provider)}?${params.toString()}`;
  }

  /**
   * Exchange an authorization code for user info (server-to-server).
   */
  async exchange(code: string): Promise<UserInfo> {
    const params = new URLSearchParams({ code });
    const url = `${this.baseURL}/exchange?${params.toString()}`;

    const response = await this.fetch(url, {
      headers: {
        Authorization: `Bearer ${this.apiKey}`,
      },
    });

    if (!response.ok) {
      await this.handleErrorResponse(response);
    }

    const data = (await response.json()) as ExchangeResponse;
    return data.user;
  }

  /**
   * List available provider names.
   */
  async getProviders(): Promise<string[]> {
    const response = await this.fetch(`${this.baseURL}/providers`);

    if (!response.ok) {
      throw new CentralAuthError(`Failed to fetch providers: ${response.status}`, response.status);
    }

    return (await response.json()) as string[];
  }

  /**
   * Check if the CentralAuth server is healthy.
   */
  async healthCheck(): Promise<boolean> {
    try {
      const response = await this.fetch(`${this.baseURL}/health`);
      if (!response.ok) return false;
      const data = (await response.json()) as HealthResponse;
      return data.status === 'ok';
    } catch {
      return false;
    }
  }

  private async fetch(url: string, init?: RequestInit): Promise<Response> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      return await fetch(url, {
        ...init,
        signal: controller.signal,
      });
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') {
        throw new CentralAuthError(`Request timed out after ${this.timeout}ms`);
      }
      throw new CentralAuthError(
        `Network error: ${error instanceof Error ? error.message : 'unknown error'}`
      );
    } finally {
      clearTimeout(timeoutId);
    }
  }

  private async handleErrorResponse(response: Response): Promise<never> {
    let message: string;
    try {
      const body = (await response.json()) as { error?: string };
      message = body.error ?? response.statusText;
    } catch {
      message = response.statusText;
    }

    switch (response.status) {
      case 400:
        if (message.toLowerCase().includes('expired')) {
          throw new ExchangeExpiredError(message);
        }
        throw new CentralAuthError(message, 400);
      case 401:
        throw new UnauthorizedError(message);
      case 403:
        throw new ForbiddenError(message);
      case 502:
        throw new ProviderError(message);
      default:
        throw new CentralAuthError(message, response.status);
    }
  }
}
