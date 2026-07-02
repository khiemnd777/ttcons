---
name: consflow-admin-governance-domain
description: Protect Consflow administration and governance workflows across auth sessions, refresh/revoke lifecycle, staff, users, roles, permissions, RBAC assignment matrix, project-scoped permission checks, partner catalog, core master data, settings, audit logs, and permission-aware navigation. Use when tasks mention admin, quan tri, staff, user, role, permission, RBAC, phan quyen, auth session, refresh token, audit, settings, partner, core data, catalog, or master data.
---

# Consflow Admin Governance Domain

Use this skill for system governance, identity, permissions, and master data.

## Required Reading

- `AGENTS.md`
- `.agents/skills/consflow-module-memory/references/module-keyword-map.md`
- `references/domain-map.md`
- Pair with `consflow-backend-workflow` for permission helpers and `consflow-frontend-workflow` for permission-aware navigation/actions.

## Workflow

1. Identify actor, auth/session behavior, global permission, project permission, role assignment, staff/user record, partner record, settings key, or audit target.
2. Preserve protected route placement under `middleware.AuthRequired`.
3. Keep backend permission checks and frontend route/action visibility aligned.
4. For partners/master data, preserve active filtering, allowed partner types, and snapshot fields on downstream project/material/procurement/accounting/invoice records.
5. Preserve audit logs for domain transitions and admin changes where local patterns exist.
6. For feature, bugfix, code-changing, or UI-changing requests, stop before editing and produce the repo review package. Include current contract/invariant, evidence, root cause or unresolved risk, clarification questions, implementation plan, validation plan, and any required Mockup as Text. Wait for explicit user confirmation.

## Guardrails

- Do not rely on frontend hiding for authorization.
- Do not loosen project-scoped permission checks into global checks.
- Do not mutate partner snapshots unexpectedly when catalog data changes.
- Do not expose inactive/deleted catalog records in user-visible lists.

## Output Expectations

- State affected governance area: auth, staff, RBAC, permission, partner catalog, settings, or audit.
- Name backend permission helper and frontend permission gate checked.
- Call out any migration/seed changes for permissions or master data.
- Report focused validation and skipped browser checks.
