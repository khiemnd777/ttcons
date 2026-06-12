# Project Operations Production Plan (Closed)

Status: closed on 2026-06-12. Batches 1 through 8 are implemented or validated in the current working tree.

## Objective

Turn Consflow from a broad admin-module demo into a construction workflow operating system centered on project runtime work, scoped operations dashboards, personal work inboxes, auditability, and guarded domain transitions.

## Implemented Baseline

- Runtime work item foundation, evidence metadata, scoped activities, and pipeline-run work item generation.
- Project Operations API with server-side project visibility, workflow summary, active work, blockers, approvals, evidence, activity, audit, and department surfaces.
- Work item actions for start, complete, block, resolve, review, approval, reject, request changes, evidence, and activity.
- Legacy module RBAC and project-scope hardening across project, pipeline, approvals, BOQ, procurement, warehouse, operations, acceptance, rework, handover, accounting, reports, staff, and settings surfaces.
- Project Operations Dashboard first UI pass in `ProjectDetailPage`.
- `/my-work` personal inbox with shared work item card/detail/action UI.
- Explicit backend transitions for procurement, stock/site material balances, acceptance failure and rework, handover blockers, payables, receivables, and payments.

## Batch 8 Validation

- Passed: `GOCACHE=/Users/khiemnguyen/Works/ttcons/ttcons/backend/.gocache go test ./...`.
- Passed: `cd frontend && npm run build`.
- Passed: `docker compose config`.
- Passed: `bash .agents/scripts/validate.sh`.
- Blocked by local DB state: `go run ./cmd/migrate` reached local PostgreSQL, but the existing local volume does not contain role `consflow`.
- Skipped by repo rule: Browser, browser-use, Playwright, and headless browser verification.

## Residual Risks

- Browser/manual QA is still needed for the new dashboard and `/my-work` UI.
- Evidence remains metadata/path based; binary upload/storage is still future work.
- Cross-domain work item generation beyond pipeline run steps remains incomplete.
- Sidebar/module-wide frontend permission states still need a dedicated pass.
- Local migration runtime validation needs a healthy PostgreSQL data volume with the expected app role, or explicit user approval to recreate the local volume.

## Suggested Next Skills

- `consflow-repo-architect` for any broad follow-up.
- `consflow-frontend-workflow` for browser/manual UI hardening.
- `consflow-backend-workflow` for evidence upload APIs or cross-domain work item generation.
- `consflow-pipeline-domain` for workflow invariants and construction-domain transitions.
- `consflow-docker-ops` for local stack and migration environment repair.
- `consflow-diagnose` for regression triage.
