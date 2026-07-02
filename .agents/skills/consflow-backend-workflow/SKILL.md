---
name: consflow-backend-workflow
description: Implement or review Consflow Go/Fiber backend work, including routes, handlers, services, repositories, auth/RBAC, PostgreSQL schema changes, custom SQL migrations, and seed data.
---

# Consflow Backend Workflow

Use this skill for backend API, data, auth, migration, and domain behavior changes.

## Required Reading

- `AGENTS.md`
- `backend/internal/core/routes.go`
- The relevant handler, service, repository, and type definitions in `backend/internal/core`
- `backend/internal/shared/migrations/runner.go` when schema changes are involved
- Existing SQL in `backend/migrations` before adding a migration

## Implementation Workflow

1. Trace the route and endpoint contract.
2. Locate the service method or repository helper that owns the behavior.
3. Confirm request and response shapes used by the frontend.
4. For bugfixes or unclear behavior changes, use `consflow-diagnose`: build a pass/fail signal when feasible, write falsifiable hypotheses, and identify the root cause or unresolved risk before proposing edits. Do not implement a speculative backend fix.
5. For feature, bugfix, or code-changing requests, present a review package with current behavior, expected behavior, evidence, root cause or unresolved risk, files likely to change, implementation plan, validation plan, and open questions. Wait for explicit user confirmation before implementation.
6. For schema changes:
   - create a new migration in `backend/migrations`
   - use a monotonically increasing version, normally `V<N>__description.sql`
   - do not change applied migrations because checksums are recorded
   - update seed data only when demo or required reference data changes
7. Keep validation and actor extraction close to handlers; keep business transitions in services.
8. Preserve response envelope behavior via `backend/internal/shared/response`.
9. User-visible list endpoints must filter out deleted records using the table's local marker, such as `status <> 'cancelled'`, `status <> 'deleted'`, or `active = true`.
10. Run focused validation.

## Migration Rules

See `references/migrations.md` for the local migration contract.

Never add Flyway, Flyway containers, Flyway config, or Flyway naming assumptions. The source of truth is the custom runner and `schema_migrations`.

## Verification

Prefer the narrowest useful checks:

```bash
cd backend && make test
cd backend && make fmt
cd backend && make migrate
```

For Docker-integrated changes:

```bash
make restart
make log SERVICE=backend
```

## Output Expectations

- Mention route, handler, service, repository, and migration files changed.
- Describe any API contract changes for the frontend.
- State whether migrations were added and whether they were run.
