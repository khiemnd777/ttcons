# [Closed] Consflow Domain Skill Refresh

## Objective

Refresh repo-scoped Codex skills after the production-grade workflow roadmap so future agents route work by durable business domains instead of outdated broad module buckets.

## Will Be Done

- Create domain skills for workflow orchestration, project planning, construction operations, commercial/procurement, materials/warehouse, quality/acceptance/handover, accounting/finance, admin/governance, and reporting/analytics.
- Update existing Consflow skills so broad routing points to the new domain skills where relevant.
- Update module memory and README so prompt keywords map to the current production-grade repo surface.
- Validate agent artifacts with the local `.agents/scripts/validate.sh` script and skill metadata checks.

## Done

- Reviewed the closed production-grade roadmap checklist.
- Reviewed existing repo-scoped skills, route registration, frontend routes/modules, migrations, and domain owners.
- Identified outdated skill coverage around workflow forms, evidence/signature/PDF, construction execution, acceptance/handover, accounting/AP/AR/reports, partner catalog, auth sessions, RBAC, audit, and settings.
- Created 9 production-grade business domain skills under `.agents/skills`.
- Added `references/domain-map.md` to each new domain skill for owner paths, routes, tables/migrations, and invariants.
- Updated `.agents/skills/README.md` with the new domain skill catalog.
- Updated `consflow-module-memory` and `module-keyword-map.md` to route prompt keywords to the new domain skills.
- Updated `consflow-repo-architect` to identify business domain skill ownership before edits.
- Updated `consflow-pipeline-domain` as a legacy broad guardrail that delegates to narrower production-grade domain skills.
- Updated `consflow-handoff` suggested skills list with the new domain skills.
- `bash .agents/scripts/validate.sh` passed after the skill refresh.
- `git diff --check` passed after the skill refresh.
- A shell frontmatter check passed for all repo skills.
- `quick_validate.py` could not run because both system Python and bundled Codex Python lack the `yaml` module.
