# Current States

## Active Plan: Canonical production bootstrap and operator commands

Status: IMPLEMENTED AND TECHNICALLY VERIFIED LOCALLY AND IN PR CI; EXTERNAL PRODUCTION SETUP IN PROGRESS (approved by user on 2026-08-20)

### Will be done

- Replace the one-off randomly named local secret bundle with an ignored, mode-0600 `deploy/config/production.env` created from a committed secret-safe schema.
- Add idempotent commands to bootstrap/generate secrets, render and validate the VPS payload, provision the VPS over pinned key-only SSH, configure only deploy-related GitHub secrets, and create an exact `vYYYY-MM-DD.N` release tag.
- Keep PostgreSQL/Redis/JWT/settings/S3/Admin runtime credentials off GitHub; provision them directly to `/opt/consflow/shared` while GitHub receives only VPS host/port/user/private-key/verified-known-hosts.
- Preserve GHCR digest-pinned releases, main-ancestry validation, no Actions trigger on push to `main`, Caddy Automatic HTTPS, one-shot migration and fail-closed Admin bootstrap.
- Migrate the already generated temporary secret material into the canonical ignored config without printing secrets, then validate permissions, placeholder gates, Compose rendering and shell contracts.
- Update the production runbook, README, central runtime map and root-cause triage map for the new operator workflow.

### Done

- Compared Noah's canonical `project.env`, GitHub-secret helper, VPS renderer/provisioner and tag helper with Consflow's current release design.
- User approved adopting Noah's compact operator experience while retaining Consflow's key-only pinned SSH, runtime-secret isolation, strict tag format and immutable GHCR digest deployment.
- Added ignored mode-`0600` canonical operator config and SSH material under `deploy/config`, with a committed secret-safe schema and shared parser that never evaluates config values as shell code.
- Added idempotent bootstrap, fail-closed validation, runtime payload rendering, explicit-scope GitHub secret upload, pinned key-only VPS provisioning and strict tag-release helpers behind concise Make targets.
- GitHub receives only five raw deploy-transport secrets; PostgreSQL, Redis, JWT, settings, Admin and S3 runtime credentials remain local/VPS-only and are never Base64-encoded into GitHub Secrets.
- Migrated the prior random temporary bundle byte-for-byte into the canonical ignored paths, verified file modes and ignore rules, then removed the exact old temporary directory.
- Added deterministic local production-tooling coverage to reusable validation. It exercises bootstrap idempotence, placeholder rejection, SSH key/fingerprint consistency, payload rendering and permissions, GitHub/VPS dry runs, Production Compose rendering and isolated annotated tag creation/push.
- Updated the production runbook, README, Makefile, CI validation step, central runtime map and root-cause triage map for the canonical operator workflow.
- Current approved source state passes Bash syntax, release-tag tests, production-tooling end-to-end tests, actionlint, generated-secret leak/ignored-path/mode checks, `git diff --check`, all five reconciled focused tests on separate clean databases, full `go test -p 1 ./...` on a sixth clean database, 212/212 frontend tests, frontend production build and bundle budget. `shellcheck` was unavailable and therefore skipped.
- The real canonical config has all provider-controlled values resolved, pins the user-confirmed ED25519 host fingerprint and passes `bash deploy/scripts/validate-production-config.sh --all` without printing secrets.
- Committed the implementation on `codex/production-deploy`, pushed it and opened draft PR [#1](https://github.com/khiemnd777/ttcons/pull/1). Follow-up CI and release reconciliation is committed through `c6030aa`; GitHub Actions run `32354371208` passed all backend, frontend, Caddy-image, bundle-budget, whitespace, release-tag and production-tooling checks.
- Remote CI at `529777e` failed on four stale invariant fixtures already failing on baseline `main` plus one accounting fixture newly exposed by V112. The approved reconciliation updates only test fixtures/expectations: Rev8 workspace counts, exact select-other validation, reserved seal-management actors, isolated V26 replay and reuse of V112's Admin role. All five now pass independently and in the full backend suite; no production gate was weakened or skipped.
- Replaced the long-lived VPS GHCR token with the release job's short-lived `GITHUB_TOKEN`. Provisioning no longer accepts or stores registry credentials; deployment sends the job token over SSH stdin into an ephemeral remote `DOCKER_CONFIG` and deletes it at shell exit. Static production tooling and actionlint cover this contract.
- Vietnix S3 credentials authenticate successfully to `tt-dnh-consflow-prod` with region `vn-hcm-1` and virtual-host addressing. The empty bucket initially granted anonymous listing; its ACL was changed to private, authenticated temporary put/get/delete passed with zero objects left behind, and anonymous access now returns HTTP `403`.
- Created GitHub Environment `production`, uploaded exactly five raw deploy-transport secrets (`VPS_HOST`, `VPS_PORT`, `VPS_USER`, `VPS_SSH_PRIVATE_KEY`, `VPS_SSH_KNOWN_HOSTS`) and limited deployments to tag pattern `v*`; the exact workflow validator still enforces `vYYYY-MM-DD.N` and main ancestry.
- GitHub rejected required-reviewer protection for this private repository with HTTP `422` because the current billing plan does not support it. No manual approval gate is active; a valid release tag will deploy automatically after validation unless the repository plan is upgraded and a reviewer is configured.

### Skipped / remaining acceptance

- No VPS provisioning, merge, tag creation, GHCR build, production migration/Admin bootstrap or deployment has run yet.
- The first password-based `ssh-copy-id` attempt was rejected and installed no key. The user confirmed that password was rotated and confirmed the VPS ED25519 fingerprint; the canonical config pins it. Installing the public key is pending only because the current local clipboard is empty.
- DNS A resolves `tt-dnh.com` to `160.30.113.13`, but ports 80/443 are not serving yet. Public Caddy/ACME, production smoke/login and rollback remain unverified.

## Active Plan: Tag-gated VPS production release, Caddy Auto HTTPS, S3 and Admin bootstrap

Status: IMPLEMENTED AND TECHNICALLY VERIFIED LOCALLY AND IN PR CI; VPS PROVISIONING AND RELEASE PENDING (approved by user on 2026-08-20)

### Will be done

- Make Production Compose consume immutable backend/admin image references produced from an exact `vYYYY-MM-DD.N` release tag; a push to `main` must not trigger GitHub Actions.
- Run PR/manual validation separately, validate that every release tag is calendar-valid and points to a commit reachable from `main`, then build/push GHCR images and enter the GitHub `production` Environment before VPS deployment. The current repository plan cannot enforce required reviewers, so the deploy is automatic after validation until that protection becomes available.
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
- Split reusable validation from CI. Pull requests/manual dispatch validate; push/merge to `main` has no workflow trigger. Only exact calendar-valid `vYYYY-MM-DD.N` tags reachable from `origin/main` build backend/admin GHCR images and pass digest references to the `production` Environment deploy job.
- Added fail-closed VPS deploy script: validate tag/digests/Compose/secrets, pull exact images, start dependencies, drain, run one-shot migration and Admin bootstrap, wait backend/Caddy, then record the successful release/current symlink; database rollback is never automatic.
- Added the authoritative Vietnamese VPS runbook including copy-ready DNS request, Vietnix bucket/key request, host directory ownership, secret layout, ephemeral GHCR authentication, GitHub Environment, release command, Admin first-use cleanup, HTTPS smoke checks and rollback boundary. Updated production/rollback docs, README and all matched mapping references.
- The approved test/invariant reconciliation and ephemeral GHCR authentication change pass local focused/full backend, frontend, workflow, release-tag, production-tooling and bundle checks on the current source state. GitHub Actions run `32354371208` passed the same PR validation surface on pushed revision `c6030aa`.
- Created GitHub Environment `production`, uploaded exactly the five deploy-transport secrets and set the custom deployment tag policy to `v*`. Required-reviewer configuration remains unavailable on the current private-repository billing plan; HTTP `422` was returned and no approval gate was applied.
- Final isolated Caddy smoke used the current built image as its default UID/GID 101 with read-only root, all Linux capabilities dropped and container-scoped low-port sysctl: local Automatic HTTPS returned 200 and loopback health returned 204. The public domain/ACME path was intentionally not exercised before DNS cutover.

### Skipped / remaining acceptance

- DNS A is ready, the Vietnix bucket exists/authenticates/is private, GitHub Environment/secrets/tag policy are configured, and the user confirmed password rotation plus the pinned ED25519 fingerprint. VPS Docker/firewall/key-only SSH/runtime secrets are not provisioned because the rotated root password is not currently present in the local clipboard; no tag, GHCR package or production deployment exists yet.
- Workflow YAML and local release components are technically checked, and PR validation passed on GitHub-hosted Actions. The real tag build and Environment deploy path have not run.
- Public Caddy ACME issuance, HTTP redirect, production migration/bootstrap, login and rollback are not verified until those external prerequisites are complete. Real Vietnix S3 write/read/delete and private-access preflight passed independently.
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
