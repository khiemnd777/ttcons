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

export type StreamStatus = 'connected' | 'reconnecting' | 'offline';

export type SSEMessage<T = any> = {
  event: string;
  data: T;
};

export class ApiError extends Error {
  code: string;
  status: number;

  constructor(message: string, code = 'API_ERROR', status = 500) {
    super(message);
    this.code = code;
    this.status = status;
  }
}

export function getToken() {
  return localStorage.getItem('consflow_token');
}

export function setToken(token: string | null) {
  if (token) localStorage.setItem('consflow_token', token);
  else localStorage.removeItem('consflow_token');
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = getToken();
  const headers = new Headers(init.headers);
  headers.set('Content-Type', 'application/json');
  if (token) headers.set('Authorization', `Bearer ${token}`);

  const res = await fetch(`${API_URL}${path}`, {
    ...init,
    headers,
  });
  const json = (await res.json().catch(() => null)) as ApiEnvelope<T> | null;

  if (!res.ok || !json?.success) {
    throw new ApiError(json?.error?.message ?? 'Không gọi được API', json?.error?.code, res.status);
  }
  return json.data;
}

export function get<T>(path: string) {
  return api<T>(path);
}

export function post<T>(path: string, body?: unknown) {
  return api<T>(path, { method: 'POST', body: JSON.stringify(body ?? {}) });
}

export function put<T>(path: string, body?: unknown) {
  return api<T>(path, { method: 'PUT', body: JSON.stringify(body ?? {}) });
}

export function del<T>(path: string) {
  return api<T>(path, { method: 'DELETE' });
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

  async function run() {
    while (!stopped) {
      try {
        const token = getToken();
        const headers = new Headers();
        headers.set('Accept', 'text/event-stream');
        if (token) headers.set('Authorization', `Bearer ${token}`);
        const res = await fetch(`${API_URL}${path}`, { headers, signal: controller.signal });
        if (!res.ok || !res.body) throw new ApiError('Không mở được realtime stream', 'STREAM_ERROR', res.status);
        handlers.onStatus?.('connected');
        await readSSEStream<T>(res.body, handlers.onEvent);
      } catch (error) {
        if (stopped || controller.signal.aborted) break;
        handlers.onStatus?.('reconnecting');
        await wait(2000);
      }
    }
    if (!stopped) handlers.onStatus?.('offline');
  }

  void run();
  return () => {
    stopped = true;
    controller.abort();
  };
}

async function readSSEStream<T>(body: ReadableStream<Uint8Array>, onEvent: (message: SSEMessage<T>) => void) {
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
      if (message) onEvent(message);
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

function wait(ms: number) {
  return new Promise((resolve) => {
    window.setTimeout(resolve, ms);
  });
}
