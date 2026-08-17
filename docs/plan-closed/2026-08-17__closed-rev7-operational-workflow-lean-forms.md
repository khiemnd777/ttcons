# [Closed] Rev7 - tinh gọn biểu mẫu workflow chính thức

Status: IMPLEMENTED — TECHNICALLY VERIFIED for the focused Rev7 scope; NOT UX ACCEPTED (2026-08-17)

## Will be done

- Publish an immutable Rev7/engine-4 official workflow with 15 nodes and 50 steps; keep `planning.assignment_minutes`, remove the duplicate `planning.assignment_meeting`, and preserve every older template/runtime snapshot.
- Reduce repeated administrative capture by applying the approved field decision matrix: keep core fields, move optional details behind progressive disclosure, replace approval/signature/document confirmation fields with real actions, and source module-owned facts from backend-owned readiness data.
- Add the minimum document/BOQ release metadata required for trustworthy revision/version sources and fail closed when a source cannot be proven.
- Expose one backend source/default/readiness contract to both workflow-form rendering and completion validation; keep permissions, actor, audit, evidence, approval, signature, PDF, BOQ/procurement, acceptance, stock and finance invariants intact.
- Update the work-item UI with source-backed read-only facts, a compact required-first form, `Thông tin bổ sung` for optional fields, permission-aware domain actions, and the approved responsive action layout.
- Add Rev7 Golden seed/validation coverage, focused backend/frontend regression tests, migration verification, backend tests and frontend build.
- Update central/backend/frontend/domain mapping files in the same work turn.

## Done

- User approved the Rev7 review package and Mockup as Text with `OK triển khai theo plan Rev7`.
- Baseline source revision recorded as `f6569f0`; working tree was clean before implementation.
- Mandatory module, backend, frontend and affected domain maps were loaded; likely owners and collision surfaces were confirmed.
- Added custom migrations `V103__operational_official_workflow_rev7.sql` and `V104__rev7_returned_scan_source_gate.sql`; both were applied by the official runner and their checksums pass `make migrate-check` across all 104 migrations.
- Published immutable official template `40000000-0000-0000-0000-000000000026` (Rev7/engine 4) without mutating Rev6 or historical runs: 15 nodes, 50 steps, 27 form-backed steps, 22 source/action-only steps and 87 field occurrences. The duplicate assignment meeting and returned-scan form were removed.
- Added `suggested_answers` defaults for current date, actor name and step title. Persisted/user answers win over suggestions, and the backend applies the same defaults before validation.
- Added fail-closed `business_source` readiness for canonical planning, design, BOQ, contract, procurement, acceptance, handover, accounting and invoice data; official hard automation now recognizes the full template family, including Rev7.
- Added monotonic document release revisions and immutable BOQ release snapshots. Historical releases remain auditable rather than being overwritten.
- Kept Rev7 `operational_capture_v3` form versions pinned to its steps and excluded them from generic catalog/default selection, preventing behavior drift in unrelated workflows.
- Updated the work-item UI with a canonical source card, direct source navigation, required-first fields and collapsed `Thông tin bổ sung`; no empty form is rendered for source/action-only steps.
- Added and applied additive Golden fixture `GOLDEN-REV7-OPS-001` (project/run suffix `...027`) and extended seed validation/Makefile ordering.
- Added focused migration, source-gate, suggested-default, document-release, BOQ-release and frontend regression coverage. Frontend tests pass 126/126; frontend production build passes; Go tests without external DB pass; focused DB-backed Rev7/default-form regressions pass.
- Updated central, backend, frontend and all affected workflow/planning/construction/commercial/quality/accounting domain maps.

## Verification limits

- Browser/UAT verification was not run because the repository requires explicit user request for browser checks. Current UI status is `NOT UX ACCEPTED`.
- The full DB-backed Go suite is not globally green: existing concurrent fixed-code collisions, one pre-existing official-edge fixture mismatch and one migration-runner pending-trigger failure remain outside the Rev7 focused checks.
- The full Golden validator stops at the existing Golden Live App-role count mismatch (expected 8, local data has 5). The Rev7 Golden fixture's own assertions pass.

## Approved follow-up - field wording and one-column capture

Status: IMPLEMENTED — TECHNICALLY VERIFIED; NOT UX ACCEPTED (2026-08-17)

### Will be done

- Present `constraints` as `Ghi chú khác`, `design_exclusions` as `Phong cách / chi tiết cần tránh`, and `owner_feedback` as `Nội dung cần chỉnh trên layout`, each with clear help text.
- Scope the aliases to `operational_capture_v3` at runtime so work-item UI, validation and newly generated PDFs agree without mutating published form definitions, stable field keys, stored answers or historical variants.
- Keep all workflow-form fields and the `Thông tin bổ sung` disclosure in one explicit column at every breakpoint.
- Add focused backend/frontend regressions and update the central/backend/frontend/workflow/root-cause maps.

### Done

- User approved the review package and Mockup as Text with `OK triển khai theo review package này`.
- Confirmed the implicit-grid root cause and identified the three currently affected Rev7 steps: `survey_review.branch_decision`, `design.construction_drawing`, and `design.handover_to_qs`.
- Added a non-mutating backend presentation adapter scoped to `operational_capture_v3`: `constraints`, `design_exclusions` and `owner_feedback` now expose the approved labels/help text to runtime UI, validation, amendments and newly generated PDFs without changing published definitions, stable keys, stored answers or historical variants.
- Enforced one explicit column for the primary workflow-form grid, the `Thông tin bổ sung` grid and every field wrapper at all breakpoints; removed the implicit responsive column span that created the second track.
- No migration was required or added for this follow-up. Existing V103/V104 files were not edited; `make migrate-check` still validates all 104 migration checksums.
- Added focused backend presentation tests and frontend one-column regressions, and aligned the existing structured-list layout assertion with the new invariant.
- Verification on the current working tree: focused Go presentation/default tests pass; full Go tests without external DB pass; frontend tests pass 126/126; frontend production build passes; `git diff --check` passes.
- Updated central, backend, frontend, workflow-domain and root-cause mapping files.

### Verification limits

- Browser/UAT verification was not run because the user did not explicitly request browser checks in this turn. Current UI status is `NOT UX ACCEPTED`.
