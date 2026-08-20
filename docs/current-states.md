# Current States

## Active Plan: Canonical production bootstrap and operator commands

Status: IMPLEMENTED AND TECHNICALLY VERIFIED LOCALLY; PRODUCTION INPUTS AND EXECUTION PENDING (approved by user on 2026-08-20)

### Will be done

- Replace the one-off randomly named local secret bundle with an ignored, mode-0600 `deploy/config/production.env` created from a committed secret-safe schema.
- Add idempotent commands to bootstrap/generate secrets, render and validate the VPS payload, provision the VPS over pinned key-only SSH, configure only deploy-related GitHub secrets, and create an exact `vYYYY-MM-DD.N` release tag.
- Keep PostgreSQL/Redis/JWT/settings/S3/Admin/GHCR runtime credentials off GitHub; provision them directly to `/opt/consflow/shared` while GitHub receives only VPS host/port/user/private-key/verified-known-hosts.
- Preserve GHCR digest-pinned releases, main-ancestry validation, no Actions trigger on push to `main`, Caddy Automatic HTTPS, one-shot migration and fail-closed Admin bootstrap.
- Migrate the already generated temporary secret material into the canonical ignored config without printing secrets, then validate permissions, placeholder gates, Compose rendering and shell contracts.
- Update the production runbook, README, central runtime map and root-cause triage map for the new operator workflow.

### Done

- Compared Noah's canonical `project.env`, GitHub-secret helper, VPS renderer/provisioner and tag helper with Consflow's current release design.
- User approved adopting Noah's compact operator experience while retaining Consflow's key-only pinned SSH, runtime-secret isolation, strict tag format and immutable GHCR digest deployment.
- Added ignored mode-`0600` canonical operator config and SSH material under `deploy/config`, with a committed secret-safe schema and shared parser that never evaluates config values as shell code.
- Added idempotent bootstrap, fail-closed validation, runtime payload rendering, explicit-scope GitHub secret upload, pinned key-only VPS provisioning and strict tag-release helpers behind concise Make targets.
- GitHub receives only five raw deploy-transport secrets; PostgreSQL, Redis, JWT, settings, Admin, S3 and GHCR runtime credentials remain local/VPS-only and are never Base64-encoded into GitHub Secrets.
- Migrated the prior random temporary bundle byte-for-byte into the canonical ignored paths, verified file modes and ignore rules, then removed the exact old temporary directory.
- Added deterministic local production-tooling coverage to reusable validation. It exercises bootstrap idempotence, placeholder rejection, SSH key/fingerprint consistency, payload rendering and permissions, GitHub/VPS dry runs, Production Compose rendering and isolated annotated tag creation/push.
- Updated the production runbook, README, Makefile, CI validation step, central runtime map and root-cause triage map for the canonical operator workflow.
- Current uncommitted working-tree checks on 2026-08-20 passed: Bash syntax; release-tag tests; production-tooling end-to-end test; full `go test ./...`; frontend production build; workflow YAML parsing; generated-secret leak scan; ignored-path/mode checks; and `git diff --check`. `shellcheck` was unavailable and therefore skipped.
- The real canonical config intentionally fails `make production-check` while provider-controlled placeholders remain unresolved; no incomplete configuration can be rendered or provisioned.

### Skipped / remaining acceptance

- No real VPS provisioning, GitHub secret mutation, commit, push, tag creation, GHCR build, production migration/Admin bootstrap or deployment was run.
- External inputs still required are S3 Access Key/Secret Key, a VPS host-key fingerprint independently verified through the provider console and a GHCR read token. The current scanned fingerprint is only a candidate and is not trusted automatically.
- Public DNS/ACME, real S3 preflight, GitHub-hosted workflow/Environment approval and production smoke/login/rollback remain unverified.

## Active Plan: Tag-gated VPS production release, Caddy Auto HTTPS, S3 and Admin bootstrap

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; PRODUCTION DEPLOYMENT BLOCKED ON EXTERNAL PREREQUISITES (approved by user on 2026-08-20)

### Will be done

- Make Production Compose consume immutable backend/admin image references produced from an exact `vYYYY-MM-DD.N` release tag; a push to `main` must not trigger GitHub Actions.
- Run PR/manual validation separately, validate that every release tag is calendar-valid and points to a commit reachable from `main`, then build/push GHCR images and require the GitHub `production` environment approval before VPS deployment.
- Change single-host Production Compose from managed-edge HTTP `:8080` to direct Caddy automatic HTTPS for `tt-dnh.com` on public TCP `80/443`, while preserving the reusable `:8080` application listener for Kubernetes and persisting Caddy certificate state.
- Pin the Vietnix production bucket contract to private bucket `tt-dnh-consflow-prod`, HTTPS endpoint `https://s3.vn-hcm-1.vietnix.cloud`, provider-confirmed region and startup read/write/delete preflight.
- Add a production-safe V112 RBAC catalog migration because fresh production migrations currently do not create the base roles/permissions held in demo seed data.
- Add an idempotent, fail-closed, non-HTTP Admin bootstrap command that reads its first-use password from a Docker secret, creates linked user/staff/global Admin assignment plus audit atomically, and never resets an existing Admin password.
- Add a single-host VPS deploy script and runbook covering host/bootstrap secrets, migration, Admin bootstrap, application rollout, DNS, HTTPS smoke checks and rollback boundaries.
- Update central/backend/root-cause/Admin governance maps for the new migration, command and production delivery contracts.
- Validate migration integrity and fresh-database RBAC, bootstrap behavior, backend tests/format, release-tag tests, workflow syntax, frontend build/Caddy validation, Production Compose shape and whitespace. Browser/UAT remains skipped because no UI behavior changes.

### Done

- User approved V112, fail-closed Admin bootstrap, direct Caddy Auto HTTPS, tag-only release with production approval, GHCR immutable images and Vietnix bucket name `tt-dnh-consflow-prod`.
- Mandatory central/backend/root-cause/Admin maps and Repo Architect, Impact Analysis, Docker Ops, Backend, Admin Governance and Git workflow instructions loaded.
- Baseline revision `27fb2be` on clean `main` confirmed. At that baseline, CI still ran on pushes to `main`; Production Compose built mutable local images and published Caddy HTTP `:8080`; Caddy certificate directories were tmpfs; fresh migrations depended on demo seed for the base Admin/RBAC catalog.
- Impact review found no frontend/API contract change. Required collision checks are migration checksums, reserved Admin/Developer permissions, external-only Client behavior, atomic audit, S3 startup preflight, Caddy/Kubernetes listener compatibility and production secret isolation.
- Added additive/idempotent V112 to create the production-safe 18-role/base-permission catalog without demo users or business seed data, preserve custom roles/assignments, reapply later quality/construction grants and enforce Client/Settings/Event Logs/professional-accounting boundaries.
- Added `consflow-bootstrap-admin` as a non-HTTP, serializable, advisory-locked one-shot command. It requires V112 and complete Admin grants, reads only a first-use password file, bcrypt-creates linked user/staff/global Admin plus audit atomically, fails closed on conflicting identity/Admin state and no-ops for the same healthy Admin without rereading or resetting the password.
- Production Compose now consumes required backend/admin image references, persists Caddy certificate/config named volumes, publishes only Caddy TCP 80/443, gives the unpublished backend a dedicated outbound network for S3/SMTP while data services stay internal-only, mounts AWS shared credentials as a Docker secret and fixes the S3 bucket to `tt-dnh-consflow-prod` with provider-confirmed Region pending.
- Caddy now selects `APP_DOMAIN=tt-dnh.com` for Automatic HTTPS while keeping default HTTP `:8080` for Kubernetes and a forced-HTTP loopback-only `:8081` Docker health listener. The image runs UID/GID 101, removes the inherited low-port file capability; Production Compose drops all capabilities and enables only the container-scoped unprivileged-low-port sysctl.
- Split reusable validation from CI. Pull requests/manual dispatch validate; push/merge to `main` has no workflow trigger. Only exact calendar-valid `vYYYY-MM-DD.N` tags reachable from `origin/main` build backend/admin GHCR images and pass digest references to the reviewer-gated `production` Environment deploy job.
- Added fail-closed VPS deploy script: validate tag/digests/Compose/secrets, pull exact images, start dependencies, drain, run one-shot migration and Admin bootstrap, wait backend/Caddy, then record the successful release/current symlink; database rollback is never automatic.
- Added the authoritative Vietnamese VPS runbook including copy-ready DNS request, Vietnix bucket/key request, host directory ownership, secret layout, GHCR login, GitHub Environment, release command, Admin first-use cleanup, HTTPS smoke checks and rollback boundary. Updated production/rollback docs, README and all matched mapping references.
- Current working-tree technical checks passed on 2026-08-20: 112 migration checksum validation; fresh disposable PostgreSQL full migration chain plus V112 idempotence/RBAC assertions; fresh Admin create/audit/no-op/conflict integration; `go test ./...`; Go format check; 212/212 frontend tests; frontend production build and bundle budget; backend and frontend Docker image builds; release-tag tests and Bash syntax; YAML parsing for all workflows/Compose; Production Compose render; Kubernetes 11-object deterministic render; and `git diff --check`.
- Final isolated Caddy smoke used the current built image as its default UID/GID 101 with read-only root, all Linux capabilities dropped and container-scoped low-port sysctl: local Automatic HTTPS returned 200 and loopback health returned 204. The public domain/ACME path was intentionally not exercised before DNS cutover.

### Skipped / remaining acceptance

- External blockers remain fail-closed: DNS A/AAAA/CAA has not been changed; VPS Docker/firewall/SSH/GHCR login/secrets have not been provisioned; Vietnix has not created the bucket or confirmed the exact Region/path-style behavior; GitHub Environment/reviewer/secrets/tag protection have not been configured; no tag, GHCR package or production deployment exists yet.
- Workflow YAML and local release components are technically checked, but GitHub-hosted Actions and the real Environment approval/deploy path have not run.
- Public Caddy ACME issuance, HTTP redirect, real S3 write/read/delete preflight, production migration/bootstrap, login and rollback are not verified until those external prerequisites are complete.
- Browser/UAT is out of scope because there is no UI behavior change.

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
- At that plan's verification revision, Production Compose ran Caddy as UID/GID `101:101` with a read-only root filesystem and owner-only bounded tmpfs for `/config`, `/data` and `/tmp`; the later active tag-gated VPS plan supersedes `/config` and `/data` with persistent named volumes for Automatic HTTPS certificate state. Backend rate limiting remains `enforce`; development Compose/Vite is unchanged.
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
