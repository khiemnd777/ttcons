---
name: consflow-module-memory
description: Map Consflow prompt keywords and aliases to the right modules, features, frontend pages, backend routes, handlers, services, repositories, SQL tables, migrations, shared utilities, and domain risks. Use when the user mentions Consflow feature keywords such as cong trinh, project, pipeline, template, connector, work item, activity, approval, BOQ, du toan, procurement, mua hang, warehouse, kho, vat tu, acceptance, nghiem thu, rework, replanning, handover, ban giao, accounting, cong no, staff, RBAC, audit, reports, settings, or asks agents to find modules/features/utilities quickly.
---

# Consflow Module Memory

Use this skill before broad repo scans when a prompt contains module, feature, domain, route, table, or utility keywords.

## Workflow

1. Read `references/module-keyword-map.md`.
2. Match the user's keywords to the closest feature owner and aliases.
3. Start with the listed owner files, then use targeted `rg` searches inside those paths.
4. If the cache conflicts with local code, trust local code and update the cache as part of the change when appropriate.
5. For feature, bugfix, code-changing, or UI-changing work, keep the human review gates from `AGENTS.md`.
6. Pair with the narrower Consflow skill after routing:
   - Backend/API/schema: `consflow-backend-workflow`.
   - Frontend/admin UI: `consflow-frontend-workflow`.
   - Pipeline, BOQ, procurement, warehouse, acceptance, rework, handover invariants: `consflow-pipeline-domain`.
   - Docker/Makefile/runtime: `consflow-docker-ops`.
   - Diagnosis: `consflow-diagnose`.

## Output Contract

When using this memory, name the matched feature, the likely owner files, and the first targeted searches to run before proposing or editing code.
