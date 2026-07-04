# Current States

## Client Workflow Official Template Automation

Status: backend automation complete; UI monitoring deferred by user

Will be done:
- Phase 1: monitor/review the newly applied template in the running app and address any client-flow data corrections without mutating the preserved draft template.
- Phase 2: implement backend-only workflow automation events and idempotent hooks for the official client template.
- Phase 2: use existing work item activity/blocker surfaces only; do not add new UI.
- Phase 2: create/link draft domain scaffolds for BOQ procurement, input acceptance/AP, output acceptance/AR, and handover/closure checks without auto-approving, auto-passing acceptance, or auto-paying/receiving money.
- Phase 2: add a new migration after V38; do not edit applied V38.
- Phase 2: do not run seed.
- Phase 3: implement backend-only finance completion scaffolding with no new UI and no seed.
- Phase 3: create idempotent draft payment schedules for AP/AR created by the official workflow automation.
- Phase 3: do not create fake input/output invoice records; use existing activity/blocker surfaces with Vietnamese user-facing messages for real invoice requirements.
- Later phases: UI monitoring/dashboard is deferred because the user explicitly requested backend-only work for Phase 2 and Phase 3.

Done:
- User approved Phase 1 implementation and migration run with `OK triển khai Phase 1 và sau đó chạy migrate nếu có`.
- Read workflow orchestration, backend workflow, database workflow, impact analysis, and relevant domain maps.
- Confirmed the current engine clones template nodes/steps into runtime work items and workflow form instances.
- Confirmed current edge routing has no form-answer condition and chooses the first matching edge by priority.
- Confirmed work item completion already enforces required evidence/workflow form gates.
- Added migration `backend/migrations/V38__client_workflow_template.sql` for the new published official client template, preserving the existing draft template.
- Added the `client_design_branch_decision` workflow form and `workflow_form_answer_equals` edge routing for Có thiết kế / Không thiết kế.
- Modeled `Đầu Vào: Tổ Đội / Thầu Phụ / Cung cấp vật tư` as the mandatory shared gate after both BOQ branches.
- Updated `backend/seed/seed.sql` so reseeding keeps the official client template and branch-decision form after template table truncation.
- Added backend tests for official template shape, branch routing intent, and workflow form answer matching.
- Ran `cd backend && make migrate-check` successfully.
- Ran `cd backend && make test` successfully.
- Rebuilt/restarted the Docker backend with `docker compose -f docker-compose.yml up -d --build backend`; backend auto-migrated V38 with `applied=1 skipped=37`.
- Verified DB state: `schema_migrations` has version 38 `client workflow template`; official template is published with 12 nodes and 52 steps; form-answer branch edges count is 2; Đầu Vào has `mandatory_gate=true`.
- User approved Phase 2 with `OK triển khai Phase 2 backend trước, không làm UI mới, chỉ dùng activity/blocker hiện có. Không chạy seed.`
- Added migration `backend/migrations/V39__workflow_automation_events.sql` for idempotent workflow automation events and updated the official Đầu Vào -> Đàm phán edge to `procurement_approved`.
- Added backend automation hooks when work items/run steps complete for the official client workflow template only.
- Implemented Phase 2 backend automation actions for BOQ -> procurement generation, mandatory input procurement gate, incoming acceptance/AP draft scaffolding, outgoing acceptance/AR draft scaffolding, and handover/financial closure blocker checks.
- Automation records outcomes into `workflow_automation_events`, work item activity, and audit log; blocker outcomes use the existing blocker activity surface.
- Added backend integration tests for blocked BOQ automation idempotency, procurement generation idempotency, official template edge condition, and outgoing acceptance/receivable draft scaffolding.
- Ran `cd backend && make migrate-check` successfully after V39.
- Ran focused workflow automation tests with `cd backend && GOCACHE="$PWD/.gocache" go test ./internal/core -run 'TestClientWorkflowAutomation|TestClientOfficialWorkflow'` successfully.
- Ran full backend tests with `cd backend && GOCACHE="$PWD/.gocache" make test` successfully.
- Rebuilt/restarted the Docker backend with `docker compose -f docker-compose.yml up -d --build backend`; backend auto-migrated V39 with `applied=1 skipped=38`.
- Verified DB state: `schema_migrations` has versions 38 and 39; `workflow_automation_events` exists; official Đầu Vào -> Đàm phán edge is `procurement_approved`.
- Verified API health with `curl -sS http://localhost:8080/api/health`.
- Did not run `make seed`.
- User approved Phase 3 with `OK triển khai Phase 3 chỉ phía máy chủ, không làm giao diện mới, không chạy seed, không tạo hóa đơn giả; tạo lịch thanh toán/thu tiền dạng nháp và ghi blocker tiếng Việt cho hóa đơn thật.`
- Implemented Phase 3 backend-only finance automation: incoming acceptance/AP now creates an idempotent draft payable payment schedule, and outgoing acceptance/AR now creates an idempotent draft receivable payment schedule.
- Kept invoice handling as blockers only: no fake input/output invoice records are created, and the blocker messages for required real invoices are Vietnamese.
- Updated official workflow automation messages that can surface to users to Vietnamese for procurement approval and handover/financial-closure blockers.
- Added backend integration coverage for incoming payment schedule + real invoice blocker and outgoing collection schedule + real invoice blocker idempotency.
- Ran `cd backend && make fmt` successfully.
- Ran focused workflow automation tests with `cd backend && GOCACHE="$PWD/.gocache" go test ./internal/core -run 'TestClientWorkflowAutomation|TestClientOfficialWorkflow'` successfully after Phase 3.
- Ran `cd backend && make migrate-check` successfully after Phase 3.
- Ran `git diff --check` successfully.
- Ran full backend tests with `cd backend && GOCACHE="$PWD/.gocache" make test` successfully after Phase 3.
- Did not run `make seed` for Phase 3.
- Continued execution after the Goal UI had been marked blocked; verified the active Docker stack and DB directly without running seed.
- Verified live DB has 39 applied migrations, including V38 `client workflow template` and V39 `workflow automation events`.
- Verified the official client template and the preserved draft template coexist in DB: official template `40000000-0000-0000-0000-000000000020` is published and draft template `75151291-ae2e-449e-83f7-57999a43baf0` remains draft.
- Created live backend verification project `AUTO-P4-20260704` through the API and started the official workflow template through the API.
- Verified the official runtime clones 12 nodes, 52 steps, 52 work items, and 51 workflow form instances.
- Completed two checklist-only incoming acceptance work items through the API with checklist payload only; no fake invoice or seed data was created.
- Verified automation recorded blocker events and activity using Vietnamese text for missing approved input procurement: `Chưa có yêu cầu mua/đầu vào đã duyệt để mở nghiệm thu đầu vào`.
- Cleaned the first verification blocker row that was created before backend rebuild so the verification project no longer displays the old English word `procurement` in blocker messages.
- Verified the verification project has 0 input invoices and 0 output invoices.
- Created, submitted, and approved live procurement request `PR-AUTO-P4-001` through the API for verification project `AUTO-P4-20260704`.
- Completed another checklist-only `Nghiệm thu Đầu Vào` work item through the API after procurement approval; verified automation created 1 incoming acceptance, 1 draft payable, draft payable schedule `LICH-THANH-TOAN-001`, blocker `Cần tải lên và xác minh hóa đơn đầu vào thật`, and still 0 input invoices.
- Completed one checklist-only `Nghiệm thu Đầu Ra` work item through the API; verified automation created 1 outgoing acceptance, 1 draft receivable, draft receivable schedule `LICH-THU-TIEN-001`, blocker `Cần tải lên và phát hành hóa đơn đầu ra thật`, and still 0 output invoices.
- Completed checklist-only `Kết thúc: Bàn giao + Đóng tài chính` work items through the API; verified automation creates/reuses a draft handover record and records blocker `Bàn giao/đóng tài chính còn điều kiện chưa hoàn tất` without creating payments or invoice records.
- Replaced the remaining user-facing handover blocker text `Kho công trình chưa closure` with Vietnamese text `Kho công trình chưa đóng`, rebuilt backend, and verified the new blocker metadata is Vietnamese.
- Ran final `git diff --check` successfully after live verification note updates.
- Re-verified DB state after live checks: 39 migrations applied, official template remains published, preserved draft template remains draft, verification project has 1 payable schedule, 1 receivable schedule, 0 input invoices, and 0 output invoices.
- Rebuilt/restarted the backend container after message hardening; backend auto-migrate reported `applied=0 skipped=39`.
- Ran `cd backend && GOCACHE="$PWD/.gocache" make test` successfully after the final Vietnamese blocker fix.
- Verified backend health with `curl -sS http://localhost:8080/api/health` after the final rebuild.
- Hardened user-facing workflow edge condition labels/reasons toward Vietnamese while preserving internal enum keys.
- Verified `/api/health` and `/api/pipeline-edge-conditions` after rebuild.
- Ran `cd backend && make fmt` successfully after Phase 4/5 hardening.
- Ran focused workflow tests with `cd backend && GOCACHE="$PWD/.gocache" go test ./internal/core -run 'TestClientWorkflowAutomation|TestClientOfficialWorkflow|TestPipelineEdge'` successfully.
- Ran `cd backend && make migrate-check` successfully after Phase 4/5 hardening.
- Ran `git diff --check` successfully after Phase 4/5 hardening.
- Ran full backend tests with `cd backend && GOCACHE="$PWD/.gocache" make test` successfully after Phase 4/5 hardening.

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
