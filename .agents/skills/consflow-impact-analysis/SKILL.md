---
name: consflow-impact-analysis
description: Analyze Consflow feature overlap, logic collision, cross-domain impact, shared state risks, API/frontend contract drift, status transition conflicts, RBAC/audit/report consequences, and downstream module effects before implementing or reviewing feature, bugfix, or behavior changes.
---

# Consflow Impact Analysis

Use this skill when a Consflow task may overlap existing features, mutate shared state, change status transitions, affect more than one module, or introduce cross-domain logic consequences.

## Required Inputs

- The requested feature, bugfix, review target, or behavior change.
- The matched feature owner from `consflow-module-memory`.
- The relevant domain skill maps for any affected business domain.

## Required Workflow

1. Read `.agents/skills/consflow-module-memory/references/module-keyword-map.md` and identify the primary feature owner, likely owner files, routes, tables, migrations, shared utilities, and domain checks.
2. Read the relevant `.agents/skills/consflow-*-domain/references/domain-map.md` files for every affected domain. Prefer narrow production-grade domain skills over `consflow-pipeline-domain`.
3. Enumerate overlap surfaces:
   - frontend routes, modules, dialogs, shared components, status labels, API client usage
   - backend routes, handlers, services, repositories, middleware, response envelopes
   - SQL tables, migrations, seed data, soft-delete or active-row filters
   - shared workflow/status/action names and permission checks
   - reports, dashboard cards, CSV exports, audit/activity streams, notifications
4. Trace logic collisions:
   - source state -> target state transitions
   - template definitions vs project runtime copies
   - user-visible action readiness vs backend authorization
   - derived balances, reconciliations, report metrics, and blockers
   - actor, timestamp, comment, evidence, signature, and history requirements
5. Decide whether the task is safe to implement as scoped, needs a split, or needs review clarification. Keep the human review gates from `AGENTS.md`.
6. During implementation or review, verify that every changed contract is matched across backend, frontend, shared types/status utils, migrations, tests, and reports/exports when relevant.

## Collision Checklist

- Does another module already own this action, table, status, route, or UI surface?
- Can the change create duplicate sources of truth or conflicting derived data?
- Will a backend state transition expose an unsupported frontend action or stale status label?
- Will reports, dashboard metrics, CSV exports, audit logs, or activity feeds become inconsistent?
- Does RBAC/project visibility still match between UI affordances and protected endpoints?
- Are soft-deleted, cancelled, inactive, or historical records still filtered correctly?
- Are immutable historical records, published form versions, template/runtime copies, stock movements, financial reconciliations, and acceptance history preserved?

## Output Contract

When using this skill, produce an impact matrix before implementation or as the first section of a review:

| Area | Existing owner | Potential collision | Required check | Result |
|---|---|---|---|---|

Then state:

- affected features and downstream modules checked
- shared tables/routes/components/contracts touched
- status/RBAC/audit/report/export consequences
- missing tests, missing audit, or unresolved risk
- recommendation: safe as scoped, split the work, or ask for clarification
