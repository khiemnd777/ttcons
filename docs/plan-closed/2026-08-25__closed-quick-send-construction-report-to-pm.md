# [Closed] Gửi nhanh báo cáo cho PM tại Chi tiết Thi công

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED. Closed by user request on 2026-08-25.

## Will be done

- Make the saved construction-report detail dialog the fastest delivery surface: an eligible authored draft shows primary CTA `Gửi cho PM · <tên>`, while content editing remains the separate secondary `Sửa báo cáo` action.
- Reuse the existing protected `POST /work-reports/daily-reports/:id/submit` route for saved-source delivery. Do not create a second content editor, another delivery table or a new API route.
- Share project-scoped recipient loading/recommendation between the create/edit form and detail dialog: prefer the active eligible project PM, fall back to the sole eligible reviewer, and expose a compact recipient selector only when several eligible reviewers exist without a recommendation.
- Enforce the detail-action state matrix: draft author with `work_report.submit` may send; changed `needs_changes` may resend; submitted/acknowledged/recorded/historical, non-author and unauthorized sources remain status-only/read-only.
- Prevent no-op resubmission after `needs_changes` by comparing the current canonical source snapshot hash with the latest immutable revision before incrementing the revision. An unchanged source must fail before report status/event/revision/audit mutation.
- Keep delivery server-authoritative and concurrency-safe under the existing source/report row locks. Repeat clicks, stale state, revoked permission or an invalid recipient must fail closed and refresh the visible report status.
- Preserve the existing immutable revision/event/audit lifecycle, `/reports` review register, source/report cache parity, and project-scoped RBAC. Quick send must not edit report content, start/complete the Work Item, advance the Node, clear blockers or alter any previous revision.
- Add focused PostgreSQL regressions for first quick send, unauthorized/invalid-recipient rollback, concurrent/stale submit and unchanged-versus-changed resubmit; add frontend regressions for CTA/state/RBAC/loading/error/mobile behavior and absence of a full edit-form detour.
- Update central/backend/frontend/root-cause mappings plus Construction Operations, Reporting Analytics and Workflow Orchestration maps after approved implementation. No schema migration, seed rewrite or data repair is planned.
- Validate focused and full backend tests, all frontend tests, TypeScript/Vite production build, bundle budget and `git diff --check`. Browser/UAT remains skipped unless separately authorized and the result stays `NOT UX ACCEPTED` until then.

## Done

- User approved the review package, responsive Mockup as Text, one-click recommended recipient, unchanged-resubmit guard and keeping `/reports` review-only with `OK triển khai`.
- Matched the feature to Construction Operations daily-report detail, authored Work Report delivery and Workflow Work Item invariants using the central, backend, frontend and three domain maps.
- Confirmed from `ConstructionReportDetailDialog.tsx` that the footer previously exposed only `Đóng`, `Sửa báo cáo` and `Mở công việc liên quan`; it had no delivery capability or callback.
- Confirmed from `ConstructionReportFormDialog.tsx` and `OperationsPage.tsx` that recipient recommendation and `work_report.submit` capability were mounted only after opening the create/edit form, which caused the extra user step.
- Confirmed that the backend route, author/RBAC/recipient validation, source/report row locking and immutable submit/resubmit transition already existed. The missing behavior was the source-detail UX plus a no-op resubmission guard.
- Completed the responsive Mockup as Text and impact review before implementation.
- Added shared project-scoped recipient discovery for both the content form and saved-source detail. Detail auto-selects the eligible project PM/sole reviewer, supports a compact exceptional selector, and exposes direct `Gửi cho …` / `Gửi lại cho …` with loading, error, no-recipient and mobile full-width states.
- Kept edit separate from delivery. An unchanged `needs_changes` source now shows `Sửa theo yêu cầu`; submitted/acknowledged and unauthorized/non-author sources remain status-only/read-only.
- Reused the protected saved-source submit endpoint in `OperationsPage`, added author/permission presentation gates, disabled repeated clicks while pending, refreshed all report/source caches, merged the authoritative response into the open dialog, and preserved API-error toast/recovery behavior.
- Added backend `delivery_source_changed` annotation and a transaction-level canonical snapshot-hash guard. An unchanged resubmit fails before report status, revision, event or audit mutation; a changed source still creates the next immutable revision.
- Added focused frontend source-contract coverage and PostgreSQL-backed unchanged/changed resubmit regressions. Both focused DB tests pass against the local Consflow database; the current frontend TypeScript/Vite production build also passes.
- Updated central/backend/frontend/root-cause mappings plus Construction Operations, Reporting Analytics and Workflow Orchestration domain maps. No migration, seed rewrite or data repair was required.
- Current working-tree verification at HEAD `12e8287` plus the listed dirty-tree changes passed: focused PostgreSQL integration for PM recommendation, atomic first send, invalid/unauthorized recipient rollback, repeat-submit rejection and unchanged/changed resubmit; full backend `go test ./...`; 270/270 frontend tests; TypeScript/Vite production build; 250 KiB gzip bundle budget with 142 chunks; and `git diff --check`.

## Resolved decisions

- Direct one-click send is used when a PM or sole reviewer is recommended; a compact inline selector is used only for the exceptional ambiguous-recipient case.
- After `needs_changes`, unchanged content cannot be sent again; the UI presents `Sửa theo yêu cầu` until the source snapshot differs from the latest submitted revision.
- `/reports` remains delivery/review history only and does not regain a competing submit form.

## Skipped / remaining acceptance

- Browser/UAT was not authorized in the implementation turn and remains `NOT UX ACCEPTED`.
