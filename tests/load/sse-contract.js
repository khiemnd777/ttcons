import { check } from 'k6';
import http from 'k6/http';
import { Rate } from 'k6/metrics';
import { classifyHTTPResponse } from './lib/backpressure.js';
import {
  endpoint,
  enumEnv,
  requestTimeout,
  requiredEnv,
  target,
} from './lib/config.js';

if (String(__ENV.CONFIRM_SSE_CONTRACT_PROBE || '') !== 'YES_ACTOR_CAP_IS_ALREADY_OCCUPIED') {
  throw new Error('SSE contract probe requires CONFIRM_SSE_CONTRACT_PROBE=YES_ACTOR_CAP_IS_ALREADY_OCCUPIED');
}

const streamKind = enumEnv('SSE_KIND', ['attention', 'project'], 'attention');
const projectID = streamKind === 'project' ? requiredUUID('PROJECT_ID') : '';
const authToken = requiredEnv('AUTH_TOKEN');
if (/^(replace[-_ ]?me|change[-_ ]?me|example)$/i.test(authToken)) {
  throw new Error('AUTH_TOKEN contains a placeholder, not a usable test credential');
}
const streamPath = streamKind === 'project'
  ? `/api/projects/${projectID}/activity/stream`
  : '/api/me/activity-attention/stream';

const contractValid = new Rate('consflow_sse_rejection_contract_valid');
const expectedRejection = http.expectedStatuses(429);

export const options = {
  discardResponseBodies: true,
  scenarios: {
    sse_rejection_contract: {
      executor: 'shared-iterations',
      vus: 1,
      iterations: 1,
      maxDuration: '30s',
      exec: 'probeSSERejectionContract',
      tags: { workload: 'sse_contract', stream_kind: streamKind },
    },
  },
  thresholds: {
    checks: ['rate==1'],
    http_req_failed: ['rate==0'],
    consflow_sse_rejection_contract_valid: ['rate==1'],
  },
};

export function setup() {
  return { targetEnvironment: target.environment, streamKind };
}

export function probeSSERejectionContract() {
  const response = http.get(endpoint(streamPath), {
    headers: {
      Authorization: `Bearer ${authToken}`,
      Accept: 'text/event-stream',
      'User-Agent': 'consflow-k6-sse-contract-probe/1.0',
    },
    timeout: requestTimeout,
    redirects: 0,
    responseType: 'text',
    responseCallback: expectedRejection,
    tags: { workload: 'sse_contract', stream_kind: streamKind },
  });
  const result = classifyHTTPResponse(response, [], 'general');
  contractValid.add(result.validLimited, { stream_kind: streamKind });
  check(response, {
    'SSE cap rejects the companion request with 429': () => result.status === 429,
    'SSE 429 includes Retry-After and RATE_LIMITED envelope': () => result.validLimited,
  }, { workload: 'sse_contract', stream_kind: streamKind });
}

function requiredUUID(name) {
  const value = requiredEnv(name);
  if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value)) {
    throw new Error(`${name} must be a UUID`);
  }
  return value;
}
