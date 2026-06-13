# Consflow Skills

Repo-scoped skills in this directory:

- `consflow-repo-architect`: routes broad or cross-cutting work across backend, frontend, migrations, and Docker.
- `consflow-module-memory`: maps prompt keywords and aliases to Consflow modules, routes, tables, utilities, and domain risks before broad scans.
- `consflow-backend-workflow`: implements Go/Fiber/PostgreSQL backend changes with custom migrations.
- `consflow-frontend-workflow`: implements React/Vite admin UI changes.
- `consflow-pipeline-domain`: protects pipeline, approval, BOQ, procurement, warehouse, acceptance, and rework invariants.
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
