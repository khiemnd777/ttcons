# [Closed] Event Logs for user-triggered backend 5xx

Status: IMPLEMENTED; FEATURE-SCOPED TECHNICALLY VERIFIED; FULL DB-BACKED REPOSITORY SUITE HAS OUT-OF-SCOPE FAILURES; NOT UX ACCEPTED (approved and closed by user on 2026-08-20)

## Will be done

- Add a dedicated PostgreSQL `event_logs` store and reserved `event_log.view` permission for Admin and Developer only, separate from business `audit_logs`.
- Capture backend unexpected 5xx, panics and controlled 503 responses with the existing Caddy/Fiber request ID, actor context, safe route template and bounded redacted diagnostics; exclude 4xx/429, probes and Caddy-only upstream failures.
- Replace raw unexpected-error responses with a generic message plus request ID while preserving safe controlled errors and structured stdout fallback when persistence fails.
- Add permission-protected paginated list/detail APIs, 90-day bounded retention and multi-replica-safe cleanup.
- Add the `/event-logs` admin page with filters, summary, responsive list/detail states and request-ID-aware global error toasts; relabel the existing `/audit` navigation entry as `Audit nghiệp vụ`.
- Update migration seed parity, focused backend/frontend tests, runtime/admin/frontend/root-cause maps and domain memory.
- Validate Go formatting/tests, migration behavior, frontend tests/build, Compose/Caddy contracts and current-tree whitespace. Browser/UAT remains skipped unless explicitly requested.

## Done

- Reviewed the current repository and confirmed that request IDs, central Fiber error handling, Prometheus metrics, business audit persistence and Audit UI patterns exist, while durable technical Event Logs do not.
- Review package, three data/access decisions, Caddy boundary and Mockup as Text approved by the user.
- Mandatory module-memory, Backend, Frontend, Database, Impact Analysis and Admin Governance instructions loaded; owner code, V110 RBAC, Caddy request-ID flow and current error response behavior inspected.
- Added V111 `event_logs`, indexes and reserved `event_log.view`; migration, repository replacement guard and seed grant access only to Admin/Developer and reject other roles. V111 applied locally and its DB trigger rejected a PM assignment probe.
- Added final-status Fiber capture for direct backend 5xx, returned errors and panics; categories distinguish unexpected 5xx, panic and controlled 503. Metrics, health/readiness and 4xx/429 are excluded, while Caddy-only gateway responses remain outside the backend process.
- Added bounded redaction, deterministic fingerprints, route/request/known actor-project context, best-effort PostgreSQL persistence, sanitized structured stdout fallback and advisory-lock-protected 90-day retention. No request body, query string, auth header/cookie or raw IP is stored.
- Generic backend 500 responses now hide internal error text and include the authoritative request ID without discarding existing structured error details. Fetch, upload, blob and SSE `ApiError` paths preserve `X-Request-ID`; shared 5xx error toasts expose it as the user trace code.
- Added protected paginated list/detail APIs with server search/date/category/status/method/route/actor/project filters, category summary and source-backed filter options. Local API probes returned 200 for Admin, 403 for PM, and list/detail contract parity for a temporary sanitized fixture that was removed afterward.
- Added the lazy `/event-logs` page with approved dense summary/filter/table/mobile-card/detail-dialog states, request-ID copy action and security-boundary copy. Existing `/audit` navigation is now labelled `Audit nghiệp vụ`.
- Updated central/backend/frontend/root-cause/Admin Governance maps to make Event Logs, business Audit and Caddy gateway diagnostics separate durable concepts.
- Current Event Logs scope verification on 2026-08-20: `gofmt` clean; non-DB `go test ./...` passed; DB-backed `shared/eventlog` redaction/persistence/retention test passed; focused capture/error/core/server tests passed; all 111 migrations passed checksum validation; V111 applied with 110 skipped and backend restarted with 111/111 recorded; local `/api/ready` returned 200; frontend 212/212 tests passed; current production build passed; Event Logs chunk was 5.1 KiB gzip and the bundle budget passed; `git diff --check` passed.

## Skipped / remaining acceptance

- The full DB-backed `go test ./... -count=1` is not green even on a disposable clean database: three existing core workflow/Rev8/V110-seal tests fail, and two migration-runner tests fail under package-parallel DB execution. None of those failures touch Event Logs owners; focused Event Logs packages and DB behavior pass. This repository-wide criterion remains unresolved and prevents a global green-suite claim.
- Browser/UAT has not run because the user did not explicitly request browser verification; UI/UX remains `NOT UX ACCEPTED`.
