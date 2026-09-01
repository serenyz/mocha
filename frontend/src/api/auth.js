const API_BASE = import.meta.env.VITE_API_BASE || '';
const AUTH_STORAGE_KEY = 'mocha.auth';
let refreshOperation = null;

function newAuthSessionInstanceID() {
  return globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

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
  const headers = new Headers(fetchOptions.headers || {});
  if (fetchOptions.body !== undefined && fetchOptions.body !== null && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json');
  }
  let response;

  try {
    response = await fetch(`${API_BASE}${path}`, {
      ...fetchOptions,
      headers,
    });
  } catch (error) {
    if (error?.name === 'AbortError') throw error;
    throw new AuthApiError('暂时无法连接服务，请确认后端已启动', 'NETWORK_ERROR');
  }

  const payload = await response.json().catch(() => null);

  if (!response.ok || payload?.success !== true) {
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
    auth_session_instance_id: session.auth_session_instance_id || newAuthSessionInstanceID(),
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
    const parsed = JSON.parse(value);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('invalid auth session');
    return { ...parsed, remember };
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
  refreshOperation = null;
}

function withBearer(options, accessToken, tokenType = 'Bearer') {
  const headers = new Headers(options.headers || {});
  headers.set('Authorization', `${tokenType || 'Bearer'} ${accessToken}`);
  return {
    ...options,
    headers,
  };
}

function isTerminalAuthenticationError(error) {
  return error?.code === 'INVALID_REFRESH_TOKEN' || error?.code === 'ACCOUNT_DISABLED';
}

function isSameAuthSession(left, right) {
  if (!left || !right) return false;
  if (left.auth_session_instance_id || right.auth_session_instance_id) {
    return Boolean(left.auth_session_instance_id) &&
      left.auth_session_instance_id === right.auth_session_instance_id;
  }
  return Boolean(left.refresh_token) && left.refresh_token === right.refresh_token;
}

function clearAuthSessionIfCurrent(session) {
  const current = readAuthSession();
  if (!current || !session) return;
  if (current.refresh_token === session.refresh_token || current.access_token === session.access_token) {
    clearAuthSession();
  }
}

async function refreshAuthSession(session) {
  const refreshToken = session?.refresh_token;
  if (!refreshToken) {
    throw new AuthApiError('登录状态已失效，请重新登录', 'INVALID_REFRESH_TOKEN', 401);
  }

  if (refreshOperation?.refreshToken === refreshToken) return refreshOperation.promise;

  let promise;
  promise = (async () => {
    try {
      const refreshed = await request('/api/v1/auth/refresh', {
        method: 'POST',
        body: JSON.stringify({ refresh_token: refreshToken }),
      });
      if (!refreshed?.access_token || !refreshed?.refresh_token) {
        throw new AuthApiError('服务端返回了无效的登录凭据', 'INVALID_REFRESH_RESPONSE');
      }

      const current = readAuthSession();
      if (current?.refresh_token !== refreshToken) {
        if (current?.access_token && isSameAuthSession(current, session)) return current;
        if (current?.access_token) {
          throw new AuthApiError('登录账号已切换，请重新操作', 'AUTH_SESSION_CHANGED');
        }
        throw new AuthApiError('登录状态已失效，请重新登录', 'UNAUTHENTICATED', 401);
      }

      return persistAuthSession({
        ...session,
        ...refreshed,
        user: session.user,
      }, session.remember);
    } catch (error) {
      const current = readAuthSession();
      if (current?.refresh_token !== refreshToken && current?.access_token) {
        if (isSameAuthSession(current, session)) return current;
        throw new AuthApiError('登录账号已切换，请重新操作', 'AUTH_SESSION_CHANGED');
      }
      throw error;
    } finally {
      if (refreshOperation?.promise === promise) refreshOperation = null;
    }
  })();

  refreshOperation = { refreshToken, promise };
  return promise;
}

async function requestWithSession(path, options, session) {
  return request(path, withBearer(options, session.access_token, session.token_type));
}

function handleTerminalAuthenticationError(error, session) {
  if (isTerminalAuthenticationError(error)) clearAuthSessionIfCurrent(session);
  return error;
}

async function retryWithFreshSession(path, options, failedSession, originalError) {
  let session = readAuthSession();
  if (!session?.access_token) {
    clearAuthSessionIfCurrent(failedSession);
    throw originalError;
  }

  if (session.access_token !== failedSession.access_token) {
    if (!isSameAuthSession(session, failedSession)) {
      throw new AuthApiError('登录账号已切换，请重新操作', 'AUTH_SESSION_CHANGED');
    }
    try {
      return await requestWithSession(path, options, session);
    } catch (error) {
      handleTerminalAuthenticationError(error, session);
      if (error.code !== 'UNAUTHENTICATED') throw error;
    }
  }

  if (!session.refresh_token) {
    clearAuthSessionIfCurrent(session);
    throw originalError;
  }

  let refreshed;
  try {
    refreshed = await refreshAuthSession(session);
  } catch (error) {
    handleTerminalAuthenticationError(error, session);
    throw error;
  }

  try {
    return await requestWithSession(path, options, refreshed);
  } catch (error) {
    if (error.code === 'UNAUTHENTICATED') clearAuthSessionIfCurrent(refreshed);
    handleTerminalAuthenticationError(error, refreshed);
    throw error;
  }
}

export async function authenticatedRequest(path, options = {}) {
  const session = readAuthSession();
  if (!session?.access_token) {
    throw new AuthApiError('登录状态已失效，请重新登录', 'UNAUTHENTICATED', 401);
  }

  try {
    return await requestWithSession(path, options, session);
  } catch (error) {
    handleTerminalAuthenticationError(error, session);
    if (error.code !== 'UNAUTHENTICATED') throw error;
    return retryWithFreshSession(path, options, session, error);
  }
}

export async function logoutAccount() {
  const session = readAuthSession();
  try {
    await authenticatedRequest('/api/v1/auth/logout', { method: 'POST' });
  } finally {
    if (isSameAuthSession(readAuthSession(), session)) clearAuthSession();
  }
}
