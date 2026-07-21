import {
  isStableSSEEvent,
  nextSSEFailureCount,
  reconnectDelayMs,
  parseRetryAfter,
  SSE_STABLE_CONNECTION_MS,
  streamStatusAfterFailure,
} from './backpressure';

export type ApiEnvelope<T> = {
  success: boolean;
  data: T;
  message?: string;
  error?: {
    code: string;
    message: string;
    details?: unknown;
  };
};

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080/api';
let authFailureHandler: (() => void) | null = null;
let refreshPromise: Promise<string | null> | null = null;

export type StreamStatus = 'connected' | 'reconnecting' | 'offline';

export type SSEMessage<T = any> = {
  event: string;
  data: T;
};

export class ApiError extends Error {
  code: string;
  status: number;
  retryAfterMs?: number;

  constructor(message: string, code = 'API_ERROR', status = 500, retryAfterMs?: number) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
    this.retryAfterMs = retryAfterMs;
  }
}

export function getToken() {
  return localStorage.getItem('consflow_token');
}

export function setToken(token: string | null) {
  if (token) localStorage.setItem('consflow_token', token);
  else localStorage.removeItem('consflow_token');
}

export function setAuthFailureHandler(handler: (() => void) | null) {
  authFailureHandler = handler;
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  return request<T>(path, init, true);
}

async function request<T>(path: string, init: RequestInit = {}, allowRefresh: boolean): Promise<T> {
  const token = getToken();
  const headers = new Headers(init.headers);
  const isFormData = typeof FormData !== 'undefined' && init.body instanceof FormData;
  if (!isFormData && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json');
  if (token) headers.set('Authorization', `Bearer ${token}`);

  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers,
    credentials: 'include',
  });
  const json = (await res.json().catch(() => null)) as ApiEnvelope<T> | null;

  if (!res.ok || !json?.success) {
    if (allowRefresh && shouldRefresh(path, res.status, json?.error?.code)) {
      const refreshedToken = await refreshAccessToken();
      if (refreshedToken) return request<T>(path, init, false);
    }
    throw new ApiError(
      json?.error?.message ?? 'Không gọi được API',
      json?.error?.code,
      res.status,
      parseRetryAfter(res.headers.get('Retry-After')),
    );
  }
  return json.data;
}

export function get<T>(path: string) {
  return api<T>(path);
}

export function post<T>(path: string, body?: unknown) {
  return api<T>(path, { method: 'POST', body: JSON.stringify(body ?? {}) });
}

export function postForm<T>(path: string, body: FormData) {
  return api<T>(path, { method: 'POST', body });
}

export function put<T>(path: string, body?: unknown) {
  return api<T>(path, { method: 'PUT', body: JSON.stringify(body ?? {}) });
}

export function del<T>(path: string) {
  return api<T>(path, { method: 'DELETE' });
}

export async function logoutRequest() {
  await request<{ logged_out: boolean }>('/auth/logout', { method: 'POST', body: JSON.stringify({}) }, false).catch(() => undefined);
  setToken(null);
}

async function refreshAccessToken() {
  if (!refreshPromise) {
    refreshPromise = request<{ token: string }>('/auth/refresh', { method: 'POST', body: JSON.stringify({}) }, false)
      .then((data) => {
        setToken(data.token);
        return data.token;
      })
      .catch(() => {
        setToken(null);
        authFailureHandler?.();
        return null;
      })
      .finally(() => {
        refreshPromise = null;
      });
  }
  return refreshPromise;
}

function shouldRefresh(path: string, status: number, code?: string) {
  return status === 401 && code !== 'INVALID_REFRESH_TOKEN' && !path.startsWith('/auth/login') && !path.startsWith('/auth/refresh') && !path.startsWith('/auth/logout');
}

export async function downloadBlob(path: string) {
  return downloadBlobRequest(path, true);
}

async function downloadBlobRequest(path: string, allowRefresh: boolean) {
  const token = getToken();
  const headers = new Headers();
  if (token) headers.set('Authorization', `Bearer ${token}`);
  const res = await fetch(`${API_URL}${path}`, { headers, credentials: 'include' });
  if (!res.ok) {
    if (allowRefresh && res.status === 401) {
      const refreshedToken = await refreshAccessToken();
      if (refreshedToken) return downloadBlobRequest(path, false);
    }
    const contentType = res.headers.get('Content-Type') ?? '';
    if (contentType.includes('application/json')) {
      const json = (await res.json().catch(() => null)) as ApiEnvelope<unknown> | null;
      throw new ApiError(
        json?.error?.message ?? 'Không tải được file',
        json?.error?.code,
        res.status,
        parseRetryAfter(res.headers.get('Retry-After')),
      );
    }
    const text = await res.text().catch(() => '');
    throw new ApiError(
      text || 'Không tải được file',
      'DOWNLOAD_ERROR',
      res.status,
      parseRetryAfter(res.headers.get('Retry-After')),
    );
  }
  return res.blob();
}

export function streamSSE<T = any>(
  path: string,
  handlers: {
    onEvent: (message: SSEMessage<T>) => void;
    onStatus?: (status: StreamStatus) => void;
  },
) {
  let stopped = false;
  const controller = new AbortController();
  let failedAttempts = 0;

  async function run() {
    while (!stopped) {
      let connectionWasStable = false;
      let stableTimeoutID: number | undefined;

      const markConnectionStable = () => {
        connectionWasStable = true;
        if (stableTimeoutID !== undefined) {
          window.clearTimeout(stableTimeoutID);
          stableTimeoutID = undefined;
        }
      };

      try {
        const res = await openSSE(path, controller.signal, true);
        handlers.onStatus?.('connected');
        stableTimeoutID = window.setTimeout(markConnectionStable, SSE_STABLE_CONNECTION_MS);
        await readSSEStream<T>(res.body, handlers.onEvent, markConnectionStable);
        if (!stopped) throw new ApiError('Realtime stream đã ngắt kết nối', 'STREAM_DISCONNECTED', 0);
      } catch (error) {
        if (stableTimeoutID !== undefined) window.clearTimeout(stableTimeoutID);
        if (stopped || controller.signal.aborted) break;
        failedAttempts = nextSSEFailureCount(failedAttempts, connectionWasStable);
        handlers.onStatus?.(streamStatusAfterFailure(failedAttempts));
        const retryAfterMs = error instanceof ApiError ? error.retryAfterMs : undefined;
        const delayMs = reconnectDelayMs(failedAttempts - 1, retryAfterMs);
        await wait(delayMs, controller.signal);
      }
    }
  }

  void run();
  return () => {
    stopped = true;
    controller.abort();
  };
}

async function openSSE(path: string, signal: AbortSignal, allowRefresh: boolean): Promise<Response & { body: ReadableStream<Uint8Array> }> {
  const token = getToken();
  const headers = new Headers();
  headers.set('Accept', 'text/event-stream');
  if (token) headers.set('Authorization', `Bearer ${token}`);

  const res = await fetch(`${API_URL}${path}`, { headers, signal, credentials: 'include' });
  if (allowRefresh && shouldRefresh(path, res.status)) {
    const refreshedToken = await refreshAccessToken();
    if (refreshedToken) return openSSE(path, signal, false);
  }
  if (!res.ok || !res.body) {
    throw new ApiError(
      'Không mở được realtime stream',
      'STREAM_ERROR',
      res.status,
      parseRetryAfter(res.headers.get('Retry-After')),
    );
  }
  return res as Response & { body: ReadableStream<Uint8Array> };
}

async function readSSEStream<T>(
  body: ReadableStream<Uint8Array>,
  onEvent: (message: SSEMessage<T>) => void,
  onFrame: () => void,
) {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const frames = buffer.split('\n\n');
    buffer = frames.pop() ?? '';
    frames.forEach((frame) => {
      const message = parseSSEFrame<T>(frame);
      if (message) {
        if (isStableSSEEvent(message.event)) onFrame();
        onEvent(message);
      }
    });
  }
}

function parseSSEFrame<T>(frame: string): SSEMessage<T> | null {
  let event = 'message';
  const dataLines: string[] = [];
  frame.split('\n').forEach((line) => {
    if (line.startsWith('event:')) event = line.slice(6).trim();
    if (line.startsWith('data:')) dataLines.push(line.slice(5).trim());
  });
  if (!dataLines.length) return null;
  const raw = dataLines.join('\n');
  try {
    return { event, data: JSON.parse(raw) as T };
  } catch {
    return { event, data: raw as T };
  }
}

function wait(ms: number, signal: AbortSignal) {
  return new Promise<void>((resolve) => {
    if (signal.aborted) {
      resolve();
      return;
    }
    const timeoutID = window.setTimeout(finish, ms);
    signal.addEventListener('abort', finish, { once: true });

    function finish() {
      window.clearTimeout(timeoutID);
      signal.removeEventListener('abort', finish);
      resolve();
    }
  });
}
