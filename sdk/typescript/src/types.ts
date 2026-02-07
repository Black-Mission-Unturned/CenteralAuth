export interface CentralAuthConfig {
  /** CentralAuth server URL (e.g., "https://auth.blackmission.com") */
  baseURL: string;
  /** Registered client ID */
  clientID: string;
  /** API key for /exchange calls */
  apiKey: string;
  /** Request timeout in ms (default: 5000) */
  timeout?: number;
}

export interface UserInfo {
  provider: string;
  provider_id: string;
  username: string;
  display_name: string;
  avatar_url: string;
  email?: string;
}

export interface ExchangeResponse {
  user: UserInfo;
}

export interface HealthResponse {
  status: string;
}
