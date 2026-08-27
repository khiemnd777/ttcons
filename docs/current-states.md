# Current States

## Release v2026-08-27.1

### Will be done

- Retain this completed release record until the user explicitly asks to close the plan; then move it to `docs/plan-closed/` under the normal close-plan workflow.

### Done

- Reproduced the previous release failure locally on a migration-only PostgreSQL 16 and Redis 7 environment.
- Confirmed 11 backend failures: eight missing domain execution contexts, one stale handoff lifecycle assertion, and two sample-data tests coupled to demo seed data.
- Confirmed the runtime Work Item lock query joined `pipeline_run_steps` without selecting `surface_kind` or `domain_action_key`.
- Implemented the persisted Run Step context selection, domain-aware integration fixture binding, handoff lifecycle assertion, and isolated Developer test actor.
- Passed all 11 targeted DB-backed regressions on a fresh migration-only PostgreSQL 16 and Redis 7 environment.
- Updated the module keyword, backend function, and root-cause triage maps with the corrected execution-context contract.
- Passed backend formatting, all 131 migration checksum checks, and `go test -p 1 ./...` on a fresh PostgreSQL 16 and Redis 7 environment.
- Passed release-tag validation, production operator tooling tests, all 287 frontend tests, frontend production build, the 250 KiB gzip bundle budget, production Caddy gateway image validation, and `make production-check`.
- Committed and pushed runtime revision `a19b2d6debe416c24e1e97a116c2058ae9fe4482`, then created release tag `v2026-08-27.1` from that exact revision.
- GitHub Actions run `33027115506` completed successfully: tag validation, reusable validation, immutable backend/admin image build, fail-closed VPS production deployment, and public HTTPS verification all passed.
- Independent post-deploy checks passed: `/gateway-health` returned HTTP 204, `/api/ready` returned HTTP 200 with database `ok`, and the frontend root returned HTTP 200.
