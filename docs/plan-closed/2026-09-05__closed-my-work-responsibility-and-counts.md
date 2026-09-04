# [Closed] My Work responsibility semantics and counts

Closed at the user's request on 2026-09-05. Implementation and recorded verification are archived below; browser/UAT remains `NOT UX ACCEPTED`.

## [Closed] Restore My Work responsibility semantics

### Will be done

- Separate accounting authorization from accounting responsibility: `accounting_handoff.view`/`acknowledge` controls access and actions, while the global system role `accounting` controls whether an accounting handoff belongs to `Tôi phụ trách`.
- Keep accounting handoffs available to authorized Admin/Developer users under `Tất cả được phép xem` and the dedicated `Nghiệp vụ Kế toán` lane without adding them to those users' personal responsibility queue.
- Restore the general `Công việc` scope label to `Tôi phụ trách` and keep its quick filters generic; accounting-specific states remain in `Nghiệp vụ Kế toán`.
- Align My Work scope totals, navigation totals, AppShell badge, and Utility Dock with the same responsibility contract.
- Add backend role-matrix regression coverage and frontend presentation coverage, then run focused tests, full relevant tests, and the frontend production build.
- Update the central, backend, frontend, root-cause, accounting, workflow-orchestration, and admin-governance maps for the corrected responsibility boundary.

### Done

- Review package and Mockup as Text approved by the user.
- Root cause confirmed: the current implementation treats the permission pair `accounting_handoff.view` + `accounting_handoff.acknowledge` as proof of accounting responsibility, and the unified queue hard-codes every accounting row as `is_responsible=true`.
- Confirmed the approved contract: Accounting users receive accounting handoffs in `Tôi phụ trách`; Admin/Developer retain authorized visibility but do not inherit accounting responsibility.
- Added `Repository.UserHasGlobalRole` and the internal service-owned `AccountingResponsible` flag. HTTP/caller-supplied visibility or responsibility flags are overwritten; only a global `roles.name='accounting'` assignment qualifies.
- Unified queue rows now remain visible under `all` for authorized actors, while `responsible` includes only pending `handed_off` rows for Accounting. Returned and acknowledged records remain follow-up/history in `all` and the dedicated Accounting lane.
- `MyWorkScopeSummary` adds visible pending handoffs to `all` for authorized actors and to `responsible` only for Accounting. Navigation keeps `accounting_total` as a visibility breakdown but adds it to `work_total` only for Accounting responsibility, aligning AppShell and Utility Dock without frontend duplication.
- Restored the fixed `Tôi phụ trách` label and generic ordinary-work quick filters. Removed `Chờ bổ sung` and the Accounting-specific `Đã xử lý` wording from `Công việc`; legacy `bucket=waiting_supplement` links now normalize to `mode=accounting&accounting_status=returned` for authorized actors.
- Added regression coverage for permission-only/Admin access, project-scoped versus global Accounting roles, Accounting responsibility, returned/acknowledged history exclusion, scope totals, navigation totals, and caller-flag bypass protection.
- Updated central, backend, frontend, root-cause, accounting-finance, workflow-orchestration, admin-governance, and existing project-planning mapping references. No database migration was required because the correction uses the existing global role assignment contract.
- Focused frontend My Work tests passed: 22/22. Full frontend tests passed: 319/319.
- Focused backend role-matrix tests ran against the current local PostgreSQL stack and passed: 3/3, including the two DB-backed queue tests without skip.
- Full backend `go test ./... -count=1` passed for the current Go source; DB-backed suites outside the focused role-matrix run retain their normal environment-gated skip behavior.
- Current frontend production build passed after the frontend changes; Vite reported only the existing large-chunk advisory.
- Rebuilt and recreated only the local Docker backend from the current working tree; `GET http://127.0.0.1:8080/api/health` returned `success=true`, `service=consflow`, `status=ok`.
- Browser/UAT verification was not run under the repository rule for this turn; UI/UX status remains `NOT UX ACCEPTED`.

## [Closed] Fix My Work quick counts when selecting Chờ bổ sung

### Will be done

- Separate project-startup row visibility from project-startup summary contribution on `/my-work`.
- Keep `Xem tất cả` and `Làm ngay` source-backed counts stable for the compatibility `waiting_supplement` input while its UI ownership is resolved.
- Keep the top-level `Công việc` count aligned with the current scope/project `Làm ngay` total and independent of bucket/search/advanced filters.
- Add a focused frontend regression test and update the relevant Consflow mapping references.
- Run the focused My Work tests and the current frontend build.

### Done

- Review package and Mockup as Text approved by the user.
- Root cause confirmed in `frontend/src/modules/my-work/MyWorkPage.tsx`: `visibleStartupSummary` is incorrectly coupled to the selected bucket through `startupActionsMatchCurrentWorkFilters`.
- Added `projectStartupVisibility` so list-row visibility remains bucket-specific while quick-filter summary contribution remains bucket-independent.
- Added a regression covering `waiting_supplement`: startup rows are hidden, while `Xem tất cả` remains 6 and `Làm ngay` remains 3 for the reproduced fixture.
- Updated the central module map, frontend map, root-cause triage map, and project-planning domain map.
- Focused My Work tests passed: 21/21.
- Full frontend tests passed: 318/318.
- Current frontend production build passed; Vite reported only the existing large-chunk advisory.
- Browser/UAT verification was skipped under the repository rule; status remains `NOT UX ACCEPTED`.
- Supplemental review package approved after the user clarified that the top-level `Công việc` count must not follow the selected bucket.
- Changed the top-level `Công việc` count to read the selected scope/project total from `scopeSummary[queue].total`; it no longer derives from the bucket-filtered result page.
- Updated the presentation regression test to reject the former `firstPage.total + workflowActionTotal + startupActionTotal` calculation.
- Re-ran verification after the supplemental source change: focused My Work tests passed 21/21, full frontend tests passed 318/318, and the production build passed with only the existing large-chunk advisory.
- Superseded presentation note: the semantic recovery plan above removes `waiting_supplement` from ordinary-work quick filters and redirects its legacy URL to the dedicated Accounting lane; `projectStartupVisibility` remains the compatibility safeguard for source-summary stability.
