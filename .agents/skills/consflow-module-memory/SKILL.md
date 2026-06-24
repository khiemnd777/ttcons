---
name: consflow-module-memory
description: Map Consflow prompt keywords and aliases to the right modules, features, frontend pages, backend routes, handlers, services, repositories, SQL tables, migrations, shared utilities, and domain risks. Use when the user mentions Consflow feature keywords such as cong trinh, project, pipeline, template, connector, work item, activity, approval, BOQ, du toan, procurement, mua hang, warehouse, kho, vat tu, acceptance, nghiem thu, rework, replanning, handover, ban giao, accounting, cong no, staff, RBAC, audit, reports, settings, or asks agents to find modules/features/utilities quickly.
---

# Consflow Module Memory

Use this skill before broad repo scans when a prompt contains module, feature, domain, route, table, or utility keywords.

## Workflow

1. Read `references/module-keyword-map.md`.
2. Treat `AGENTS.md`, this skill, and `.agents/skills/**/references/*.md` as durable agent memory. Treat `docs/current-states.md` as temporary active-plan memory and `docs/plan-closed/*` as optional historical context only.
3. If a closed plan contains durable module, feature, invariant, or impact knowledge, promote the useful part into `references/module-keyword-map.md` or the relevant domain map through the normal review gate before depending on it.
4. Match the user's keywords to the closest feature owner and aliases.
5. Start with the listed owner files, then use targeted `rg` searches inside those paths.
6. If the cache conflicts with local code, trust local code and update the cache as part of the change when appropriate.
7. For feature, bugfix, code-changing, or UI-changing work, keep the human review gates from `AGENTS.md`.
8. Pair with `consflow-impact-analysis` when the request may overlap existing features, mutate shared state, change status transitions, affect reports/exports, or cross backend/frontend/domain boundaries.
9. Pair with the narrower Consflow skill after routing:
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

## Output Contract

When using this memory, name the matched feature, the likely owner files, and the first targeted searches to run before proposing or editing code.
