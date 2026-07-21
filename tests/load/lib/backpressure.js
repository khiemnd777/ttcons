const BACKEND_BUSY_WORKLOADS = ['dashboard', 'report', 'upload'];

export function classifyHTTPResponse(response, successStatuses, workload) {
  const status = Number(response && response.status) || 0;
  const success = successStatuses.includes(status);
  const retryAfter = responseHeader(response, 'Retry-After');
  const envelopeCode = status === 429 || status === 503 ? errorEnvelopeCode(response) : '';
  const limited = status === 429;
  const validLimited = limited && Boolean(retryAfter) && envelopeCode === 'RATE_LIMITED';
  const busy = status === 503;
  const validBusyCode = envelopeCode === 'SERVICE_UNAVAILABLE'
    || (BACKEND_BUSY_WORKLOADS.includes(workload) && envelopeCode === 'SERVER_BUSY');
  const validBusy = busy && Boolean(retryAfter) && validBusyCode;

  return {
    status,
    success,
    limited,
    validLimited,
    busy,
    validBusy,
    unexpected: !success && !validLimited && !validBusy,
  };
}

export function responseHeader(response, name) {
  const headers = response && response.headers ? response.headers : {};
  const key = Object.keys(headers).find((candidate) => candidate.toLowerCase() === name.toLowerCase());
  return key ? String(headers[key] || '').trim() : '';
}

function errorEnvelopeCode(response) {
  if (!response || typeof response.body !== 'string' || !response.body.trim()) return '';
  try {
    const envelope = JSON.parse(response.body);
    if (envelope && envelope.success === false && envelope.error && typeof envelope.error.code === 'string') {
      return envelope.error.code.trim();
    }
  } catch (_) {
    return '';
  }
  return '';
}
