const API_BASE = import.meta.env.VITE_API_BASE || '';
const AUTH_STORAGE_KEY = 'mocha.auth';

export class AuthApiError extends Error {
  constructor(message, code = 'REQUEST_FAILED', status = 0) {
    super(message);
    this.name = 'AuthApiError';
    this.code = code;
    this.status = status;
  }
}

async function request(path, options = {}) {
  const { returnEnvelope = false, ...fetchOptions } = options;
  let response;

  try {
    response = await fetch(`${API_BASE}${path}`, {
      ...fetchOptions,
      headers: {
        'Content-Type': 'application/json',
        ...fetchOptions.headers,
      },
    });
  } catch {
    throw new AuthApiError('暂时无法连接服务，请确认后端已启动', 'NETWORK_ERROR');
  }

  const payload = await response.json().catch(() => null);

  if (!response.ok || !payload || payload.success === false) {
    throw new AuthApiError(
      (payload && payload.error && payload.error.message) ||
        `请求失败（${response.status}）`,
      (payload && payload.error && payload.error.code) || 'REQUEST_FAILED',
      response.status,
    );
  }

  return returnEnvelope ? payload : payload.data ?? null;
}

export function sendRegisterCode(phone) {
  return request('/api/v1/auth/register-code', {
    method: 'POST',
    body: JSON.stringify({ phone }),
  });
}

export function registerAccount({ phone, password, nickname, code }) {
  return request('/api/v1/auth/register', {
    method: 'POST',
    body: JSON.stringify({ phone, password, nickname, code }),
  });
}

export function loginAccount({ phone, password }) {
  return request('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ phone, password }),
  });
}

export function persistAuthSession(session, remember) {
  const target = remember ? window.localStorage : window.sessionStorage;
  const stale = remember ? window.sessionStorage : window.localStorage;
  const storedSession = {
    ...session,
    remember: Boolean(remember),
    saved_at: Date.now(),
  };

  stale.removeItem(AUTH_STORAGE_KEY);
  target.setItem(AUTH_STORAGE_KEY, JSON.stringify(storedSession));
  return storedSession;
}

function readFromStorage(storage, remember) {
  const value = storage.getItem(AUTH_STORAGE_KEY);
  if (!value) return null;

  try {
    return { ...JSON.parse(value), remember };
  } catch {
    storage.removeItem(AUTH_STORAGE_KEY);
    return null;
  }
}

export function readAuthSession() {
  return readFromStorage(window.sessionStorage, false) ||
    readFromStorage(window.localStorage, true);
}

export function clearAuthSession() {
  window.sessionStorage.removeItem(AUTH_STORAGE_KEY);
  window.localStorage.removeItem(AUTH_STORAGE_KEY);
}

function withBearer(options, accessToken, tokenType = 'Bearer') {
  return {
    ...options,
    headers: {
      ...options.headers,
      Authorization: `${tokenType || 'Bearer'} ${accessToken}`,
    },
  };
}

async function refreshAuthSession(session) {
  const refreshed = await request('/api/v1/auth/refresh', {
    method: 'POST',
    body: JSON.stringify({ refresh_token: session.refresh_token }),
  });
  return persistAuthSession({
    ...session,
    ...refreshed,
    user: session.user,
  }, session.remember);
}

export async function authenticatedRequest(path, options = {}) {
  const session = readAuthSession();
  if (!session?.access_token) {
    throw new AuthApiError('登录状态已失效，请重新登录', 'UNAUTHENTICATED', 401);
  }

  try {
    return await request(path, withBearer(options, session.access_token, session.token_type));
  } catch (error) {
    if (error.status !== 401 || !session.refresh_token) throw error;

    try {
      const refreshed = await refreshAuthSession(session);
      return await request(path, withBearer(options, refreshed.access_token, refreshed.token_type));
    } catch (refreshError) {
      clearAuthSession();
      throw refreshError;
    }
  }
}

export async function logoutAccount() {
  try {
    await authenticatedRequest('/api/v1/auth/logout', { method: 'POST' });
  } finally {
    clearAuthSession();
  }
}
