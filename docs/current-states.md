# Current States

## Active Plan: Migrate production origin gateway from Nginx to Caddy

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; PRODUCTION DEPLOYMENT NOT VERIFIED (approved by user on 2026-08-20)

### Will be done

- Replace the production `admin` image's Nginx gateway with pinned standard Caddy while preserving service name, port `8080`, SPA/static behavior, private backend routing and managed-edge TLS termination.
- Preserve `/gateway-health`, exact public `/metrics` deny, trusted-client-IP normalization, request IDs, security/cache headers, route-specific body limits, SSE streaming and controlled upstream-unavailable responses.
- Keep development on Vite; switch Production Compose backend rate limiting to `enforce` and retain Docker log rotation without adding non-standard Caddy modules.
- Simplify the Kubernetes admin pod by removing the Nginx-specific render init container while preserving non-root, read-only, probe, resource, affinity, NetworkPolicy and stable `backend:8080` contracts.
- Update focused tests, CI gateway validation, production/Kubernetes runbooks and the mandatory module/backend maps.
- Validate Caddy adaptation, production image build and smoke behavior, Production Compose shape, Kubernetes render assertions, focused backend tests and whitespace. Browser/UAT remains skipped because no UI behavior changes.

### Done

- Review package and three migration decisions approved: standard Caddy without third-party rate-limit modules; TLS remains at the managed edge; production/Kubernetes migrate while development remains on Vite and gateway access logs become JSON.
- Mandatory module-memory and Docker Ops instructions loaded; owner code, Compose, Kubernetes, observability test and operations documentation inspected.
- Baseline revision `2a178e7` was clean. Existing Kubernetes render assertions passed and focused observability tests passed with a sandbox-local Go cache.
- Replaced both Nginx configs and the production image stage with pinned `caddy:2.11.3-alpine`; image build now validates the reviewed `Caddyfile` and CI builds that production image.
- Preserved port `8080`, managed-edge TLS, strict trusted-proxy client-IP normalization, generated request IDs, route-specific body/timeouts, immediate SSE flushing, exact public `/metrics` deny, immutable assets, no-store SPA fallback and controlled JSON `503` responses.
- Production Compose now runs Caddy as UID/GID `101:101` with a read-only root filesystem, owner-only bounded tmpfs for `/config`, `/data` and `/tmp`, and backend rate limiting defaulted to `enforce`; development Compose/Vite is unchanged.
- Kubernetes admin pods run the same image Caddyfile directly against stable `backend:8080`; the Nginx render init container and `SERVER_NAME` contract were removed while UID/GID/fsGroup, probes, resources, affinity, PDB, NetworkPolicy and bounded ephemeral storage remain asserted.
- Updated the focused observability contract test, Kubernetes deterministic render checks, production/Kubernetes operations documentation and all matched central/backend/root-cause maps.
- Final current-tree checks passed on 2026-08-20: Caddy adaptation; production image build; production and development Compose configuration; Kubernetes render/11-object deterministic contract; focused `go test ./internal/shared/observability`; and `git diff --check`.
- Final isolated Docker smoke used the built image as UID/GID 101 with a read-only root: `/gateway-health` returned `204`, exact `/metrics` returned `404`, SPA deep-link returned `200` with `no-store`, trusted forwarded IP/proto/request UUID reached a consuming mock backend, reviewed SSE emitted immediately, an oversized login body returned `413`, an unavailable upstream returned the JSON `503` envelope with `Retry-After: 5`, and access output was JSON without storage/runtime errors.

### Skipped / remaining acceptance

- No production host, managed edge/WAF policy or Kubernetes cluster was changed or exercised. Firewall allowlists, edge rate/connection quotas, real proxy CIDRs, rollout/drain behavior and cross-replica SSE remain production acceptance gates.
- Browser/UAT was not run because this migration changes no UI behavior. The production frontend compilation is technically verified, but no new UX acceptance claim is made.
- No database schema or migration changed, and no migration command was run for this gateway-only work.
- The production frontend dependency install reported the repository's existing npm audit warning (1 moderate and 3 high vulnerabilities); this migration did not attempt dependency remediation.

## Active Plan: Typed system settings, Developer governance, seal table, local login defaults

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; NOT UX ACCEPTED (approved by user on 2026-08-19)

### Will be done

- Keep bootstrap/deployment/root-secret fields in `.env` or Docker secrets.
- Move approved runtime/application policy fields into typed database-backed settings with validation, masking, audit and restart metadata.
- Wire `client_payment_min_percent` into the construction payment gate and normalize `surplus_handling_options` to the existing warehouse enum.
- Add system role `developer` with full permissions; keep Admin seal management but make system settings read-only; hide Settings from other roles.
- Redesign `/settings` as `Con dấu` and `Thiết lập hệ thống` tabs; render seals as an actions-first, thumbnail-second responsive table.
- Remove the demo-account selector while retaining local-development prefill `admin@consflow.local` / `123456`; production login fields stay blank.
- Add local `developer@consflow.local` / `123456`; production migration creates the role but no developer user.
- Preserve current custom migration runner, Docker service names and down -> build -> run Make workflow.
- Update affected module/detail/domain maps in the same implementation turn.
- Validate backend formatting/tests, frontend tests/build and local/production Compose configuration. Browser/UAT remains skipped unless explicitly requested.

### Done

- Review package, environment/database field split, RBAC matrix, Mockup as Text and validation plan approved.
- Mandatory module mapping and Admin Governance, Backend, Frontend, Database, Docker, Workflow, Materials and Accounting guardrails loaded.
- Cross-domain collision review completed: no ledger/status transition ownership conflict; runtime bootstrap and settings/audit/RBAC contracts require coordinated changes.
- Added V110 typed/versioned `system_settings`, Developer system role, reserved Settings RBAC cleanup and future-assignment guard. The custom migration runner and applied-migration checksum contract remain unchanged.
- Added two-stage backend bootstrap: validate core `.env`, migrate/connect, one-time import only missing database settings, apply database-authoritative runtime values, validate and reconnect with configured pool values.
- Added a 78-field typed database/UI registry, atomic optimistic bulk update, audit events, restart-pending metadata, API secret redaction and AES-GCM storage for `SMTP_PASSWORD` using the core `SETTINGS_ENCRYPTION_KEY`.
- Wired the hot `client_payment_min_percent` setting into the construction payment gate and `surplus_handling_options` into server validation plus the work-item selector without rewriting historical records.
- Enforced Developer full settings/seal management, Admin seal management plus read-only system settings, and no Settings permissions/navigation for other roles in migration, seed and role-permission replacement logic.
- Rebuilt Settings into `Con dấu` and `Thiết lập hệ thống`: actions-first/thumbnail-second seal table with mobile cards; typed categorized controls, validation, secret clear flow, restart state and read-only 26-field core `.env` inventory.
- Removed the demo-account selector. Development prefills `admin@consflow.local` / `123456`; production fields are blank. Local seed adds `developer@consflow.local` / `123456`; migration creates no production user.
- Updated `.env` examples, local/production Compose secret wiring, definitive field/RBAC documentation, README behavior and all affected module/detail/domain maps.
- Technical verification on the final working tree: `go test ./...` passed; frontend `207/207` tests passed; frontend production build passed; `git diff --check` passed; local and production Compose configs passed.
- Database/runtime verification: V110 applied through the custom runner; clean disposable database ran all 110 migrations plus seed with correct Admin/Developer grants; final backend image rebuilt and started with 110 migrations recorded, PostgreSQL/Redis services healthy and `/api/ready` returned 200/database `ok`.
- Final API verification: Admin receives 78 DB/UI settings, 26 core fields and `can_update=false`; Admin PUT returns 403; Developer receives `can_update=true`; sensitive plaintext count in the Settings response is zero.

### Skipped / remaining acceptance

- Browser/UAT was not run because the user did not explicitly request browser control. UI/UX status remains `NOT UX ACCEPTED`; frontend tests and build prove technical integrity only.
