# Consflow Agent Guide

## Repository Expectations

- Treat this repository as a full-stack construction workflow system: Go/Fiber API, PostgreSQL, Redis, custom SQL migrations, React/Vite admin UI, and Docker Compose.
- Read the local code before deciding where to change behavior. Use `rg` and targeted file reads first.
- Keep changes scoped to the requested module. Do not rewrite unrelated modules or generated artifacts.
- Follow SOLID and DRY. Prefer shared components, helpers, and narrow abstractions when a UI, validation, toast, query, or action pattern repeats across modules.
- For every feature request, bugfix request, or code-changing request, first present a review package and wait for explicit user confirmation before implementation.
- For every UI or visual-interface change, first produce a "Mockup as Text" for user review and wait for explicit confirmation before implementation. "Mockup as Text" means a visual mockup represented in text, not a prose UX specification.
- Never use Flyway. Database changes use the custom migration runner in `backend/internal/shared/migrations` and SQL files in `backend/migrations`.
- Do not revert user edits or run destructive git commands unless the user explicitly asks.
- Use `apply_patch` for manual source edits.
- Do not launch or run Browser, browser-use, Playwright, or headless browser verification unless the user explicitly requests it in that turn. For frontend changes, use non-browser checks by default and report browser verification as skipped.

## Human Review Gates

Agents must stop before implementation when the request would change code, feature behavior, bugfix behavior, or UI.

For feature, bugfix, or code-changing requests, provide a review package with:

- problem summary
- files likely to change
- proposed behavior
- implementation outline
- validation plan
- risks or open questions

Then wait for an explicit confirmation from the user before editing code.

For UI or interface-changing requests, provide a "Mockup as Text" before implementation. It must be a textual visual mockup: use ASCII/wireframe-style blocks to show the actual screen layout, hierarchy, controls, labels, spacing, and states as the user would see them. Keep notes short and secondary. Do not replace the mockup with only bullet-point descriptions.

It should show:

- screen or component affected
- layout structure
- visible controls and labels
- states, empty states, loading states, and error states
- primary user flow
- responsive behavior when relevant

Then wait for explicit confirmation from the user before editing UI code.

## Repo Map

- `backend/cmd/server`: API entrypoint.
- `backend/cmd/migrate`: standalone custom migration command.
- `backend/internal/core`: routes, handlers, service logic, repository helpers, domain types.
- `backend/internal/shared`: config, DB, cache, middleware, response envelope, migration runner.
- `backend/migrations`: ordered SQL migrations, normally named `V<N>__description.sql`.
- `backend/seed`: demo data.
- `frontend/src/app`: React Router setup.
- `frontend/src/layouts`: authenticated app shell and navigation.
- `frontend/src/lib`: API client, auth context, shared domain types and utilities.
- `frontend/src/modules`: admin feature screens.
- `frontend/src/components`: shared UI primitives.
- `.agents/skills`: repo-scoped Codex skills.
- `.codex/agents`: project-scoped custom subagents.

## Commands

- Full Docker pipeline: `make up`
- Stop stack: `make down`
- Restart pipeline: `make restart`
- Logs: `make log` or `make log SERVICE=admin`
- Backend tests: `cd backend && make test`
- Backend format: `cd backend && make fmt`
- Backend migration run against local DB: `cd backend && make migrate`
- Seed demo data: `cd backend && make seed`
- Frontend build: `cd frontend && npm run build`

`make up` and `make restart` intentionally run `down -> build -> run`. Keep that sequence intact.

## Backend Rules

- Route registration lives in `backend/internal/core/routes.go`.
- HTTP handlers should use `response.OK`, `response.Created`, and `response.Fail` from `backend/internal/shared/response`.
- Keep request parsing in handlers and business transitions in `Service`.
- Keep SQL access through repository helpers unless a very small direct query is already the local pattern.
- When adding or changing tables, add a new migration rather than editing an applied migration.
- The migration runner records checksums in `schema_migrations`; changing an applied migration will break future runs.
- User-visible list endpoints must return records that are not deleted. Use each table's local marker, such as `status <> 'cancelled'`, `status <> 'deleted'`, or `active = true`, and do not expose soft-deleted rows in admin lists.
- Preserve auth and actor semantics. Protected endpoints should stay under `middleware.AuthRequired`.

## Frontend Rules

- Add route entries in `frontend/src/app/router.tsx`.
- Put feature pages under `frontend/src/modules/<feature>`.
- Reuse `frontend/src/components` before adding new UI primitives.
- Create, view-detail, and edit interactions must open a shared dialog/modal component. Avoid one-off drawer or full-page implementations unless the feature is a specialized canvas/editor with explicit UX needs.
- Save/edit flows must show toast feedback for success, warning, and error states. Toasts must include an immediate close `(x)` button.
- Required and invalid fields must be highlighted inline, and the warning toast must summarize required/invalid fields.
- Use the API helpers in `frontend/src/lib/api/client.ts`; do not duplicate fetch/token logic in feature modules.
- Keep UI operational and dense. This is an admin tool, not a marketing page.
- After frontend changes, run `cd frontend && npm run build` when feasible.

## Domain Rules

- Pipeline templates define reusable workflows. Pipeline runs are project-specific runtime copies.
- Publishing and cloning templates must not mutate historical project runs unexpectedly.
- Approval actions should preserve auditability: actor, status, comments, and timestamps matter.
- BOQ approval can generate procurement. Do not bypass approval states when connecting BOQ to procurement.
- Acceptance failures can create rework. Rework completion should not silently skip acceptance history.
- Warehouse stock operations must preserve material balance and movement traceability.

## Review Guidelines

- Prioritize correctness, authorization/RBAC, migration safety, financial/stock consistency, and missing regression tests.
- Check backend/frontend contract drift whenever an API response shape changes.
- Check Docker and Makefile behavior when ports, service names, migrations, or startup order change.
- Treat broad refactors as risky unless they directly support the requested behavior.

## Agent Artifacts

- Repo skills live in `.agents/skills/<name>/SKILL.md` with optional `agents/openai.yaml` metadata.
- Project custom agents live in `.codex/agents/<name>.toml`.
- Runtime sync is not automatic. These files are the repo source of truth.
