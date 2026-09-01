import { AuthApiError, authenticatedRequest } from './auth';

const SOCKET_PATH = '/api/v1/ws';
const MAX_EVENT_SIZE = 65_536;

function socketURL(ticket) {
  if (typeof ticket !== 'string' || !ticket) {
    throw new AuthApiError('服务端没有返回有效的 WebSocket Ticket', 'INVALID_WEBSOCKET_TICKET');
  }
  const configured = import.meta.env.VITE_WS_URL;
  const apiBase = import.meta.env.VITE_API_BASE;
  let url;

  if (configured) {
    url = new URL(configured, window.location.href);
  } else if (apiBase) {
    url = new URL(`${apiBase.replace(/\/$/, '')}${SOCKET_PATH}`, window.location.href);
    url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  } else {
    url = new URL(SOCKET_PATH, window.location.href);
    url.protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  }

  if (url.protocol === 'http:') url.protocol = 'ws:';
  if (url.protocol === 'https:') url.protocol = 'wss:';
  if (url.protocol !== 'ws:' && url.protocol !== 'wss:') {
    throw new AuthApiError('WebSocket 地址不正确', 'INVALID_WEBSOCKET_URL');
  }

  url.search = '';
  url.searchParams.set('ticket', ticket);
  return url.toString();
}

export async function requestWebSocketTicket() {
  const result = await authenticatedRequest('/api/v1/ws/tickets', { method: 'POST' });
  if (typeof result?.ticket !== 'string' || !result.ticket) {
    throw new AuthApiError('服务端没有返回有效的 WebSocket Ticket', 'INVALID_WEBSOCKET_TICKET');
  }
  return result;
}

export class ChatSocket {
  constructor({ onEvent, onState, onAuthenticationFailure } = {}) {
    this.onEvent = onEvent;
    this.onState = onState;
    this.onAuthenticationFailure = onAuthenticationFailure;
    this.socket = null;
    this.authenticated = false;
    this.stopped = true;
    this.reconnectAttempts = 0;
    this.reconnectTimer = null;
    this.authenticationTimer = null;
    this.connectionVersion = 0;
  }

  start() {
    if (!this.stopped) return;
    this.stopped = false;
    this.reconnectAttempts = 0;
    this.connect();
  }

  restart() {
    this.stop();
    this.start();
  }

  stop() {
    this.stopped = true;
    this.authenticated = false;
    window.clearTimeout(this.reconnectTimer);
    window.clearTimeout(this.authenticationTimer);
    this.reconnectTimer = null;
    this.authenticationTimer = null;
    this.connectionVersion += 1;
    const socket = this.socket;
    this.socket = null;
    if (socket && socket.readyState < WebSocket.CLOSING) socket.close(1000, 'client closed');
    this.emitState('offline');
  }

  send(type, data = {}) {
    if (!this.authenticated || this.socket?.readyState !== WebSocket.OPEN) return false;
    if (typeof type !== 'string' || !type.trim()) return false;
    try {
      const payload = JSON.stringify({ type, data });
      if (new TextEncoder().encode(payload).byteLength > MAX_EVENT_SIZE) return false;
      this.socket.send(payload);
      return true;
    } catch {
      return false;
    }
  }

  emitState(status, detail = {}) {
    this.onState?.({ status, ...detail });
  }

  async connect() {
    if (this.stopped) return;
    window.clearTimeout(this.reconnectTimer);
    this.reconnectTimer = null;
    if (this.socket && this.socket.readyState < WebSocket.CLOSING) return;
    const version = ++this.connectionVersion;
    this.authenticated = false;
    this.emitState(this.reconnectAttempts ? 'reconnecting' : 'connecting', {
      attempt: this.reconnectAttempts,
    });

    let ticket;
    try {
      ({ ticket } = await requestWebSocketTicket());
    } catch (error) {
      if (version !== this.connectionVersion || this.stopped) return;
      if (error.status === 401 || error.code === 'INVALID_REFRESH_TOKEN' || error.code === 'ACCOUNT_DISABLED') {
        this.stopped = true;
        this.emitState('offline', { error });
        this.onAuthenticationFailure?.(error);
        return;
      }
      this.scheduleReconnect(error);
      return;
    }

    if (version !== this.connectionVersion || this.stopped) return;

    let socket;
    try {
      socket = new WebSocket(socketURL(ticket));
    } catch (error) {
      this.scheduleReconnect(error);
      return;
    }
    this.socket = socket;

    this.authenticationTimer = window.setTimeout(() => {
      if (this.socket === socket && !this.authenticated) socket.close();
    }, 10_000);

    socket.addEventListener('message', (event) => {
      if (this.socket !== socket || this.stopped) return;
      let payload;
      try {
        payload = JSON.parse(event.data);
      } catch {
        return;
      }

      if (payload?.type === 'auth.ok') {
        window.clearTimeout(this.authenticationTimer);
        this.authenticationTimer = null;
        window.clearTimeout(this.reconnectTimer);
        this.reconnectTimer = null;
        this.authenticated = true;
        this.reconnectAttempts = 0;
        this.emitState('online');
        return;
      }
      if (this.authenticated) this.onEvent?.(payload);
    });

    socket.addEventListener('close', (event) => {
      if (this.socket !== socket) return;
      window.clearTimeout(this.authenticationTimer);
      this.authenticationTimer = null;
      this.socket = null;
      this.authenticated = false;
      if (this.stopped) return;
      if (event.code === 4003) {
        this.stopped = true;
        this.connectionVersion += 1;
        this.emitState('replaced', { code: event.code, reason: event.reason });
        return;
      }
      this.scheduleReconnect(null, event.code);
    });

    socket.addEventListener('error', () => {
      if (this.socket === socket && socket.readyState === WebSocket.OPEN) socket.close();
    });
  }

  scheduleReconnect(error, code) {
    if (this.stopped) return;
    const attempt = this.reconnectAttempts + 1;
    this.reconnectAttempts = attempt;
    const delay = Math.min(15_000, 750 * (2 ** Math.min(attempt - 1, 5)));
    this.emitState('reconnecting', { attempt, delay, error, code });
    window.clearTimeout(this.reconnectTimer);
    this.reconnectTimer = window.setTimeout(() => this.connect(), delay);
  }
}
