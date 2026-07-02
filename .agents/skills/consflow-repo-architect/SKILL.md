---
name: consflow-repo-architect
description: Route broad Consflow repo work across Go backend, React admin, SQL migrations, Docker, and construction workflow domains. Use when a task is cross-cutting, ambiguous, asks where to implement, or touches API/frontend contracts.
---

# Consflow Repo Architect

Use this skill before broad implementation or review work in Consflow.

## Required Workflow

1. Read `AGENTS.md`, `README.md`, `docker-compose.yml`, and the minimum backend/frontend files relevant to the request.
   - If the prompt includes module, feature, domain, route, table, or utility keywords, first read `.agents/skills/consflow-module-memory/references/module-keyword-map.md` and use it to choose the initial owner paths.
2. Classify the work:
   - backend API or domain logic
   - frontend admin UI
   - database migration or seed data
   - Docker/Makefile operations
   - cross-contract change
   - business domain skill ownership
3. If the work may overlap existing features, mutate shared state, change status transitions, affect reports/exports, or cross backend/frontend/domain boundaries, run `consflow-impact-analysis` before proposing or editing behavior.
4. If the request is a bugfix or unclear behavior change, run `consflow-diagnose` before implementation. State current behavior, expected behavior, evidence, hypotheses, root cause or unresolved risk, and clarification questions. Do not propose an implementable fix from assumption.
5. If the request is a feature, bugfix, or code-changing task, prepare a review package with current behavior, expected behavior, evidence, root cause or unresolved risk, files likely to change, implementation plan, validation plan, and open questions. Wait for explicit user confirmation before implementation.
6. If the request changes UI or visual interface, prepare a "Mockup as Text" and wait for explicit user confirmation before implementation. "Mockup as Text" means an ASCII/wireframe-style visual mockup in text, not only a prose UX specification.
7. Identify the ownership path before editing:
   - Module/feature keyword cache: `.agents/skills/consflow-module-memory/references/module-keyword-map.md`
   - Feature overlap / logic collision: `consflow-impact-analysis`
   - Workflow orchestration: `consflow-workflow-orchestration-domain`
   - Project planning: `consflow-project-planning-domain`
   - Construction operations: `consflow-construction-operations-domain`
   - Commercial/procurement: `consflow-commercial-procurement-domain`
   - Materials/warehouse: `consflow-materials-warehouse-domain`
   - Quality/acceptance/handover: `consflow-quality-acceptance-handover-domain`
   - Accounting/finance: `consflow-accounting-finance-domain`
   - Admin/governance: `consflow-admin-governance-domain`
   - Reporting/analytics: `consflow-reporting-analytics-domain`
   - Backend routes: `backend/internal/core/routes.go`
   - Backend handlers/service/repository: `backend/internal/core`
   - Shared backend behavior: `backend/internal/shared`
   - Migrations: `backend/migrations`
   - Frontend route: `frontend/src/app/router.tsx`
   - Frontend module: `frontend/src/modules/<feature>`
   - Shared UI/API/types: `frontend/src/components`, `frontend/src/lib`
8. State the verification plan before substantial edits.
9. Pair with the narrower layer skill and the business domain skill that owns the invariants.

## Architecture Guardrails

- Keep backend behavior in handler -> service -> repository flow unless local code clearly uses a tighter path.
- Keep API response envelopes compatible with `frontend/src/lib/api/client.ts`.
- Add migrations for schema changes; do not edit an already-applied migration.
- Keep `admin` as the Docker service/container for the frontend.
- Preserve `make up` and `make restart` as `down -> build -> run`.
- Avoid adding abstractions until they remove real duplication or isolate a stable boundary.
- Prefer the narrow production-grade domain skills over the legacy broad `consflow-pipeline-domain` when routing new work.

## Output Expectations

- Name the files that own the requested behavior.
- Name the selected domain skill and why it owns the invariant.
- Call out overlap/collision findings when `consflow-impact-analysis` applies.
- Call out API contract changes explicitly.
- List the commands needed to verify the change.
- Mention residual risks when a full Docker or browser check was not run.
