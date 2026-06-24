# Consflow Skills

Repo-scoped skills in this directory:

- `consflow-repo-architect`: routes broad or cross-cutting work across backend, frontend, migrations, and Docker.
- `consflow-module-memory`: maps prompt keywords and aliases to Consflow modules, routes, tables, utilities, and domain risks before broad scans.
- `consflow-impact-analysis`: analyzes feature overlap, logic collision, shared state, contract drift, and downstream module effects before risky changes.
- `consflow-backend-workflow`: implements Go/Fiber/PostgreSQL backend changes with custom migrations.
- `consflow-frontend-workflow`: implements React/Vite admin UI changes.
- `consflow-workflow-orchestration-domain`: protects pipeline templates, runtime runs, work items, approvals, workflow forms, signatures, PDFs, and execution gates.
- `consflow-project-planning-domain`: protects projects, planning workspace, planning daily reports, meeting minutes, planning checklists, and project activity.
- `consflow-construction-operations-domain`: protects field execution, operations reports, site issues, work item evidence, and construction checklist/report forms.
- `consflow-commercial-procurement-domain`: protects BOQ, unit price lifecycle, procurement, and quantity/price reconciliation.
- `consflow-materials-warehouse-domain`: protects materials, stock movements, site material balances, material snapshots, and surplus handling.
- `consflow-quality-acceptance-handover-domain`: protects incoming/outgoing acceptance, rework, replanning, and handover readiness.
- `consflow-accounting-finance-domain`: protects budgets, AP/AR, invoices, payment schedules, advances, settlements, reconciliation, cashflow, tax, and profit.
- `consflow-admin-governance-domain`: protects auth sessions, staff, RBAC, permissions, partner catalog, settings, and audit logs.
- `consflow-reporting-analytics-domain`: protects dashboard, operational reports, finance reports, scoped visibility, warnings, and CSV export parity.
- `consflow-pipeline-domain`: legacy broad construction workflow guardrail; prefer the narrower domain skills above when a prompt matches them.
- `consflow-diagnose`: disciplined bug/performance diagnosis loop.
- `consflow-docker-ops`: operates the Docker/Makefile stack safely.
- `consflow-handoff`: writes compact continuation handoffs.

Skill authoring rules:

- Folder name must match frontmatter `name`.
- Description must front-load trigger words because Codex may shorten skill lists.
- Prefer imperative workflow steps.
- Put long examples in `references/`, not in the main `SKILL.md`.
- Do not add scripts unless deterministic execution is needed.

Human review gates:

- Feature, bugfix, or code-changing work must produce a review package and wait for explicit user confirmation before implementation.
- UI or interface-changing work must produce a "Mockup as Text" and wait for explicit user confirmation before implementation. "Mockup as Text" means an ASCII/wireframe-style visual mockup in text, not only a prose UX specification.

Durable memory policy:

- Treat `AGENTS.md`, `.agents/skills/**/SKILL.md`, and `.agents/skills/**/references/*.md` as the durable agent memory surfaces.
- Treat `docs/current-states.md` as temporary active-plan memory only.
- Treat `docs/plan-closed/*` as historical records only. Do not require closed plans for future routing, feature, or bugfix work; promote durable knowledge from them into the skill references instead.
