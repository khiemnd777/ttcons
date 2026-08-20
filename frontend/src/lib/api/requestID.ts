export function resolveRequestID(details: unknown, headerValue?: string | null) {
  const fromHeader = normalizedRequestID(headerValue);
  if (fromHeader) return fromHeader;
  if (!details || typeof details !== 'object' || Array.isArray(details)) return undefined;
  return normalizedRequestID((details as Record<string, unknown>).request_id);
}

function normalizedRequestID(value: unknown) {
  if (typeof value !== 'string') return undefined;
  const normalized = value.trim();
  if (!normalized) return undefined;
  return normalized.slice(0, 128);
}
