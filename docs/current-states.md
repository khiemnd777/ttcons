# Current States

## Canonical Workflow Terminology Rollout

Status: AGREEMENT RECORDED; IMPLEMENTATION PLAN PREPARED; PRODUCT CHANGES NOT IMPLEMENTED; NOT UX ACCEPTED.

Matched feature/domain: Pipeline Template Builder, Pipeline Runtime, Work Item execution and official client workflow final-stage definition. Likely owners: Pipeline Builder presentation, project startup/runtime workspaces, Work Item context/status presentation, pipeline-template PDF/export, workflow validation/error copy, official template seed plus a future additive migration. First searches: `Node|Step|Work Item|work item|Tài chính công trình|Khóa tài chính công trình`, followed by per-match classification as display copy, current persisted definition, immutable history or technical contract.

Will be done:

- Before product edits, prepare a review package and a `Mục đích chính`-first Mockup as Text covering Pipeline Builder, project startup, runtime navigation, Work Item detail/context and final-stage presentation; wait for explicit implementation confirmation.
- Build and preserve a reviewed occurrence inventory. Change only user-facing workflow labels and human-readable validation, activity, notification, PDF and export copy; exclude code symbols, types, API fields, DB columns, query keys, route params, permissions, business codes and event keys.
- Update Pipeline Builder presentation in `frontend/src/modules/pipeline-builder/PipelineBuilderPage.tsx`, `components/NodeLibraryPanel.tsx`, `components/NodeInspectorPanel.tsx`, `components/EdgeInspectorPanel.tsx` and `components/diagram/NodeCard.tsx` so the interface consistently says `Giai đoạn` and `Gói công việc`.
- Update project/runtime presentation in `frontend/src/modules/projects/ProjectStartupWizard.tsx`, `ProjectWorkspaceLayout.tsx`, `ProjectDetailPage.tsx`, `frontend/src/modules/planning/PlanningPage.tsx`, relevant Operations messages, `frontend/src/modules/work-items/WorkItemPrimarySurface.tsx`, `workItemContext.ts`, `frontend/src/modules/planning-daily-reports/PlanningDailyReportsPage.tsx` and `frontend/src/lib/utils/status.ts` so user-facing workflow text consistently says `Giai đoạn`, `Gói công việc` and `Đầu việc`.
- Update backend user-facing text and generated artifacts in the exact owners confirmed by review, led by `backend/internal/core/pipeline_template_pdf.go`, `pipeline_template_pdf_diagram.go`, `project_startup.go`, `service_pipeline_runtime.go`, `repository_pipeline.go`, `repository_construction_quality.go` and workflow-definition Excel presentation. Keep response field names and workbook machine contracts stable unless an explicit compatibility decision says otherwise.
- Publish the renamed official final stage through the next available additive migration, expected to follow V143 at implementation time, as a new immutable template revision based on Rev9. Update `backend/seed/seed.sql` for canonical seed parity. Do not edit V83, V105, V131 or any other applied migration; do not rewrite active or historical Pipeline Runs, Work Items, forms, audits or generated PDFs.
- Keep the standalone `frontend/src/modules/project-finance/ProjectFinancePage.tsx` workspace and its source-module labels unchanged unless a separate review confirms that `Bàn giao công trình và hồ sơ kế toán` accurately names the whole finance workspace rather than only the workflow's final stage.
- Update focused frontend and backend assertions for every changed entrypoint, including Pipeline Builder labels, startup/runtime wording, Work Item context, status labels, template PDF/diagram text and the new official template revision. Add a regression scan that fails on disallowed legacy user-facing workflow terms while allowing approved technical and historical occurrences.
- Run focused frontend Node tests, focused Go tests, `cd frontend && npm run build`, `cd backend && go test ./...` when feasible, migration/seed parity checks, `git diff --check`, and a final categorized terminology scan. Browser/UAT will run only if explicitly authorized; without it, report the UI as `NOT UX ACCEPTED`.
- Update the central, frontend, backend and Workflow Orchestration mappings in the same implementation turn for the final owner list, new revision and verified behavior.

Done:

- Recorded the user-approved agreement dated 2026-09-14 in `docs/workflow-terminology-agreement.md` and anchored it in `AGENTS.md`, the central module map, frontend UI map and Workflow Orchestration domain map.
- Confirmed the exact shared vocabulary: Node -> `Giai đoạn`; Step -> `Gói công việc`; Work Item -> `Đầu việc`; final workflow stage `Tài chính công trình` -> `Bàn giao công trình và hồ sơ kế toán`.
- Loaded the mandatory central, frontend, backend and Workflow Orchestration maps and inspected local source before forming this plan.
- Confirmed current mixed presentation evidence: Pipeline Builder still exposes `Node`, `node` and `step`; Project Startup still exposes `Node đầu`, `bước` and `Work Item`; runtime surfaces include residual `Node`/`bước`; template PDF metadata and tables still use `node`/`step`.
- Confirmed the current official Rev9 source preserves Rev8 topology and changes only the final work package to `Xác nhận bàn giao hồ sơ kế toán`; older applied migrations and runtime snapshots remain historical evidence, not replacement targets.
- Classified stable technical identifiers and historical snapshots as protected exclusions. No product source, UI, API, database schema, seed or migration was changed while preparing this plan.
