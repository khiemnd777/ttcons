# Project Explorer Pagination And Redis Cache

## Scope

`GET /api/projects/explorer` is the server-paginated source for `ProjectsPage`. The legacy `GET /api/projects` contract remains available to bounded existing consumers. The explorer accepts `page`, `page_size` (`25`, `50`, `100`), `search`, `status`, `sort` (`project`, `status`, `owner`, `created_at`) and `direction` (`asc`, `desc`).

Redis is an optional, disposable acceleration layer. PostgreSQL remains authoritative for records, revisions and access scope. `AuthSessionActive` and permission decisions are never cached.

## Cache Model

- Index key: catalog revision + actor ID + actor revision + canonical query hash.
- Row key: project ID + project revision.
- Every cached index page is re-authorized with the current `ProjectAccessFilter` before Redis rows are read.
- Database triggers bump catalog, actor or project revisions in the same transaction as source writes and enqueue an invalidation event in `project_explorer_invalidation_outbox`.
- The dispatcher publishes invalidation-only SSE after commit, retries failed Redis Pub/Sub delivery with backoff and deletes published outbox rows after seven days.
- Redis timeout, corrupt or oversized values, lock contention, circuit-open and scope mismatch all fall back to the scoped PostgreSQL query.

## Rollout

1. Apply migrations before enabling cache mode. V97 creates revisions/outbox, V98 fixes first-write actor version bootstrap, V99 adds query indexes and V100 indexes published-outbox retention cleanup.
2. Configure a dedicated `REDIS_CACHE_URL` (or `REDIS_CACHE_URL_FILE`) that is separate from runtime `REDIS_URL`.
3. Start with `PROJECT_EXPLORER_CACHE_MODE=off` to verify the paginated DB path.
4. Change to `shadow`; alert on any increase in `consflow_project_explorer_cache_shadow_mismatch_total` and investigate before serving cache results.
5. Change to `serve` only after shadow mismatch remains zero under representative mutations and permission changes.

Mode changes require a backend restart. Redis data does not need migration or backup and may be flushed by replacing only the dedicated cache service.

## Observability

- `consflow_project_explorer_cache_requests_total{layer,outcome}`: index/row hit and miss counts.
- `consflow_project_explorer_cache_operation_duration_seconds{operation}`: bounded Redis operation latency.
- `consflow_project_explorer_cache_fallback_total{reason}`: database fallback reason.
- `consflow_project_explorer_cache_shadow_mismatch_total`: semantic mismatch while shadowing.
- Backend structured logs: `project_explorer_cache_shadow_mismatch`, invalidation dispatch failure and invalidation cleanup failure.

Watch PostgreSQL query latency and the count/age of unpublished outbox rows alongside Redis metrics. A growing unpublished queue means SSE freshness is degraded; query correctness remains protected by version reads and the 60-second frontend polling fallback.

## Non-Production Load Check

Obtain an authenticated token for a non-production actor with representative project access, then run:

```bash
TARGET_BASE_URL=http://127.0.0.1:8080 \
TARGET_ENV=local \
WORKLOAD=project_explorer \
PROFILE=baseline \
AUTH_TOKEN='<token>' \
k6 run tests/load/project-explorer.js
```

The script refuses production-like targets, rotates page sizes `25/50/100`, exercises both creation-date directions and validates the paginated response envelope. Its default baseline target is p95 <= 750 ms with <= 1% request/contract errors; override thresholds only when the acceptance target is documented.

## Rollback

1. Set `PROJECT_EXPLORER_CACHE_MODE=off` and restart the backend. The endpoint immediately uses PostgreSQL only.
2. Keep V97-V100 applied. They are additive and safe while cache mode is off; do not edit or roll back applied migration files.
3. If needed, stop or replace only the dedicated `redis-cache` service. Do not stop runtime Redis, which serves rate limiting and SSE fanout.
4. Keep frontend pagination enabled unless the endpoint itself is being rolled back. Cache rollback and pagination rollback are independent.
