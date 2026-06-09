---
name: consflow-frontend-workflow
description: Implement or review Consflow React/Vite admin UI work, including routes, modules, shared components, API calls, auth, forms, tables, and operational dashboard screens.
---

# Consflow Frontend Workflow

Use this skill for the React admin frontend.

## Required Reading

- `AGENTS.md`
- `frontend/src/app/router.tsx`
- `frontend/src/layouts/AppShell.tsx` when navigation changes
- `frontend/src/lib/api/client.ts` when API calls change
- `frontend/src/lib/types/domain.ts` when data shapes change
- The target module under `frontend/src/modules`

## Implementation Workflow

1. Identify whether the work is a route, module screen, shared component, API contract, or auth flow.
2. If the request changes UI or visual interface, produce a "Mockup as Text" and wait for explicit user confirmation before implementation.
3. If the request is a feature, bugfix, or code-changing task, present a review package and wait for explicit user confirmation before implementation.
4. Reuse existing components before creating new primitives.
5. Keep feature code in `frontend/src/modules/<feature>`.
6. Use the shared API client helpers: `get`, `post`, `put`, `del`.
7. Do not duplicate token storage or response envelope parsing.
8. Keep admin screens dense, scannable, and operational.
9. Check responsive text and table behavior for small screens when changing layout.

## UI Guardrails

See `references/ui.md` for local UI expectations.

## Verification

```bash
cd frontend && npm run build
```

For Docker-integrated validation:

```bash
make restart
make log SERVICE=admin
```

If a browser check is needed, open `http://localhost:5173`.

## Output Expectations

- Name routes/modules/components changed.
- State any backend API dependency.
- Report build or browser verification results.
