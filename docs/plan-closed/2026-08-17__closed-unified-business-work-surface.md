# [Closed] Unified business work surface

Status: TECHNICALLY VERIFIED COMPACT WORKFLOW CONTEXT / BROWSER UAT SKIPPED / NOT UX ACCEPTED

Objective: make `Bàn làm việc` and project queues navigate every work item to one canonical business workspace while keeping Node/Step as orchestration, dependency, approval and audit context.

## Will be done

- Add one backend-owned `execution_target` contract to work-item detail, My Work and project operations payloads.
- Preserve existing `business_source`, workflow/runtime snapshots, permissions and completion gates.
- Add shared frontend execution-target routing, task context and return navigation.
- Rename/reframe runtime `Quy trình` as `Tiến độ & Phụ thuộc` and remove it as a competing editor entry point.
- Make My Work a role-adaptive `Bàn làm việc` that opens the correct domain workspace.
- Integrate task context/deep links into Planning, BOQ, Contracts, Procurement, Warehouse, Operations, Acceptance, Handover and Project Finance.
- Provide an in-project preparation surface for form/evidence work that has no dedicated domain ledger.
- Keep legacy work-item routes as fail-closed fallback and retain old URLs for compatibility.
- Validate the Rev7 50-step inventory, mixed form/domain steps, RBAC, history and domain invariants.
- Update module/function/domain maps and run focused backend/frontend verification.
- Replace the cramped Preparation master-detail split with mutually exclusive full-width queue/detail modes, make the actionable queue the default focus, and leave only the active child highlighted in the project tree.
- Group the Preparation queue into clearly separated, collapsible stage sections labelled only by the stage name (for example `Giai đoạn Khảo sát`), without ordinal stage numbers or pagination.
- Replace the technical `Node · Bước` breadcrumb in Preparation detail with the approved compact same-stage context toolbar: stage name, current step position, previous/next step links and one progress link, without duplicating the current title or creating a mini workflow panel.

## Done

- Review package and Mockup as Text approved by the user.
- Current navigation, work-item presentation, Rev7 business-source routing and domain ownership mapped.
- Rev7 baseline inventoried: 50 steps = 18 form-only + 9 mixed form/domain + 22 source/action-only + 1 evidence-only.
- Implementation goal created for this plan.
- Added the backend-owned additive `execution_target` contract to work-item detail, paginated My Work, project work-item lists and Project Operations without changing stored workflow/runtime/domain state.
- The resolver uses stable action/surface/node/form metadata, preserves a guarded detail fallback, rejects unsafe/cross-project source routes and assigns clear domain/capture/evidence/approval/mixed completion modes.
- Renamed the actor landing surface to `Bàn làm việc`; every ordinary row and primary CTA now opens its canonical business workspace and preserves safe return navigation.
- Rebuilt the project information architecture around one business tree: `Kế hoạch` contains `Hồ sơ chuẩn bị`, `Kế hoạch công việc`, `Tiến độ & Phụ thuộc`; BOQ moved under `Thương mại & Vật tư`; work-item detail no longer replaces the tree with a second workflow sidebar.
- Added `/projects/:id/preparation` for form/evidence-first records without a dedicated ledger. It embeds the existing guarded Work Item workspace, rejects tasks outside Preparation, supports empty/loading/error/search states and returns to the originating workbench when available.
- Added shared project task context to BOQ, Contracts, Procurement, Warehouse, Operations, Acceptance, Handover and Project Finance. Presentation-only `?task=` shows assignment/source readiness and opens the existing guarded dialog for conditions, form/evidence/approval and step completion; it does not duplicate domain data entry.
- Reframed `/projects/:id/pipeline` and the project overview tab as `Tiến độ & Phụ thuộc`; lists, planning and overview use the same execution target rather than opening a parallel generic editor.
- Preserved existing work-item URLs and unsafe/missing/legacy target fallback. Frontend rejects external and cross-project target routes; project task context rejects a task from another project.
- Audited Rev7 in a fresh disposable PostgreSQL database: all 50 items resolve exactly once — Preparation 16, Planning 3, BOQ 4, Contracts 6, Procurement 5, Operations 1, Acceptance 6, Project Finance 8 and Handover 1; zero Rev7 items use Warehouse or generic fallback because Rev7 currently defines no warehouse-owned step. Legacy material/surplus targets still resolve to Warehouse.
- No migration or new template revision was required: this change adds a derived response/presentation contract and does not add a persisted field, domain transition or duplicate business form. Rev7 published definitions and all existing runtime/history records remain unchanged.
- Updated central, backend, frontend, workflow, planning, commercial, warehouse, construction, acceptance/handover, finance, admin and reporting maps.
- Verification on the current working tree: full backend `go test ./...` passed; focused DB-backed Rev7 target test passed against a disposable migrated PostgreSQL database and the database was removed; frontend `npm test` passed 129/129; TypeScript/Vite production build passed; bundle budget passed with 116 chunks and 124.72 KiB gzip entry against a 250 KiB limit.
- Built-in Codex Browser UAT passed on the running local build as Admin and PM. Verified Pipeline → Preparation with form fields and return navigation; Pipeline → BOQ, Operations, Acceptance and Project Finance with visible task context; My Work → Handover, Warehouse and Operations with canonical CTA labels and `returnTo=/my-work`; the project business tree remained visible throughout. No console warning/error was recorded. One non-blocking UX wording observation remains: the visible legacy label `Quản lý quy trình` still appears inside `Tiến độ & Phụ thuộc` and should be reviewed before user acceptance. The UAT was read-only and did not submit or mutate business records.
- The user rejected the first Preparation layout after visual review. Diagnosis proved that the unconditional 19rem list rail reused a horizontal `WorkItemCard`, while the selected Work Item added its own 18rem action rail; the result was severe text wrapping, nested rails and an empty detail canvas. The replacement queue/detail mockups were approved, including Built-in Browser UAT.
- Replaced the Preparation master-detail split with one full-width, searchable action queue and one mutually exclusive full-width detail mode. Desktop uses a compact operational table, mobile uses stacked cards, future/completed/history filters remain explicit, and the empty detail canvas plus repeated project breadcrumbs were removed.
- Corrected project-tree selection so an expanded group stays neutral and only its exact active child receives the strong blue state and `aria-current="page"`.
- Built-in Codex Browser UAT on the current local source passed at desktop 1280×720 and mobile 390×844. Verified queue/detail replacement, filter/search states, single active child, responsive table/cards, hidden mobile business tree, full-width detail, return navigation and zero horizontal page overflow. UAT found retained mobile scroll after opening a task; the transition now resets to the detail header and the rerun proved `scrollY=0` with `Danh sách hồ sơ` inside the viewport. A clean cold-load tab recorded zero console warning/error. No business mutation was submitted.
- Verification after the scroll correction on the current working tree: frontend `npm test` passed 132/132; `npm run build:budget` passed TypeScript/Vite production build with 115 chunks, 124.69 KiB gzip entry and a 250 KiB limit; `git diff --check` passed. Backend and database were unchanged by this correction, so backend/database checks were not rerun.
- The user approved the responsive Preparation stage-grouping mockup: desktop and mobile use distinct stage header bands, each group may collapse, the repeated `Giai đoạn` row/column label is removed, stage ordinals are not shown, step ordinals remain, all groups start expanded, and the 16-record queue remains unpaginated.
- Implemented workflow-order-preserving stage sections in the Preparation queue. Desktop and mobile now share the same expanded/collapsed group state, render only labels such as `Giai đoạn Khảo sát` and `Giai đoạn Họp báo cáo khảo sát`, keep `Bước 1`, `Bước 2`, etc. on individual records, and no longer repeat a stage column or stage label on each row. The stage header exposes its record count and stable `aria-expanded`/`aria-controls` semantics.
- Added a pure grouping helper and focused regression coverage proving that records from the same runtime node stay together in workflow order and that group names contain no ordinal prefix. Updated the central, frontend, root-cause and project-planning maps with the durable grouping behavior and owner files.
- Verification on the final frontend source for this increment: `npm test` passed 133/133; `npm run build:budget` passed TypeScript/Vite production build with 115 chunks, a 121.6 KiB gzip entry and a 250 KiB limit; the Preparation chunk is 4.7 KiB gzip; `git diff --check` passed. Backend, database and migrations are unchanged, so those checks were not rerun. Built-in Browser UAT was not requested in this implementation turn and was therefore skipped; this increment remains `NOT UX ACCEPTED` until browser or user visual acceptance runs against the current build.
- The user rejected the first three-card previous/current/next mockup because it consumed too much vertical space, then approved the compact replacement: one 40-44px desktop toolbar showing only `Giai đoạn <tên>`, `Bước x/y`, previous step, next step and `Tiến độ`; the current step title remains solely in the work-item heading below.
- Implemented the approved compact context toolbar only for selected Preparation details. It derives position and adjacent records from the complete project work-item set but restricts siblings to the current pipeline run and `run_node_id`, sorts by snapshotted step order, exposes explicit first/last boundaries, and opens each adjacent record through its canonical `execution_target` while preserving the incoming safe return route. The existing work title, primary surface, action rail, permissions, requirements, history and mutations are unchanged; other Work Item detail consumers retain their previous header.
- Replaced the technical `Node · Bước` breadcrumb in that context with `Giai đoạn <tên>` and `Bước x/y`. The desktop presentation is one compact 44px row; narrow screens wrap it into short rows with truncated optional neighbor titles instead of rendering a three-card mini-workflow or repeating the current title.
- Added focused regression coverage for scrambled input order, same-node previous/next derivation, first-step boundary, cross-node isolation, optional shared-header behavior and compact accessible markup. Updated the central, frontend, root-cause, project-planning and workflow-orchestration maps with the new helper, ownership and no-cross-node invariant.
- Verification on the current frontend source for this increment: `npm test` passed 134/134; `npm run build:budget` passed TypeScript/Vite production build with 115 chunks, a 121.6 KiB gzip entry and a 250 KiB limit; the Preparation chunk is 5.1 KiB gzip and the shared Work Item detail chunk is 51.7 KiB gzip; `git diff --check` passed. Backend, database and migrations are unchanged by this increment, so those checks were not rerun. Built-in Browser UAT was not requested in this implementation turn and was therefore skipped; the compact toolbar remains `NOT UX ACCEPTED` until visual UAT or user acceptance runs against this build.

## Constraints

- Published templates, project runtime snapshots, submitted form revisions, signatures, PDFs, approvals and audit history remain immutable.
- Backend authorization remains authoritative; frontend capability fields are presentation hints only.
- Browser UAT is agent-run evidence against the local build; final user-based UX acceptance remains pending.
- Any workflow-definition change must be a new additive template revision/migration and must not rebind existing runs.

## Closure

- Closed on 2026-08-17 at the user's request after implementation and technical verification.
- The final compact-toolbar increment did not receive Browser UAT in its implementation turn and therefore remains `NOT UX ACCEPTED` in this historical record.
- This file is historical only. Durable routing, ownership and invariants remain in `.agents/skills/**/references/*.md`.
