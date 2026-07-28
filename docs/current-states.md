# Current States

## [Active] Reliable CTGS Tooltip Regression Fix

### Will be done

- Obtain one direct Edge/manual pointer-hover acceptance signal against the rebuilt source. The in-app Browser session could not produce a CSS `:hover` state from its pointer-move control, so mouse hover remains `NOT UX ACCEPTED` even though the implemented handler and the shared rendered tooltip path are present.
- Close this plan only when the user says `Đóng plan`.

### Done

- User confirmed that hovering the compact CTGS number did not display the approved full-number tooltip.
- Root cause was proven: the served Vite source contained only an HTML `title` attribute, no rendered `role="tooltip"` element; the previous build/tests did not exercise hover behavior.
- User approved points 1–4 and explicitly authorized implementation plus Browser UAT.
- Replaced native `title` with a portalled application tooltip, kept the full immutable CTGS in the accessible label/raw data, and added mouse-hover, keyboard-focus, Escape, touch/pen and outside-pointer handling.
- Rebuilt and restarted the local Docker runtime after the final source change; PostgreSQL and Redis were healthy and backend/admin started successfully.
- Browser UAT passed desktop focus/full-value/viewport placement and Escape closure; at 390×844 it passed tap/click, full-value rendering, outside closure and no horizontal overflow. No browser console warnings/errors were recorded.
- The same Browser session could not synthesize a pointer hover: moving over the proven visible CTGS element left `element.matches(':hover') === false`. This criterion is recorded as skipped by tool limitation, not passed.
- User required click-to-copy for `BF-PAY-ffd22b…58faeb` and approved review points 1–4, including implementation and Browser UAT.
- Made every non-empty CTGS a copy control with an explicit copy icon and accessible label. Click, Enter and Space copy the full immutable CTGS value, while compact display text remains presentation-only; clipboard rejection produces an error toast without changing any journal entry.
- Added focused clipboard and keyboard-key regression coverage. The complete frontend suite passed 31/31 tests, the production build passed, and the bundle-budget gate passed on the final source state.
- Rebuilt and restarted the Docker stack after the final keyboard change; PostgreSQL and Redis became healthy and backend/admin started successfully.
- Browser UAT against the rebuilt `Golden Project – Hoàn thiện Văn phòng An Phát` Sổ Nhật ký chung passed desktop click, Enter, Space, exact full-value clipboard content, success toast, tooltip text and Escape closure. At 390×844 it passed click-to-copy, success toast, tooltip containment and zero document-level horizontal overflow. No browser console warnings/errors were recorded.

## [Active] Production Readiness And Maintainability Program

### Will be done

- Rerun C1 and C2 J0-J9 with the 15-role Golden Sample matrix in an isolated disposable environment when Browser verification is explicitly authorized in a future turn.
- Provision the selected production S3-compatible provider and migrate existing local objects with inventory, checksum and cross-replica validation before enabling multiple application replicas.
- Select the managed Kubernetes/edge vendor, production cluster/network CIDRs, managed PostgreSQL/Redis/S3 instances and external secret controller; Compose remains the controlled single-host topology until the reviewed Kubernetes overlay and cross-replica gates pass.
- Continue decomposing oversized backend core owners after the reporting slice, without changing public API, schema, RBAC or workflow behavior.
- Update the central/detail/domain maps whenever implementation changes owner files, CI/runtime contracts, audit behavior, storage, or route-loading behavior.

### Done

- Assessed the repository at 78/100 and confirmed the seven readiness gaps against current code, tests, migrations, CI, production Compose, and runbooks.
- Prepared and received user approval for the cross-cutting implementation roadmap and delegated execution.
- Added the aggregate `npm test` command for all committed frontend tests and made it a required CI step before the frontend production build; all 23 tests pass locally.
- Added a PostgreSQL CI service and required DB-backed core/migration tests so those checks cannot silently skip; fixed the PostgreSQL `DISTINCT`/ordering defect exposed by that gate.
- Centralized audit writes: critical governance/workflow/finance/quality transitions are transactional and fail closed, while explicitly non-critical events remain observable best-effort with structured failure logs.
- Split the giant backend integration suite into domain-owned files with shared fixtures, and moved the reporting handler/service/repository/type slice into focused owner files without changing routes, API, schema or RBAC.
- Added the 12-role J0-J9 Golden Sample UAT matrix, isolated-DB/reset contract, evidence requirements and explicit `BLOCKED-COVERAGE` state for missing dedicated demo actors.
- Added `_FILE` production secrets, a pinned-session advisory-locked one-shot migration job, incompatible-rollback detection, safe drain/migrate/start runbooks and secret-safe backup guidance.
- Added internal Prometheus HTTP/Go/process/DB metrics with bounded labels and kept `/metrics` unavailable through public Nginx.
- Added a provider-neutral protected-file storage registry plus local provider and migrated evidence, workflow signatures/PDFs and input/output invoice files behind authorization-before-open streaming. Independent review closed post-commit cleanup data loss, symlink-swap TOCTOU and registry concurrency risks with regression/race coverage.
- Added the portable S3-compatible provider, fail-closed startup write/read/delete preflight, dual-provider legacy reads, V70/V71 provider and storage-key integrity migrations, and a dry-run-by-default local-to-S3 operator command with checksum verification, conditional metadata updates and no source deletion.
- Added a fail-closed portable Kubernetes/Kustomize application baseline with two backend/admin replicas, private Services and ingress NetworkPolicies, one-at-a-time two-node-safe rollout, PDBs, cross-node anti-affinity/zone spread, probes/resources/security contexts, enforced rate limiting, separate runtime DML versus migration DDL/owner credentials, release-unique migration Job, managed PostgreSQL/Redis/S3 and secret-manager contracts, startup S3 preflight, forward-only object-cutover guidance, optional gated HPA/Ingress examples and deterministic offline validation. No cluster was deployed.
- Made evidence gallery/preview and handover source presentation provider-aware: internal `local|s3` objects use protected endpoints, while external, none and unknown providers remain excluded.
- Converted authenticated page modules to route-level lazy loading below auth/permission guards while keeping Login, guards and AppShell eager; added standard/focus-shell loading and recoverable error states plus a CI gzip budget of 250 KiB for the entry and every JavaScript chunk. The production entry is now 119.9 KiB gzip and the largest lazy chunk is 35.8 KiB gzip.
- Corrected the production runbook: Redis Pub/Sub already provides shared SSE fanout, while shared object storage, deployment/runtime choices and cross-replica validation remain required before scaling.
- Updated central, backend, root-cause and affected domain maps for the new owners and operational contracts.
- Passed the full backend suite against an isolated PostgreSQL database, storage/core race checks, `go vet ./...`, all 72 migration manifest checks, 23 frontend tests, the production frontend build and bundle budget, Kubernetes render validation, agent mapping validation and diff hygiene checks; browser verification was not run by policy.
- Ran the authorized Golden Sample Browser UAT in isolated DB `consflow_uat_20260722_115155`: all 12 demo actors authenticated and saw Golden Live, completed fixtures remained unchanged, but C1 failed at J1 because PM lacks the required `planning.edit` permission for draft/evidence mutations; C2 and J2-J9 remain blocked by that shared prerequisite and missing dedicated `Thiết kế`/`Sale`/`Pháp lý` actors. Evidence is stored outside the repo at `/private/tmp/consflow-uat-evidence/20260722_115155`.
- Resolved the confirmed Golden Sample UAT blockers: V72 and seed parity grant PM `planning.view`/`planning.edit` while preserving project/work-item scope; dedicated `Thiết kế`, `Sale` and `Pháp lý` users/staff/roles raise Golden Live membership to 15; validators, runbooks, UAT matrix and maps now cover all actors and rate-limit-safe evidence capture. Migration/seed validation and the full backend suite pass on disposable PostgreSQL databases; Browser rerun remains explicitly pending.
