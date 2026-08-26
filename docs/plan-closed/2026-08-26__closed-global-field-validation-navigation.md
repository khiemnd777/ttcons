# [Closed] Điều hướng tự động đến trường validation lỗi trên toàn ứng dụng

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED. Closed by user request on 2026-08-26.

## Will be done

- Add one scoped frontend validation-navigation contract that opens controlled parent UI when needed, scrolls the first addressable invalid field into view, focuses its enabled control and applies a temporary error highlight while respecting reduced-motion preferences.
- Integrate the contract into shared `SimpleForm`, Workflow Forms, React Hook Form auth/profile flows and every current custom field-addressable validation surface; keep network, authorization, workflow-state and other non-field errors on their existing toast/banner paths.
- Preserve current field order, inline errors, warning summaries, API allowlists, nested-dialog isolation, domain validation, RBAC, workflow transitions and persistence contracts. Add no backend, API, schema, migration or seed changes.
- Add focused regression coverage, update central/frontend durable mappings, then run the frontend test suite, TypeScript/Vite production build, bundle budget and whitespace validation. Browser/UAT remains skipped unless separately authorized.

## Done

- User approved the all-application scope, first-error navigation, automatic tab/accordion/category reveal and responsive Mockup as Text with `OK triển khai toàn bộ`.
- Loaded the mandatory central and frontend maps plus the Frontend Workflow instructions. Confirmed the current application has shared `SimpleForm`, Workflow Form, React Hook Form and handwritten validation families, while the existing smooth-scroll/focus/highlight helper is local to Work Item requirements.
- Confirmed the implementation boundary: only errors with an explicit editable field target navigate; general API/network/authorization/state warnings retain their existing feedback without arbitrary scrolling.
- Added the shared scoped `formValidationNavigation` contract. It orders invalid keys deterministically, resolves only inside the active form/dialog/panel, opens controlled parent UI before navigation, selects a visible responsive duplicate, respects reduced-motion, scrolls to center, focuses the first usable control and removes its temporary error ring automatically.
- Integrated the contract into the shared `SimpleForm`, Workflow Form/runtime editors, React Hook Form auth/profile pages and the current handwritten field-addressable validation surfaces across construction reports/checks/actions, acceptance/quality, Pipeline Builder, BOQ/quotation, accounting/debt/finance, contracts, settings, project/staff membership, Work Item operational panels, work reports and shared evidence/approval/signature flows.
- Preserved inline validation, warning summaries, nested-dialog scope and the existing handling for API/network/RBAC/workflow-state errors. Immediate constraint warnings whose offending control already retains focus continue to use their current inline/toast feedback without an artificial scroll.
- Added focused regression coverage for ordering, scoped lookup, controlled reveal, animation/focus/reduced-motion behavior and representative integrations; adjusted the affected SimpleForm source-contract assertion. Updated the central and frontend utility mappings. No business invariant, backend contract, database schema or migration changed, so no domain/backend mapping update was required.
- Current dirty-working-tree verification at HEAD `dd4923b` passed: 273/273 frontend tests; TypeScript/Vite production build with 1,875 modules; 250 KiB gzip bundle budget with 142 chunks (entry 126.1 KiB); and `git diff --check`.

## Resolved decisions

- Navigate only to the first field-addressable validation error inside the active form, dialog or panel.
- Reveal controlled parent tabs, accordions, categories or responsive sections before measuring and scrolling to the target.
- Keep API/network/RBAC/workflow-state errors and immediate in-place constraint warnings on their current non-scrolling feedback paths.
- Preserve reduced-motion preferences and avoid removing pre-existing visual classes when the temporary validation highlight is cleaned up.

## Skipped / remaining acceptance

- Browser/UAT was not authorized in the implementation turn and remains `NOT UX ACCEPTED`.
