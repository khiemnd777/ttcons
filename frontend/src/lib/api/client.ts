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
