# [Closed] Project Explorer Paging — Utility Dock Clearance — approved 2026-09-08

Status when closed: TECHNICALLY VERIFIED; scope is frontend presentation only; NOT UX ACCEPTED until an explicitly authorized browser check runs.

Purpose: ensure every ordinary document-end control, including Warehouse and Project Explorer paging, remains visible and clickable above the fixed Trao đổi/Làm ngay Utility Dock without per-page patches.

## Pending acceptance

- Browser acceptance remains pending explicit authorization: verify `/projects` and `/warehouse` at the reported desktop width with minimized/expanded Dock, then at a narrow mobile width with the Tiện ích launcher and safe area. Confirm every pagination control remains visible and clickable.

## Done

- Loaded central, frontend, root-cause, Project Planning and Materials/Warehouse maps; matched the global `AppShell` owner, Project Workspace focus branch, Project Explorer and Warehouse.
- Confirmed from the supplied `/projects` and `/warehouse` screenshots and source that the fixed Dock (`bottom-4`, 44px desktop launchers) overlays unreserved document-end controls.
- The earlier Project Explorer-only clearance was technically verified but is deliberately superseded by the approved global AppShell mechanism.
- User approved the targeted presentation-only global replacement. No API, permission, project/warehouse status, Dock placement or paging request contract will change.
- Added one `AppShell` clearance class: `8rem + safe-area` on mobile and `4rem` from `sm`. It applies to standard-shell `main` and the Project Workspace focus wrapper.
- Kept Pipeline Builder out of the rule because its explicit `h-screen` canvas has no document-end controls; this avoids inventing a scrollable blank region in the editor.
- Removed the local Project Explorer clearance and changed its regression test to prevent a route-specific override from returning.
- Updated Utility Dock, central, Project Planning and Materials/Warehouse mappings with the global owner and unchanged data contracts.

## Prior page-local verification — 2026-09-08

- Environment: local macOS, frontend installed dependencies; current working tree containing this plan's source, test and mapping edits.
- `cd frontend && node --experimental-strip-types --test tests/utilityDock.test.ts tests/projectExplorerPagination.test.ts`: passed, 8 tests; this result covers only the now-superseded page-local clearance.
- `cd frontend && npm run build`: passed TypeScript and Vite `6.4.3`; 1,934 modules transformed, Vite stage 4.70s. This build predates the global AppShell implementation and is therefore stale.
- `git diff --check`: passed with no whitespace errors before the global replacement. It was rerun below.
- Browser/DOM measurement and click UAT remain pending explicit authorization.

## Global verification — 2026-09-08

- Environment: local macOS, frontend installed dependencies; validation ran against the current shared working tree, which also contains unrelated Project Explorer/backend changes preserved without modification by this plan.
- `cd frontend && node --experimental-strip-types --test tests/utilityDock.test.ts tests/projectExplorerPagination.test.ts`: passed, 8 tests; verifies clearance on both standard/focus AppShell branches and confirms Projects no longer owns a local clearance.
- `cd frontend && npm run build`: passed TypeScript and Vite `6.4.3`; 1,934 modules transformed, Vite stage 4.76s.
- `git diff --check`: passed with no whitespace errors.
- Skipped: browser/DOM measurement and click UAT, because no explicit browser-verification request was provided. Technical checks do not establish UX acceptance.
