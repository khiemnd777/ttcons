---
name: consflow-module-memory
description: Mandatory Consflow mapping bootstrap. Map prompt keywords and aliases to modules, features, functions, frontend pages, UI surfaces, backend routes, handlers, services, repositories, SQL tables, migrations, shared utilities, helpers, triage search recipes, and domain risks before broad scans, diagnosis, review, or code changes.
---

# Consflow Module Memory

Use this skill as the mandatory mapping bootstrap before broad repo scans, diagnosis, review, implementation planning, or code edits in Consflow.

## Mandatory Bootstrap Contract

- Read `references/module-keyword-map.md` first for every Consflow repository task that touches or reasons about source behavior.
- Load targeted detail maps after the central map:
  - Backend/API/schema/function work: `references/backend-function-map.md`.
  - Frontend/UI/helper/API usage work: `references/frontend-ui-utility-map.md`.
  - Bug, regression, slow flow, failed validation, stale data, or root-cause work: `references/root-cause-triage-map.md`.
  - Business-domain work: the relevant `.agents/skills/consflow-*-domain/references/domain-map.md`.
- Do not declare current behavior, root cause, owner files, or implementation scope until the relevant map and local source have both been checked.
- Mapping is mandatory, but it is not authoritative over code. If mapping conflicts with local code, trust code and update mapping when the correction is durable.

## Workflow

1. Read `references/module-keyword-map.md`.
2. Read `references/backend-function-map.md`, `references/frontend-ui-utility-map.md`, `references/root-cause-triage-map.md`, and domain maps only when the task matches those surfaces.
3. Treat `AGENTS.md`, this skill, and `.agents/skills/**/references/*.md` as durable agent memory. Treat `docs/current-states.md` as temporary active-plan memory and `docs/plan-closed/*` as optional historical context only.
4. If a closed plan contains durable module, feature, invariant, or impact knowledge, promote the useful part into `references/module-keyword-map.md` or the relevant domain map through the normal review gate before depending on it.
5. Match the user's keywords to the closest feature owner, aliases, UI surface, route, table, function group, utility/helper, and root-cause recipe.
6. Start with the listed owner files, then use targeted `rg` searches inside those paths.
7. If the cache conflicts with local code, trust local code and update the cache as part of the change when appropriate.
8. For feature, bugfix, code-changing, or UI-changing work, keep the human review gates from `AGENTS.md`; routing memory is not permission to edit. Bugfixes still need evidence, root cause or unresolved risk, clarification questions, implementation plan, validation plan, and explicit user confirmation.
9. Pair with `consflow-impact-analysis` when the request may overlap existing features, mutate shared state, change status transitions, affect reports/exports, or cross backend/frontend/domain boundaries.
10. Pair with the narrower Consflow skill after routing:
   - Backend/API/schema: `consflow-backend-workflow`.
   - Frontend/admin UI: `consflow-frontend-workflow`.
   - Workflow orchestration, approvals, workflow forms, work item gates: `consflow-workflow-orchestration-domain`.
   - Projects, planning reports, meeting minutes, planning checklists, activity: `consflow-project-planning-domain`.
   - Field execution, operations, evidence, construction forms: `consflow-construction-operations-domain`.
   - BOQ, unit price, procurement, reconciliation: `consflow-commercial-procurement-domain`.
   - Materials, warehouse, stock, site balances, surplus: `consflow-materials-warehouse-domain`.
   - Acceptance, rework, replanning, handover: `consflow-quality-acceptance-handover-domain`.
   - Accounting, AP/AR, invoices, payments, finance reports: `consflow-accounting-finance-domain`.
   - Auth, staff, RBAC, partners, audit, settings: `consflow-admin-governance-domain`.
   - Dashboard, reports, KPI, CSV exports: `consflow-reporting-analytics-domain`.
   - Docker/Makefile/runtime: `consflow-docker-ops`.
   - Diagnosis: `consflow-diagnose`.

## Mapping Maintenance Gate

After any approved code change, check whether touched files changed any mapped surface:

- feature/module ownership
- route, handler, service, repository function, request/response type, SQL table, migration, seed data
- frontend route, page, dialog, panel, component, hook, shared helper, API usage, domain type, status/enum label
- permission/RBAC behavior, audit/activity behavior, report/export behavior, domain invariant
- root-cause triage keyword or search recipe
- agent workflow or subagent behavior

If yes, update `module-keyword-map.md`, the relevant detail map, and any affected domain map in the same work turn. If no mapping update is needed, state why in the final response or review notes.

## Output Contract

When using this memory, name:

- matched feature/domain/function group/UI surface
- mapping files loaded
- likely owner files
- first targeted searches to run
- whether the mapping maintenance gate is expected to apply

Do this before proposing, reviewing, diagnosing, or editing code.
