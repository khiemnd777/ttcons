# [Closed] Canonical Workflow Terminology Rollout — 2026-09-14

Closed by user request on 2026-09-14. Closing this plan archives the agreement and plan; it does not claim that the product labels have been implemented.

## Status

- TARGET: terminology agreement recorded and rollout plan prepared.
- IMPLEMENTED: no product source, UI, API, database, seed or migration change was implemented under this plan.
- TECHNICALLY VERIFIED: documentation and mapping changes were checked with `git diff --check`; the terminology agreement commit was created and pushed.
- UX ACCEPTED: no; no UI rollout or browser/UAT verification was performed.
- BLOCKED: no blocker for closing the planning record. Product rollout remains a separate reviewed implementation task.

## Agreement

- Node -> `Giai đoạn`.
- Step -> `Gói công việc`.
- Work Item -> `Đầu việc`.
- The final workflow stage previously described as `Tài chính công trình` -> `Bàn giao công trình và hồ sơ kế toán`.
- The fourth mapping is scoped to the workflow final stage; the standalone project-finance workspace remains unchanged unless separately reviewed.
- Stable technical identifiers, API/schema fields, permissions, business codes, event keys and historical snapshots remain protected.

## Will Be Done (archived plan)

- Before product edits, prepare a review package and a `Mục đích chính`-first Mockup as Text covering Pipeline Builder, project startup, runtime navigation, Work Item detail/context and final-stage presentation; wait for explicit implementation confirmation.
- Build and preserve a reviewed occurrence inventory. Change only user-facing workflow labels and human-readable validation, activity, notification, PDF and export copy; exclude code symbols, types, API fields, DB columns, query keys, route params, permissions, business codes and event keys.
- Update Pipeline Builder presentation in `frontend/src/modules/pipeline-builder/PipelineBuilderPage.tsx`, `components/NodeLibraryPanel.tsx`, `components/NodeInspectorPanel.tsx`, `components/EdgeInspectorPanel.tsx` and `components/diagram/NodeCard.tsx` so the interface consistently says `Giai đoạn` and `Gói công việc`.
- Update project/runtime presentation in `frontend/src/modules/projects/ProjectStartupWizard.tsx`, `ProjectWorkspaceLayout.tsx`, `ProjectDetailPage.tsx`, `frontend/src/modules/planning/PlanningPage.tsx`, relevant Operations messages, `frontend/src/modules/work-items/WorkItemPrimarySurface.tsx`, `workItemContext.ts`, `frontend/src/modules/planning-daily-reports/PlanningDailyReportsPage.tsx` and `frontend/src/lib/utils/status.ts` so user-facing workflow text consistently says `Giai đoạn`, `Gói công việc` and `Đầu việc`.
- Update backend user-facing text and generated artifacts in the exact owners confirmed by review, led by `backend/internal/core/pipeline_template_pdf.go`, `pipeline_template_pdf_diagram.go`, `project_startup.go`, `service_pipeline_runtime.go`, `repository_pipeline.go`, `repository_construction_quality.go` and workflow-definition Excel presentation. Keep response field names and workbook machine contracts stable unless an explicit compatibility decision says otherwise.
- Publish the renamed official final stage through the next available additive migration as a new immutable template revision based on Rev9, update `backend/seed/seed.sql` for parity, and keep applied migrations and historical runtime artifacts unchanged.
- Keep the standalone `frontend/src/modules/project-finance/ProjectFinancePage.tsx` workspace and its source-module labels unchanged unless a separate review confirms the broader rename.
- Update focused frontend/backend assertions, add a categorized terminology scan, and run the scoped tests/build/migration checks before any implementation completion claim. Browser/UAT requires explicit authorization.

## Done

- Recorded the user-approved agreement in `docs/workflow-terminology-agreement.md` and anchored it in `AGENTS.md`, the central module map, frontend UI map and Workflow Orchestration domain map.
- Loaded the mandatory maps and inspected local source before preparing the plan.
- Confirmed the current mixed presentation and protected technical/history boundaries.
- No product source, UI, API, database schema, seed or migration was changed while preparing the plan.
- Closed this planning record at the user's request on 2026-09-14.

## Remaining Work

- A future implementation task must provide the review package and Mockup as Text, receive explicit implementation approval, then execute and verify the archived rollout plan.
- Until that occurs, the product remains on its existing mixed labels and the standalone `Tài chính công trình` workspace name is preserved.
