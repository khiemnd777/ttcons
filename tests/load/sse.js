import sse from 'k6/x/sse';
import { check, sleep } from 'k6';
import { Counter, Rate } from 'k6/metrics';
import { validateSSELifetimeWindow } from './lib/sse-lifetime.js';
import {
  buildScenario,
  endpoint,
  enumEnv,
  profile,
  requiredEnv,
  target,
} from './lib/config.js';

const sseMode = enumEnv('SSE_MODE', ['healthy', 'capacity'], 'healthy');
if (sseMode === 'capacity' && String(__ENV.CONFIRM_SSE_CAPACITY || '') !== 'YES_I_EXPECT_CONTROLLED_REJECTIONS') {
  throw new Error('SSE_MODE=capacity requires CONFIRM_SSE_CAPACITY=YES_I_EXPECT_CONTROLLED_REJECTIONS');
}
const streamKind = enumEnv('SSE_KIND', ['attention', 'project'], 'attention');
const projectID = streamKind === 'project' ? requiredUUID('PROJECT_ID') : '';
const sseAuthToken = requiredEnv('AUTH_TOKEN');
if (/^(replace[-_ ]?me|change[-_ ]?me|example)$/i.test(sseAuthToken)) {
  throw new Error('AUTH_TOKEN contains a placeholder, not a usable test credential');
}
const streamPath = streamKind === 'project'
  ? `/api/projects/${projectID}/activity/stream`
  : '/api/me/activity-attention/stream';

const connectionsOpened = new Counter('consflow_sse_connections_opened');
const eventsReceived = new Counter('consflow_sse_events_received');
const openError = new Rate('consflow_sse_open_error');
const rateLimited = new Rate('consflow_sse_rate_limited');

const maxErrorRate = boundedRate('SSE_MAX_ERROR_RATE', 0.02);
const minRateLimitedRate = boundedRate('SSE_MIN_429_RATE', sseMode === 'capacity' ? 0.05 : 0);
const maxRateLimitedRate = boundedRate('SSE_MAX_429_RATE', sseMode === 'capacity' ? 1 : 0.01);
const rejectionBackoffSeconds = boundedSeconds('SSE_REJECTION_BACKOFF_SECONDS', 30, 1, 300);
if (minRateLimitedRate > maxRateLimitedRate) {
  throw new Error('SSE_MIN_429_RATE must not exceed SSE_MAX_429_RATE');
}
const scenarioWorkload = sseMode === 'capacity' ? 'sse_capacity' : 'sse_healthy';
const sseScenario = buildScenario('runSSE', scenarioWorkload);
validateServerLifetimeForScenario(sseScenario);

export const options = {
  scenarios: {
    [`${profile}_sse_${sseMode}`]: sseScenario,
  },
  thresholds: {
    checks: ['rate>=0.98'],
    consflow_sse_connections_opened: ['count>0'],
    consflow_sse_events_received: ['count>0'],
    consflow_sse_open_error: [`rate<=${maxErrorRate}`],
    consflow_sse_rate_limited: [
      `rate>=${minRateLimitedRate}`,
      `rate<=${maxRateLimitedRate}`,
    ],
  },
};

export function setup() {
  return {
    targetEnvironment: target.environment,
    workload: 'sse',
    profile,
    streamKind,
    sseMode,
  };
}

export function runSSE() {
  let opened = false;
  let sawError = false;

  const response = sse.open(endpoint(streamPath), {
    method: 'GET',
    headers: {
      Authorization: `Bearer ${sseAuthToken}`,
      Accept: 'text/event-stream',
      'User-Agent': 'consflow-k6-ddos-hardening/1.0',
    },
    tags: { profile, workload: 'sse', stream_kind: streamKind, sse_mode: sseMode },
  }, (client) => {
    client.on('open', () => {
      opened = true;
      connectionsOpened.add(1, metricTags());
      openError.add(false, metricTags());
      rateLimited.add(false, metricTags());
      check(true, {
        'SSE stream opened': (value) => value,
      }, checkTags());
    });
    client.on('event', (event) => {
      eventsReceived.add(1, {
        profile,
        stream_kind: streamKind,
        sse_mode: sseMode,
        event_name: String(event.name || 'message'),
      });
    });
    client.on('error', () => {
      sawError = true;
      openError.add(true, metricTags());
      check(false, {
        'SSE stream remains healthy': (value) => value,
      }, checkTags());
      client.close();
    });
  });

  const status = Number(response && response.status) || 0;
  if (!opened) {
    const limited = status === 429;
    const retryAfter = responseHeader(response, 'Retry-After');
    // xk6-sse v0.1.12 exposes URL/status/headers/error but no response body.
    // tests/load/sse-contract.js validates the JSON RATE_LIMITED envelope separately.
    const headerQualifiedRejection = limited && Boolean(retryAfter);
    rateLimited.add(headerQualifiedRejection, metricTags());
    openError.add(!headerQualifiedRejection, metricTags());
    check(response, {
      'SSE rejection has 429 and Retry-After': () => headerQualifiedRejection,
      'SSE response is not a server error': () => status > 0 && status < 500,
    }, checkTags());
    const retryAfterSeconds = numericRetryAfterSeconds(retryAfter);
    sleep(Math.max(rejectionBackoffSeconds, retryAfterSeconds));
  } else if (sawError) {
    check(false, {
      'SSE completed without an unexpected stream error': (value) => value,
    }, checkTags());
  }
}

function metricTags() {
  return { profile, stream_kind: streamKind, sse_mode: sseMode };
}

function checkTags() {
  return { profile, workload: 'sse', stream_kind: streamKind, sse_mode: sseMode };
}

function responseHeader(response, name) {
  const headers = response && response.headers ? response.headers : {};
  const key = Object.keys(headers).find((candidate) => candidate.toLowerCase() === name.toLowerCase());
  return key ? String(headers[key] || '').trim() : '';
}

function numericRetryAfterSeconds(value) {
  const seconds = Number(value);
  return Number.isFinite(seconds) && seconds > 0 ? Math.min(seconds, 300) : 0;
}

function boundedRate(name, fallback) {
  const raw = String(__ENV[name] ?? fallback).trim();
  const value = Number(raw);
  if (!Number.isFinite(value) || value < 0 || value > 1) {
    throw new Error(`${name} must be between 0 and 1`);
  }
  return value;
}

function boundedSeconds(name, fallback, min, max) {
  const raw = String(__ENV[name] ?? fallback).trim();
  const value = Number(raw);
  if (!Number.isFinite(value) || value < min || value > max) {
    throw new Error(`${name} must be between ${min} and ${max}`);
  }
  return value;
}

function validateServerLifetimeForScenario(scenario) {
  const rawLifetime = String(__ENV.SSE_SERVER_MAX_LIFETIME_SECONDS || '').trim();
  if (!rawLifetime && profile !== 'soak') return;
  if (!rawLifetime) {
    throw new Error('SSE soak requires SSE_SERVER_MAX_LIFETIME_SECONDS after staging lifetime is raised for the test window');
  }
  const serverLifetimeSeconds = boundedSeconds('SSE_SERVER_MAX_LIFETIME_SECONDS', rawLifetime, 60, 86400);
  const minJitterRatio = boundedNumber('SSE_LIFETIME_MIN_RATIO', 0.8, 0.5, 1);
  const marginSeconds = boundedSeconds('SSE_TURNOVER_MARGIN_SECONDS', 120, 30, 1800);
  validateSSELifetimeWindow(scenario, serverLifetimeSeconds, minJitterRatio, marginSeconds);
}

function boundedNumber(name, fallback, min, max) {
  const raw = String(__ENV[name] ?? fallback).trim();
  const value = Number(raw);
  if (!Number.isFinite(value) || value < min || value > max) {
    throw new Error(`${name} must be between ${min} and ${max}`);
  }
  return value;
}

function requiredUUID(name) {
  const value = requiredEnv(name);
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value)) {
    throw new Error(`${name} must be a UUID`);
  }
  return value;
}
