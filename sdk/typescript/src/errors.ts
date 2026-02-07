export class CentralAuthError extends Error {
  public statusCode?: number;

  constructor(message: string, statusCode?: number) {
    super(message);
    this.name = 'CentralAuthError';
    this.statusCode = statusCode;
  }
}

export class UnauthorizedError extends CentralAuthError {
  constructor(message = 'Invalid API key') {
    super(message, 401);
    this.name = 'UnauthorizedError';
  }
}

export class ForbiddenError extends CentralAuthError {
  constructor(message = 'Client mismatch') {
    super(message, 403);
    this.name = 'ForbiddenError';
  }
}

export class ExchangeExpiredError extends CentralAuthError {
  constructor(message = 'Exchange code expired') {
    super(message, 400);
    this.name = 'ExchangeExpiredError';
  }
}

export class ProviderError extends CentralAuthError {
  constructor(message = 'Provider exchange failed') {
    super(message, 502);
    this.name = 'ProviderError';
  }
}
