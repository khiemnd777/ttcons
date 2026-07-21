export const SSE_RECONNECT_BASE_DELAY_MS = 2_000;
export const SSE_RECONNECT_MAX_DELAY_MS = 60_000;
export const SSE_OFFLINE_AFTER_ATTEMPTS = 5;
export const SSE_STABLE_CONNECTION_MS = 30_000;

const SSE_RECONNECT_JITTER_RATIO = 0.2;

export function parseRetryAfter(value: string | null, nowMs = Date.now()): number | undefined {
  if (!value) return undefined;

  const seconds = Number(value);
  if (Number.isFinite(seconds) && seconds >= 0) return Math.ceil(seconds * 1_000);

  const retryAt = Date.parse(value);
  if (Number.isNaN(retryAt)) return undefined;
  return Math.max(0, retryAt - nowMs);
}

export function reconnectDelayMs(
  failedAttempt: number,
  retryAfterMs?: number,
  random: () => number = Math.random,
): number {
  const attempt = Math.max(0, Math.floor(failedAttempt));
  const exponentialDelay = Math.min(
    SSE_RECONNECT_MAX_DELAY_MS,
    SSE_RECONNECT_BASE_DELAY_MS * (2 ** attempt),
  );
  const jitterMultiplier = 1 - SSE_RECONNECT_JITTER_RATIO + (random() * SSE_RECONNECT_JITTER_RATIO * 2);
  const jitteredDelay = Math.round(exponentialDelay * jitterMultiplier);
  const serverDelay = retryAfterMs === undefined ? 0 : Math.max(0, retryAfterMs);
  return Math.min(SSE_RECONNECT_MAX_DELAY_MS, Math.max(jitteredDelay, serverDelay));
}

export function nextSSEFailureCount(previousFailures: number, connectionWasStable: boolean): number {
  return connectionWasStable ? 1 : Math.max(0, Math.floor(previousFailures)) + 1;
}

export function streamStatusAfterFailure(failedAttempts: number): 'reconnecting' | 'offline' {
  return failedAttempts >= SSE_OFFLINE_AFTER_ATTEMPTS ? 'offline' : 'reconnecting';
}

export function isStableSSEEvent(event: string): boolean {
  return event !== 'connected';
}

type HttpErrorLike = {
  status?: unknown;
  name?: unknown;
};

export function shouldRetryQuery(failureCount: number, error: unknown, maxRetries = 1): boolean {
  if (failureCount >= maxRetries) return false;
  if (error instanceof TypeError) return true;
  if (!error || typeof error !== 'object') return false;

  const candidate = error as HttpErrorLike;
  if (candidate.name === 'AbortError') return false;
  return typeof candidate.status === 'number' && candidate.status >= 500 && candidate.status <= 599;
}
