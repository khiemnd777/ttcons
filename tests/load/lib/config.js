const PROFILE_NAMES = ['baseline', 'step', 'spike', 'soak'];
const HTTP_WORKLOADS = ['health', 'general', 'dashboard', 'report', 'upload'];

const rawTarget = requiredEnv('TARGET_BASE_URL');
export const target = validateTarget(rawTarget);
export const profile = enumEnv('PROFILE', PROFILE_NAMES, 'baseline');
export const workload = enumEnv('WORKLOAD', HTTP_WORKLOADS, 'health');
export const authToken = workload === 'health' ? '' : requiredSecret('AUTH_TOKEN');

export const requestTimeout = durationEnv('REQUEST_TIMEOUT', '15s', '2m');
export const thinkTimeSeconds = floatEnv('THINK_TIME_SECONDS', 0.25, 0, 60);
export const paths = Object.freeze({
  health: pathEnv('HEALTH_PATH', '/api/health'),
  general: pathEnv('GENERAL_PATH', '/api/auth/me'),
  dashboard: pathEnv('DASHBOARD_PATH', '/api/reports/dashboard'),
  report: pathEnv('REPORT_PATH', '/api/reports/projects?page_size=25'),
});

export const upload = workload === 'upload' ? Object.freeze({
  workItemID: uuidEnv('UPLOAD_WORK_ITEM_ID'),
  runID: slugEnv('UPLOAD_RUN_ID'),
  bytes: intEnv('UPLOAD_BYTES', 512, 128, 16 * 1024),
  visibility: enumEnv('UPLOAD_VISIBILITY', ['internal', 'project'], 'internal'),
}) : null;

const defaultThresholds = thresholdDefaults(profile, workload);
export const thresholds = Object.freeze({
  p95Ms: floatEnv('P95_MS', defaultThresholds.p95Ms, 1, 120000),
  maxErrorRate: floatEnv('MAX_ERROR_RATE', defaultThresholds.maxErrorRate, 0, 1),
  maxRateLimitedRate: floatEnv('MAX_429_RATE', defaultThresholds.maxRateLimitedRate, 0, 1),
  maxBusyRate: floatEnv('MAX_BUSY_503_RATE', defaultThresholds.maxBusyRate, 0, 1),
  minCheckRate: floatEnv('MIN_CHECK_RATE', 0.99, 0, 1),
});

export function buildScenario(execName, selectedWorkload = workload) {
  const defaults = scenarioDefaults(profile, selectedWorkload);
  let scenario;

  if (profile === 'baseline') {
    scenario = {
      executor: 'constant-vus',
      vus: intEnv('VUS', defaults.vus, 1, 500),
      duration: durationEnv('DURATION', defaults.duration, '2h'),
    };
  } else if (profile === 'soak') {
    scenario = {
      executor: 'constant-vus',
      vus: intEnv('VUS', defaults.vus, 1, 500),
      duration: durationEnv('DURATION', defaults.duration, '4h'),
    };
  } else {
    const startVUs = intEnv('START_VUS', defaults.startVUs, 0, 500);
    scenario = {
      executor: 'ramping-vus',
      startVUs,
      stages: defaults.stages.map((stage, index) => ({
        duration: durationEnv(`STAGE_${index + 1}_DURATION`, stage.duration, '2h'),
        target: intEnv(`STAGE_${index + 1}_VUS`, stage.target, 0, 500),
      })),
      gracefulRampDown: '0s',
    };
  }

  const maxVUs = scenario.executor === 'constant-vus'
    ? scenario.vus
    : Math.max(scenario.startVUs, ...scenario.stages.map((stage) => stage.target));
  validateLoadLevel(selectedWorkload, maxVUs);

  return {
    ...scenario,
    exec: execName,
    gracefulStop: '0s',
    tags: { profile, workload: selectedWorkload },
  };
}

export function endpoint(path) {
  return `${target.baseUrl}${path}`;
}

export function authHeaders(extra = {}) {
  return {
    Authorization: `Bearer ${authToken}`,
    'User-Agent': 'consflow-k6-ddos-hardening/1.0',
    ...extra,
  };
}

export function requiredEnv(name) {
  const value = String(__ENV[name] || '').trim();
  if (!value) throw new Error(`${name} is required`);
  return value;
}

export function enumEnv(name, values, fallback) {
  const value = String(__ENV[name] || fallback).trim().toLowerCase();
  if (!values.includes(value)) {
    throw new Error(`${name} must be one of: ${values.join(', ')}`);
  }
  return value;
}

export function intEnv(name, fallback, min, max) {
  const raw = String(__ENV[name] ?? fallback).trim();
  if (!/^-?\d+$/.test(raw)) throw new Error(`${name} must be an integer`);
  const value = Number(raw);
  if (!Number.isSafeInteger(value) || value < min || value > max) {
    throw new Error(`${name} must be between ${min} and ${max}`);
  }
  return value;
}

export function floatEnv(name, fallback, min, max) {
  const raw = String(__ENV[name] ?? fallback).trim();
  const value = Number(raw);
  if (!Number.isFinite(value) || value < min || value > max) {
    throw new Error(`${name} must be between ${min} and ${max}`);
  }
  return value;
}

export function durationEnv(name, fallback, maxDuration) {
  const value = String(__ENV[name] || fallback).trim();
  const milliseconds = durationMilliseconds(value);
  if (milliseconds <= 0 || milliseconds > durationMilliseconds(maxDuration)) {
    throw new Error(`${name} must be greater than 0 and no longer than ${maxDuration}`);
  }
  return value;
}

function validateTarget(value) {
  const parsed = parseOrigin(value);
  const hostname = parsed.hostname;
  const loopback = hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '::1';
  const privateIPv4 = /^10\./.test(hostname)
    || /^192\.168\./.test(hostname)
    || /^172\.(1[6-9]|2\d|3[01])\./.test(hostname);
  const explicitNonProductionName = /(^|[.-])(staging|stage|qa|dev|test|sandbox|perf|load)([.-]|$)/.test(hostname)
    || hostname.endsWith('.local')
    || hostname.endsWith('.test');
  const productionMarker = /(^|[.-])(prod|production|live|www)([.-]|$)/.test(hostname);
  const targetEnvironment = String(__ENV.TARGET_ENV || (loopback ? 'local' : '')).trim().toLowerCase();

  if (productionMarker) {
    throw new Error('Production-like TARGET_BASE_URL values are always refused');
  }
  if (loopback) {
    if (targetEnvironment !== 'local') throw new Error('Loopback targets require TARGET_ENV=local');
  } else {
    if (targetEnvironment !== 'staging') throw new Error('Non-loopback targets require TARGET_ENV=staging');
    if (String(__ENV.CONFIRM_NON_PRODUCTION || '') !== 'YES_I_CONFIRM_NON_PRODUCTION') {
      throw new Error('Staging targets require CONFIRM_NON_PRODUCTION=YES_I_CONFIRM_NON_PRODUCTION');
    }
    if (!privateIPv4 && !explicitNonProductionName) {
      throw new Error('Target hostname is not explicitly identifiable as staging/dev/qa/test/perf');
    }
    if (parsed.protocol !== 'https:') throw new Error('Non-loopback targets require HTTPS');
  }

  return Object.freeze({
    baseUrl: parsed.baseUrl,
    environment: targetEnvironment,
    hostname,
  });
}

function parseOrigin(value) {
  const match = /^(https?):\/\/([^/?#]+)\/?$/i.exec(value);
  if (!match) {
    throw new Error('TARGET_BASE_URL must be an absolute HTTP(S) origin without path, query, or fragment');
  }
  const protocol = `${match[1].toLowerCase()}:`;
  const authority = match[2];
  if (authority.includes('@')) throw new Error('TARGET_BASE_URL must not contain credentials');

  let hostname;
  let port = '';
  if (authority.startsWith('[')) {
    const ipv6Match = /^\[([^\]]+)\](?::(\d+))?$/.exec(authority);
    if (!ipv6Match) throw new Error('TARGET_BASE_URL contains an invalid IPv6 authority');
    hostname = ipv6Match[1].toLowerCase();
    port = ipv6Match[2] || '';
  } else {
    if ((authority.match(/:/g) || []).length > 1) {
      throw new Error('IPv6 TARGET_BASE_URL values must use brackets');
    }
    const separator = authority.lastIndexOf(':');
    hostname = (separator >= 0 ? authority.slice(0, separator) : authority).toLowerCase();
    port = separator >= 0 ? authority.slice(separator + 1) : '';
  }
  if (!hostname || /\s/.test(hostname)) throw new Error('TARGET_BASE_URL contains an invalid hostname');
  if (port && (!/^\d+$/.test(port) || Number(port) < 1 || Number(port) > 65535)) {
    throw new Error('TARGET_BASE_URL contains an invalid port');
  }
  if (!authority.startsWith('[') && authority.endsWith(':')) {
    throw new Error('TARGET_BASE_URL contains an invalid empty port');
  }

  return {
    protocol,
    hostname,
    baseUrl: `${match[1].toLowerCase()}://${authority}`,
  };
}

function pathEnv(name, fallback) {
  const value = String(__ENV[name] || fallback).trim();
  if (!value.startsWith('/api/') || value.startsWith('//') || value.includes('://') || value.includes('..') || value.includes('\\') || value.includes('#')) {
    throw new Error(`${name} must be a relative /api/... path without traversal or fragments`);
  }
  return value;
}

function requiredSecret(name) {
  const value = requiredEnv(name);
  if (/^(replace[-_ ]?me|change[-_ ]?me|example)$/i.test(value)) {
    throw new Error(`${name} contains a placeholder, not a usable test credential`);
  }
  return value;
}

function uuidEnv(name) {
  const value = requiredEnv(name);
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value)) {
    throw new Error(`${name} must be a UUID`);
  }
  return value;
}

function slugEnv(name) {
  const value = requiredEnv(name);
  if (!/^[a-zA-Z0-9][a-zA-Z0-9._-]{2,63}$/.test(value)) {
    throw new Error(`${name} must be a 3-64 character non-secret run identifier`);
  }
  return value;
}

function durationMilliseconds(value) {
  const match = /^(\d+(?:\.\d+)?)(ms|s|m|h)$/.exec(value);
  if (!match) throw new Error(`Invalid k6 duration: ${value}`);
  const multipliers = { ms: 1, s: 1000, m: 60000, h: 3600000 };
  return Number(match[1]) * multipliers[match[2]];
}

function validateLoadLevel(selectedWorkload, maxVUs) {
  let hardCap = 500;
  if (selectedWorkload === 'upload') hardCap = 10;
  if (selectedWorkload === 'sse_healthy') hardCap = 2;
  if (selectedWorkload === 'sse_capacity') hardCap = intEnv('SSE_CAPACITY_VU_CAP', 50, 3, 500);
  if (maxVUs > hardCap) {
    throw new Error(`${selectedWorkload} workload is capped at ${hardCap} VUs by the safety guard`);
  }
  if (maxVUs > 50 && String(__ENV.CONFIRM_HIGH_LOAD || '') !== 'YES_I_UNDERSTAND_HIGH_LOAD') {
    throw new Error('More than 50 VUs requires CONFIRM_HIGH_LOAD=YES_I_UNDERSTAND_HIGH_LOAD');
  }
}

function scenarioDefaults(selectedProfile, selectedWorkload) {
  const uploadDefaults = {
    baseline: { vus: 1, duration: '1m' },
    step: { startVUs: 0, stages: [{ duration: '1m', target: 1 }, { duration: '2m', target: 3 }, { duration: '2m', target: 5 }, { duration: '1m', target: 0 }] },
    spike: { startVUs: 0, stages: [{ duration: '20s', target: 1 }, { duration: '10s', target: 5 }, { duration: '1m', target: 5 }, { duration: '30s', target: 0 }] },
    soak: { vus: 2, duration: '15m' },
  };
  const sseHealthyDefaults = {
    baseline: { vus: 2, duration: '2m' },
    step: { startVUs: 0, stages: [{ duration: '30s', target: 1 }, { duration: '1m', target: 2 }, { duration: '1m', target: 2 }, { duration: '30s', target: 0 }] },
    spike: { startVUs: 0, stages: [{ duration: '15s', target: 1 }, { duration: '5s', target: 2 }, { duration: '1m', target: 2 }, { duration: '20s', target: 0 }] },
    soak: { vus: 2, duration: '30m' },
  };
  const sseCapacityDefaults = {
    baseline: { vus: 5, duration: '2m' },
    step: { startVUs: 0, stages: [{ duration: '30s', target: 2 }, { duration: '1m', target: 5 }, { duration: '2m', target: 10 }, { duration: '30s', target: 0 }] },
    spike: { startVUs: 0, stages: [{ duration: '15s', target: 2 }, { duration: '5s', target: 20 }, { duration: '1m', target: 20 }, { duration: '20s', target: 0 }] },
    soak: { vus: 5, duration: '30m' },
  };
  const httpDefaults = {
    baseline: { vus: 2, duration: '2m' },
    step: { startVUs: 0, stages: [{ duration: '1m', target: 5 }, { duration: '2m', target: 15 }, { duration: '2m', target: 30 }, { duration: '1m', target: 0 }] },
    spike: { startVUs: 0, stages: [{ duration: '20s', target: 5 }, { duration: '10s', target: 50 }, { duration: '1m', target: 50 }, { duration: '30s', target: 0 }] },
    soak: { vus: 10, duration: '30m' },
  };

  if (selectedWorkload === 'upload') return uploadDefaults[selectedProfile];
  if (selectedWorkload === 'sse_healthy') return sseHealthyDefaults[selectedProfile];
  if (selectedWorkload === 'sse_capacity') return sseCapacityDefaults[selectedProfile];
  return httpDefaults[selectedProfile];
}

function thresholdDefaults(selectedProfile, selectedWorkload) {
  const latencyByWorkload = {
    health: 500,
    general: 750,
    dashboard: 1500,
    report: 1800,
    upload: 2500,
  };
  const profileMultiplier = { baseline: 1, step: 1.5, spike: 2.5, soak: 1.5 };
  const rateLimits = {
    baseline: { error: 0.01, limited: 0.01 },
    step: { error: 0.02, limited: 0.25 },
    spike: { error: 0.03, limited: 0.80 },
    soak: { error: 0.01, limited: 0.10 },
  };
  return {
    p95Ms: latencyByWorkload[selectedWorkload] * profileMultiplier[selectedProfile],
    maxErrorRate: rateLimits[selectedProfile].error,
    maxRateLimitedRate: rateLimits[selectedProfile].limited,
    maxBusyRate: rateLimits[selectedProfile].limited,
  };
}
