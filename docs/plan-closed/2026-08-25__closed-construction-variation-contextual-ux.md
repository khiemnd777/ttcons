# [Closed] Làm rõ UX Phát sinh ngoài kế hoạch trong Thi công

Closed on: 2026-08-25

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED (approved by user on 2026-08-25)

## Will be done

- Remove the permanent amber variation-readiness band from the top of Operations; the primary construction workspace remains focused on daily reports, checks and field actions.
- Render a compact variation status only when real non-terminal variation records exist, and keep direct proposal entry contextual to `Cần xử lý` or an impacted construction action.
- Distinguish loading, API error, no active client contract, no variation and active-variation states. A missing contract must use a neutral contextual blocker and explicitly state that reporting/check/remediation remain available.
- Preserve `contract.variation.propose`, active-client-contract, eligible-source-work-item, action-impact and downstream QS/Purchasing/PM-Director gates. Add no backend/API/schema/migration/data change.
- Add focused frontend presentation regressions, update durable Operations/variation mappings and validate the current dirty working tree without reverting unrelated changes. Browser/UAT remains skipped unless separately authorized.

## Done

- User approved the review package, responsive Mockup as Text and the four confirmation points with `Approve`.
- Confirmed the current root cause in source and local PostgreSQL: Operations always renders the amber band for an Operations viewer; `context?.available=false` represents missing commercial readiness rather than an actual variation; the selected project has zero contracts and zero variations; Supervisor already has the narrow proposal permission.
- Confirmed the existing contextual seam: impacted construction-action detail already owns `Đề xuất phát sinh`, while the proposal API and backend validations remain authoritative.
- Removed the permanent amber readiness band. Operations now renders a compact neutral `Phát sinh thương mại` status only when at least one real non-terminal variation exists; the Contracts link remains permission-gated.
- Moved direct proposal entry into the active `Cần xử lý` toolbar and kept impacted-action proposal entry in the action detail. Both reuse the existing eligible Work Item, permission, contract and QS submission contract.
- Added separate contextual loading, retryable API-error, missing-active-contract and missing-permission states. Missing contract is neutral, tells PM/Thương mại what is needed and explicitly confirms that reporting, checking and field remediation remain available; Supervisor never receives an unauthorized Contracts link.
- Added the focused presentation regression `construction variation stays contextual and distinguishes unavailable readiness from real alerts`; it failed against the old permanent band and passes after implementation.
- Updated the central, frontend, root-cause and Construction Operations domain mappings with the progressive-disclosure and permission-boundary contract. No backend-function mapping, API, schema, migration or project-data update was required because this change only alters frontend presentation and entry-point placement.
- Current dirty working-tree verification at the existing HEAD passed: 265/265 frontend tests; TypeScript/Vite production build; 250 KiB gzip bundle budget with 141 chunks; and `git diff --check`.

## Skipped / remaining acceptance

- Browser/UAT is not authorized in this turn and remains `NOT UX ACCEPTED`.
