import { check, sleep } from 'k6';
import http from 'k6/http';
import { Rate, Trend } from 'k6/metrics';
import { classifyHTTPResponse } from './lib/backpressure.js';
import {
  authHeaders,
  authToken,
  buildScenario,
  endpoint,
  paths,
  profile,
  requestTimeout,
  target,
  thinkTimeSeconds,
  thresholds,
  upload,
  workload,
} from './lib/config.js';

const requestLatency = new Trend('consflow_request_latency', true);
const unexpectedError = new Rate('consflow_unexpected_error');
const rateLimited = new Rate('consflow_rate_limited');
const controlledBusy = new Rate('consflow_controlled_busy_503');
const readExpectedStatuses = http.expectedStatuses(200, 429, 503);
const uploadExpectedStatuses = http.expectedStatuses(201, 429, 503);

export const options = {
  discardResponseBodies: true,
  scenarios: {
    [`${profile}_${workload}`]: buildScenario('runWorkload'),
  },
  thresholds: {
    checks: [`rate>=${thresholds.minCheckRate}`],
    http_req_failed: [`rate<=${thresholds.maxErrorRate}`],
    consflow_unexpected_error: [`rate<=${thresholds.maxErrorRate}`],
    consflow_rate_limited: [`rate<=${thresholds.maxRateLimitedRate}`],
    consflow_controlled_busy_503: [`rate<=${thresholds.maxBusyRate}`],
    consflow_request_latency: [`p(95)<=${thresholds.p95Ms}`],
  },
};

export function setup() {
  return {
    targetEnvironment: target.environment,
    workload,
    profile,
  };
}

export function runWorkload() {
  let response;
  if (workload === 'upload') {
    response = uploadEvidence();
  } else {
    const selectedPath = paths[workload];
    const headers = workload === 'health' ? commonHeaders() : authHeaders();
    response = http.get(endpoint(selectedPath), requestParams(headers, readExpectedStatuses));
  }

  recordResponse(response, workload === 'upload' ? [201] : [200]);
  if (thinkTimeSeconds > 0) sleep(thinkTimeSeconds);
}

function uploadEvidence() {
  const payload = minimalPDF(upload.bytes);
  const iterationSuffix = `${__VU}-${__ITER}`;
  const form = {
    file: http.file(payload, `k6-${upload.runID}-${iterationSuffix}.pdf`, 'application/pdf'),
    evidence_type: 'document',
    visibility: upload.visibility,
    name: `K6 load test ${upload.runID} ${iterationSuffix}`,
    caption: `Synthetic non-production load-test artifact ${upload.runID}`,
  };
  return http.post(
    endpoint(`/api/work-items/${upload.workItemID}/evidence/upload`),
    form,
    requestParams(authHeaders(), uploadExpectedStatuses),
  );
}

function requestParams(headers, responseCallback) {
  return {
    headers,
    timeout: requestTimeout,
    redirects: 0,
    // Error envelopes are needed to distinguish controlled backpressure from arbitrary 5xx.
    // The body is parsed only for 429/503 and released with the iteration.
    responseType: 'text',
    tags: { profile, workload },
    responseCallback,
  };
}

function commonHeaders() {
  return { 'User-Agent': 'consflow-k6-ddos-hardening/1.0' };
}

function recordResponse(response, successStatuses) {
  const result = classifyHTTPResponse(response, successStatuses, workload);

  rateLimited.add(result.limited, { profile, workload });
  controlledBusy.add(result.validBusy, { profile, workload });
  unexpectedError.add(result.unexpected, { profile, workload, status: String(result.status) });
  if (response && response.timings) {
    requestLatency.add(response.timings.duration, { profile, workload });
  }
  check(response, {
    'status is successful or controlled backpressure': () => result.success || result.validLimited || result.validBusy,
    '429/503 includes Retry-After and a valid error envelope': () => (!result.limited && !result.busy) || result.validLimited || result.validBusy,
    'all other server errors are rejected': () => result.status > 0 && (result.status < 500 || result.validBusy),
  }, { profile, workload });
}

function minimalPDF(bytes) {
  const prefix = '%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n';
  const suffix = '\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n';
  const paddingLength = Math.max(0, bytes - prefix.length - suffix.length);
  return `${prefix}% ${'x'.repeat(paddingLength)}${suffix}`;
}

// Referencing authToken at init makes missing credentials fail before any VU starts.
void authToken;
