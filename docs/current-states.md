# Current States

## Work Item Support Workspace Retouch

Status: complete

Will be done:
- Refactor only the `THÔNG TIN HỖ TRỢ` area inside the work item detail workspace.
- Keep backend routes, API response contracts, mutations, and query invalidation behavior unchanged.
- Retouch the support shell so evidence, activity, and workflow form/PDF share a clearer workspace hierarchy.
- Make evidence list/gallery-first with compact upload controls.
- Make activity timeline-first with a compact composer entry point.
- Make workflow form/PDF summary-first with validation, form fields, signature, and PDF artifacts separated clearly.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `Approve`.
- Re-read Consflow frontend workflow skill, AGENTS.md, router, UI guardrails, current plan notes, and target work item components.
- Refactored `WorkItemSupportingTabs` into a clearer support workspace shell with status-aware tabs and a next support action.
- Changed work-item activity support to timeline-first with a compact composer toggle.
- Changed work-item evidence support to summary/list-first and moved upload into a compact expandable panel.
- Added a list variant to `EvidenceGallery` while preserving the existing card default for project files.
- Reworked `WorkflowFormPanel` into an action center, processing summary, form fields section, artifacts column, and form event panel.
- Removed the old unused workflow form summary helper after replacing it with the new summary layout.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Workflow Detail Workspace UI

Status: complete

Will be done:
- Redesign `WorkItemDetailDialog` into a clearer workflow detail workspace with a left workflow-position rail, central current-step content, and right action panel.
- Keep existing work item actions, evidence, activity, workflow form, and domain-specific panels without changing backend contracts.
- Move competing footer actions into a single action hierarchy with one primary CTA and secondary actions.
- Add or reuse shared sizing/layout support needed by the dialog.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved implementation with `Làm đi` after reviewing text and image mockups.
- Read Consflow frontend and workflow orchestration skills, UI guardrails, router, current work item detail implementation, and existing dirty diffs.
- Added a `workspace` dialog size for wide operational detail layouts.
- Reworked `WorkItemDetailDialog` into a 3-zone layout: workflow position rail, current-step panel, and right-side action panel.
- Preserved existing APIs and domain panels; no backend contract or route change was introduced.
- Ran `cd frontend && npm run build` successfully.
- Skipped browser verification per repository rule because it was not explicitly requested in this turn.
- Tuned `Vị trí workflow` so it no longer stretches to the body height; it is sticky on wide viewports with internal timeline scrolling.

## Full Workflow Demo Seed

Status: complete

Will be done:
- Extend `backend/seed/seed.sql` with a full demo project that covers all current work item types and workflow form templates.
- Seed a runtime pipeline with nodes, steps, work items, workflow form instances, answers, evidence, activities, approvals, and domain records.
- Seed operational data for material snapshots, surplus handling, incoming/outgoing acceptance, subcontractor/client reconciliation, and finance/debt panels.
- Validate seed syntax and run the nearest feasible backend checks without browser verification.

Done:
- Review package approved by the user with `OK triển khai`.
- Confirmed this is a seed/demo-data change only; no schema, API, or UI contract change is intended.
- Added full workflow/form coverage seed data in `backend/seed/seed.sql`.
- Added seed upload fixtures for workflow form PDF, input invoice PDF, and output invoice PDF.
- Updated `backend/Makefile` so `make seed` copies seed upload fixtures into the configured upload directory/container volume.
- Validated `backend/seed/seed.sql` against the running Postgres container inside a rollback transaction.
- Applied the seed to the running local Docker Postgres with `cd backend && make seed`.
- Ran `cd backend && make test` successfully.

## Project-First Folder UI

Status: complete

Will be done:
- Make `/projects` the primary post-login entry for users with `project.view`.
- Reduce the main sidebar to high-level entry points instead of exposing every project domain module as a global navigation item.
- Convert `frontend/src/modules/projects/ProjectsPage.tsx` from a dense table into a folder/tree explorer for construction projects.
- Convert the project detail overview in `frontend/src/modules/projects/ProjectDetailPage.tsx` to a compact project operations summary that sits inside the project workspace layout.
- Add project focus mode for `/projects/:id/*` so the project business tree becomes the main menu and the global app sidebar/header is hidden.
- Keep existing project-scoped routes and backend APIs unchanged.
- Run `cd frontend && npm run build` after frontend edits.

Done:
- User approved the visual mockup direction: `/projects` as project explorer and project detail as a folder-style business tree.
- Updated `HomeRedirect` so users with `project.view` land on `/projects`.
- Reduced the main sidebar to project-first high-level navigation and removed the project domain modules from the primary global menu.
- Reworked `ProjectsPage` into a searchable/filterable folder tree explorer with expandable construction project rows.
- Reworked `ProjectDetailPage` overview into a folder-based operations workspace with a persistent business tree and compact work table.
- Added `ProjectWorkspaceLayout` focus mode for `/projects/:id/*` with project top bar, fixed business tree menu, and scoped content outlet.
- Nested all project-scoped routes under `ProjectWorkspaceLayout` while preserving their existing URLs and permission guards.
- Updated `AppShell` so project focus mode hides the global sidebar/header, matching the focused `pipeline-builder` behavior.
- Removed the old embedded business tree from `ProjectDetailPage`; the project workspace layout is now the single owner of that menu.
- Added query-tab support for project members/files links from the workspace tree.
- Replaced user-facing `PM` labels with `Phụ trách` and removed internal `Folder` / `Theo quyền` text from project folder rows.
- Kept backend APIs, routes, migrations, and project-scoped module routes unchanged.
- Ran `cd frontend && npm run build` successfully after both project-first and focus-mode changes.

## Consflow Impact Analysis Skill

Status: complete

Will be done:
- Add `consflow-impact-analysis` as a repo-scoped skill for feature overlap and logic collision analysis.
- Add UI metadata for the new skill.
- Wire the new skill into the Consflow skill README, module routing, and repo architecture workflow.
- Validate the skill artifact with targeted file inspection and `rg`.

Done:
- Review package approved by the user.
- Added `.agents/skills/consflow-impact-analysis/SKILL.md`.
- Added `.agents/skills/consflow-impact-analysis/agents/openai.yaml`.
- Wired `consflow-impact-analysis` into `.agents/skills/README.md`.
- Added impact-analysis pairing guidance to `consflow-module-memory`.
- Added cross-feature collision routing to `consflow-repo-architect`.
- Validated `SKILL.md` frontmatter and `openai.yaml` with Ruby YAML checks because the Python validator is missing `PyYAML` in this environment.
