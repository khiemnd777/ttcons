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
3. If the request is a feature, bugfix, or code-changing task, prepare a review package and wait for explicit user confirmation before implementation.
4. If the request changes UI or visual interface, prepare a "Mockup as Text" and wait for explicit user confirmation before implementation. "Mockup as Text" means an ASCII/wireframe-style visual mockup in text, not only a prose UX specification.
5. Identify the ownership path before editing:
   - Module/feature keyword cache: `.agents/skills/consflow-module-memory/references/module-keyword-map.md`
   - Backend routes: `backend/internal/core/routes.go`
   - Backend handlers/service/repository: `backend/internal/core`
   - Shared backend behavior: `backend/internal/shared`
   - Migrations: `backend/migrations`
   - Frontend route: `frontend/src/app/router.tsx`
   - Frontend module: `frontend/src/modules/<feature>`
   - Shared UI/API/types: `frontend/src/components`, `frontend/src/lib`
6. State the verification plan before substantial edits.
7. Pair with the narrower skill that owns the implementation.

## Architecture Guardrails

- Keep backend behavior in handler -> service -> repository flow unless local code clearly uses a tighter path.
- Keep API response envelopes compatible with `frontend/src/lib/api/client.ts`.
- Add migrations for schema changes; do not edit an already-applied migration.
- Keep `admin` as the Docker service/container for the frontend.
- Preserve `make up` and `make restart` as `down -> build -> run`.
- Avoid adding abstractions until they remove real duplication or isolate a stable boundary.

## Output Expectations

- Name the files that own the requested behavior.
- Call out API contract changes explicitly.
- List the commands needed to verify the change.
- Mention residual risks when a full Docker or browser check was not run.
