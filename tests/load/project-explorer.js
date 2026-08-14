import { check, sleep } from 'k6';
import http from 'k6/http';
import { Rate, Trend } from 'k6/metrics';
import {
  authHeaders,
  buildScenario,
  endpoint,
  profile,
  requestTimeout,
  target,
  thinkTimeSeconds,
  thresholds,
} from './lib/config.js';

const explorerLatency = new Trend('consflow_project_explorer_latency', true);
const explorerInvalidResponse = new Rate('consflow_project_explorer_invalid_response');

export const options = {
  scenarios: {
    project_explorer: buildScenario('readProjectExplorer', 'project_explorer'),
  },
  thresholds: {
    checks: [`rate>=${thresholds.minCheckRate}`],
    http_req_failed: [`rate<=${thresholds.maxErrorRate}`],
    consflow_project_explorer_invalid_response: [`rate<=${thresholds.maxErrorRate}`],
    consflow_project_explorer_latency: [`p(95)<=${thresholds.p95Ms}`],
  },
};

export function setup() {
  return { targetEnvironment: target.environment, profile };
}

export function readProjectExplorer() {
  const pageSizes = [25, 50, 100];
  const pageSize = pageSizes[(__VU + __ITER) % pageSizes.length];
  const direction = (__ITER % 2) === 0 ? 'desc' : 'asc';
  const path = `/api/projects/explorer?page=1&page_size=${pageSize}&sort=created_at&direction=${direction}`;
  const response = http.get(endpoint(path), {
    headers: authHeaders({ Accept: 'application/json' }),
    timeout: requestTimeout,
    redirects: 0,
    tags: { profile, workload: 'project_explorer', page_size: String(pageSize) },
  });
  explorerLatency.add(response.timings.duration, { page_size: String(pageSize) });

  let valid = false;
  try {
    const body = response.json();
    valid = response.status === 200
      && body?.success === true
      && Array.isArray(body?.data?.items)
      && Number.isInteger(body?.data?.total)
      && body?.data?.page_size === pageSize;
  } catch (_) {
    valid = false;
  }
  explorerInvalidResponse.add(!valid, { page_size: String(pageSize) });
  check(response, {
    'project explorer returns the paginated envelope': () => valid,
  }, { workload: 'project_explorer', page_size: String(pageSize) });
  if (thinkTimeSeconds > 0) sleep(thinkTimeSeconds);
}
