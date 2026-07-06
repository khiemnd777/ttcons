# Current States

## Work Item Remove Workflow Position Rail

Status: complete

Will be done:
- Remove the left `Vị trí workflow` rail from the work item detail body.
- Keep `Node`, `Bước`, `Minh chứng`, and `Biểu mẫu` in the existing `Quy trình đang chạy` header stats.
- Change the work item detail body from three columns to two columns: main content and right `Việc cần làm`.
- Do not add a breadcrumb replacement.
- Preserve workflow actions, required gates, evidence, activity, forms, backend APIs, query invalidation, and runtime workflow semantics.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `OK`.
- Re-read Consflow frontend workflow and workflow orchestration skills and target work item detail code.
- Removed the `WorkItemProgressRail` render from the work item detail body.
- Removed the unused `WorkItemProgressRail` component implementation.
- Changed the work item detail body grid from three columns to two columns: main content plus right `Việc cần làm`.
- Kept `Node`, `Bước`, `Minh chứng`, and `Biểu mẫu` in the existing `Quy trình đang chạy` header stats.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Workflow Position Breadcrumb

Status: reverted

Before change snapshot:
- User screenshot: `Microsoft Edge Appshot 2026-07-06T10-39-01.914Z.png`.
- Work item detail body is a three-column grid: left `Vị trí workflow`, center current step/support content, right `Việc cần làm`.
- Left `Vị trí workflow` rail is rendered by `WorkItemProgressRail` in `frontend/src/modules/work-items/WorkItemDetailDialog.tsx`.
- `WorkItemProgressRail` displays four vertical items: `Công trình`, `Node quy trình`, `Bước`, `Đầu việc`.
- `WorkItemWorkspaceHeader` currently shows `Quy trình đang chạy` with title, status/project/priority chips, context stats for `Node`, `Bước`, `Minh chứng`, `Biểu mẫu`, and next action.

Attempted change:
- Remove the left `Vị trí workflow` column from the work item detail body.
- Move the workflow position information into a compact breadcrumb inside the `Quy trình đang chạy` header.
- Change the work item detail body from three columns to two columns: main content and right `Việc cần làm`.
- Preserve workflow actions, required gates, evidence, activity, forms, backend APIs, query invalidation, and runtime workflow semantics.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Attempt done:
- User approved implementation and explicitly requested preserving the before-change reference for comparison.
- Re-read Consflow frontend workflow and workflow orchestration skills, AGENTS.md, current state notes, and target work item detail code.
- Removed the `WorkItemProgressRail` left column from the work item detail body.
- Changed the work item detail body grid from three columns to two columns: main content plus right `Việc cần làm`.
- Added compact `Vị trí workflow` breadcrumb inside `Quy trình đang chạy` with `Công trình > Node > Bước > Đầu việc`.
- Kept `Minh chứng` and `Biểu mẫu` as header stats after moving `Node` and `Bước` into the breadcrumb.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

Revert done:
- User requested `Revert lại đoạn vừa rồi đi`.
- Restored the three-column work item detail body with left `Vị trí workflow`, center content, and right `Việc cần làm`.
- Restored `WorkItemProgressRail` and removed the compact breadcrumb from `Quy trình đang chạy`.
- Restored header stats for `Node`, `Bước`, `Minh chứng`, and `Biểu mẫu`.
- Kept the earlier sticky offset fix for the restored left rail: `xl:top-[5.5rem]`.
- Ran `cd frontend && npm run build` successfully after the revert.

## Work Item Sticky Side Panel Offset

Status: complete

Will be done:
- Retouch only sticky positioning for the side panels inside `frontend/src/modules/work-items/WorkItemDetailDialog.tsx`.
- Move `Vị trí workflow` and `Việc cần làm` sticky panels below the sticky project top bar so their headers remain visible while scrolling.
- Keep the existing three-column work item layout, workflow actions, evidence/forms/activity behavior, backend APIs, query invalidation, and workflow gates unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `OK`.
- Re-read Consflow frontend workflow and diagnose skills, current state notes, and target `WorkItemDetailDialog` sticky panel code.
- Updated both work item side panels from `xl:top-0` to `xl:top-[5.5rem]` so their headers sit below the sticky project top bar.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Evidence Search Row Layout

Status: complete

Will be done:
- Retouch only the search/filter layout inside `frontend/src/modules/work-items/EvidenceGallery.tsx`.
- Move the `Tìm minh chứng, đầu việc, mô tả` search input to its own full-width row above the evidence filters.
- Keep the evidence filter segmented buttons below the search input.
- Keep evidence data, upload, review, search/filter behavior, backend APIs, query invalidation, and work item workflow gates unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `làm đi`.
- Re-read Consflow frontend workflow skill and target `EvidenceGallery` code.
- Moved the evidence search input to its own full-width row above the filter group.
- Kept the evidence filter segmented buttons below search without the previous desktop two-column split.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Evidence Filter Affordance

Status: complete

Will be done:
- Retouch only the evidence filter row inside `frontend/src/modules/work-items/EvidenceGallery.tsx`.
- Make the `Tất cả`, `Ảnh`, `Tài liệu`, status filters read as interactive filter controls instead of passive chips or labels.
- Rename the `submitted` filter label from `Đã gửi` to `Đang chờ` to match the current `draft/submitted/pending` filter logic.
- Keep evidence data, upload, review, search, backend APIs, query invalidation, and work item workflow gates unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `approve`.
- Re-read Consflow frontend workflow skill, UI guardrails, current plan notes, and target `EvidenceGallery` code.
- Added a visible `Lọc minh chứng` filter group around the evidence filter controls.
- Changed evidence filter buttons to clearer segmented filter controls with active state, active icon, `aria-pressed`, and count badges.
- Renamed the `submitted` filter label from `Đã gửi` to `Đang chờ` while keeping the existing `draft/submitted/pending` filter behavior unchanged.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Detail Project Workspace Route

Status: complete

Will be done:
- Add a project-scoped work item detail route under `/projects/:projectId/work-items/:workItemId` so the detail page stays inside the project workspace shell.
- Keep `/work-items/:id` as a fallback route for callers that do not have project context.
- Update work item detail navigation to prefer the project-scoped route when `project_id` is available.
- Remove standalone work item focus mode from `AppShell`; project workspace focus mode remains responsible for project-scoped detail pages.
- Keep backend routes, API contracts, workflow gates, form/evidence/signature/PDF behavior, and query invalidation unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the correction with `Approve`.
- Re-read Consflow frontend/workflow orchestration skills, route shell, current work item detail page, navigation helper, and current work item entry points.
- Added the nested project workspace route `/projects/:id/work-items/:workItemId`.
- Updated `WorkItemDetailPage` to resolve either nested `workItemId` or fallback top-level `id` and to return to the project overview when no `returnTo` is supplied.
- Updated work item navigation links from project detail, my work, planning runtime, planning daily reports, meeting minutes, and planning checklists to pass project context.
- Removed the standalone work item focus mode from `AppShell`; project-scoped work item detail now relies on project focus mode.
- Kept `/work-items/:id` as a fallback route for callers without project context.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Detail Full Page

Status: complete

Will be done:
- Add a full-page work item detail route at `/work-items/:id`.
- Reuse the existing work item detail workspace so dialog and page behavior stay consistent without duplicated domain logic.
- Add return navigation so callers can open the page and return to the original screen.
- Convert current work item open actions from modal state to route navigation where feasible.
- Keep backend routes, API contracts, mutations, query invalidation, and workflow domain gates unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User selected implementation direction 2: open work item detail as a page and back to the caller.
- Re-read Consflow frontend/workflow orchestration skills, router, dialog shell, current work item dialog, and current callsites.
- Added a reusable work item detail workspace extracted from the dialog body.
- Added `/work-items/:id` page route shell with a `Quay lại` action.
- Added a small helper for work item detail links with safe `returnTo` paths.
- Converted project detail, planning runtime, my work, daily reports, meeting minutes, and planning checklists work-item open actions to navigate to the full-page detail route.
- Added work item focus mode in `AppShell` so `/work-items/:id` uses full page width without the global shell/sidebar.
- Preserved the existing dialog wrapper as a reuse path, but current work item entry points now open the page route.
- Kept backend APIs, workflow actions, form/evidence/signature/PDF mutations, and query invalidation behavior inside the shared detail workspace unchanged.
- Ran `cd frontend && npm run build` successfully after the route/page changes and again after enabling work item focus mode.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Main And Support Single Column Retouch

Status: complete

Will be done:
- Refactor only the `Nội dung chính` and `Thông tin hỗ trợ` areas in the work item detail workspace.
- Keep the overall work item dialog layout unchanged: workflow rail, central content, and right action panel stay as-is.
- Make `WorkflowFormPanel` stack form summary, fields, signature/PDF, and history in one column.
- Make workflow form fields full width inside `Nội dung chính`.
- Change `Thông tin hỗ trợ` selectors from card-like buttons into a proper tab strip.
- Keep backend routes, API contracts, mutations, query invalidation, and domain panels unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User clarified the correct scope: `Nội dung chính` and `Thông tin hỗ trợ`.
- User approved implementation with `làm đi`.
- Re-read Consflow frontend/workflow orchestration skills, current state notes, and target work item UI code.
- Refactored `WorkflowFormPanel` so the workflow form summary, fields, signature/PDF, and events stack in one column.
- Removed the two-column field grid inside `Nội dung chính` so workflow form controls use full width.
- Refactored `WorkItemSupportingTabs` so the support header stacks in one column.
- Replaced card-like support tab buttons with a compact tab strip using tab semantics.
- Kept the overall work item dialog layout, backend APIs, mutations, and query invalidation unchanged.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

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
