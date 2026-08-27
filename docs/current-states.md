# Current States

## Release v2026-08-27.1

### Will be done

- Preserve the fail-closed cross-module Work Item mutation policy while loading the persisted pipeline-step `surface_kind` and `domain_action_key` into the start-on-success decision.
- Make DB-backed integration fixtures carry domain-aware pipeline node/step context and align lifecycle assertions with automatic `ready -> in_progress` transitions.
- Provision an isolated Developer actor inside sample-data integration tests instead of depending on demo seed data.
- Run targeted regressions, the complete release validation suite, production checks, and only then commit, push, tag, deploy, and verify public health.
- Update the durable backend and root-cause maps for the corrected runtime query and test setup.

### Done

- Reproduced the previous release failure locally on a migration-only PostgreSQL 16 and Redis 7 environment.
- Confirmed 11 backend failures: eight missing domain execution contexts, one stale handoff lifecycle assertion, and two sample-data tests coupled to demo seed data.
- Confirmed the runtime Work Item lock query joined `pipeline_run_steps` without selecting `surface_kind` or `domain_action_key`.
- Implemented the persisted Run Step context selection, domain-aware integration fixture binding, handoff lifecycle assertion, and isolated Developer test actor.
- Passed all 11 targeted DB-backed regressions on a fresh migration-only PostgreSQL 16 and Redis 7 environment.
- Updated the module keyword, backend function, and root-cause triage maps with the corrected execution-context contract.
- Passed backend formatting, all 131 migration checksum checks, and `go test -p 1 ./...` on a fresh PostgreSQL 16 and Redis 7 environment.
- Passed release-tag validation, production operator tooling tests, all 287 frontend tests, frontend production build, the 250 KiB gzip bundle budget, production Caddy gateway image validation, and `make production-check`.
