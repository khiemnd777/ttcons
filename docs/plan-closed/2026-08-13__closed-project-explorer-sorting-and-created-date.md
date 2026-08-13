# [Closed] Project Explorer Sorting And Created Date

## Will be done

- Add local sorting for `Cây công trình`, `Trạng thái`, and `Phụ trách` on `/projects` without changing the API, project visibility, or workflow state.
- Preserve API order until the user chooses a sort; apply search/status filtering before sorting and keep expanded projects attached to their IDs.
- Provide accessible clickable desktop headers and a compact mobile sort selector with equivalent ascending/descending choices.
- Add focused regression coverage, update central/frontend/project-planning mapping, and run non-browser frontend verification.
- Show the source-backed project creation date immediately after progress on desktop and as labelled metadata on narrow screens, without changing the API.
- Keep every desktop table header visually uppercase and add chronological creation-date sorting to the desktop header and mobile selector, with missing or invalid dates last in either direction.

## Done

- Confirmed from the supplied Edge screenshot and current source that all three desktop headers are static and the filtered list preserves API order.
- Produced the approved review package and Mockup as Text; received explicit `OK triển khai` for frontend-only sorting, Vietnamese display-value semantics, missing PM values last, and responsive controls.
- Added a non-mutating project-explorer sort helper with natural Vietnamese code/name comparison, display-label status comparison, stable project tie-breaking and unassigned-PM-last behavior in both directions.
- Added accessible desktop header controls and a narrow-screen sort selector backed by the same local state; search/status filtering still runs first and expanded project IDs remain unchanged.
- Added focused sort and responsive regression coverage, and updated central/frontend/project-planning mapping for the new `/projects` presentation contract.
- Technically verified the current frontend source with all 70 tests, the TypeScript/Vite production build, the bundle budget (121.5 KiB gzip eager entry, 110 JavaScript chunks under the 250 KiB ceiling) and `git diff --check`. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.
- Confirmed `projects.created_at` is non-null database data returned by the existing permission-scoped `/projects` `SELECT p.*`; produced the additive-column review package and received explicit `Duyệt, làm đi` approval for date-only `vi-VN` presentation after progress, an unchanged sort contract and no backend work.
- Added the static `Ngày tạo` desktop column after progress, labelled mobile date metadata, the explicit `Project.created_at` frontend contract and a compact six-column desktop grid without adding a new sort option.
- Extended the focused responsive contract test and central/frontend/project-planning mapping for the source-backed creation-date presentation.
- Technically verified the current frontend source after the created-date addition with all 71 tests, the TypeScript/Vite production build (2.40s), the bundle budget (121.5 KiB gzip eager entry, 110 JavaScript chunks under the 250 KiB ceiling) and `git diff --check`. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.
- Confirmed the mixed-case sortable headers are caused by Tailwind Preflight resetting `text-transform` on buttons, while the static creation-date header is absent from `ProjectExplorerSortKey`; produced the review package and received explicit `OK triển khai` for uppercase desktop headers plus frontend-only creation-date sorting.
- Added explicit uppercase styling to every sortable desktop header and promoted `Ngày tạo` to the same accessible ascending/descending header control; the mobile selector now includes oldest-first and newest-first creation-date choices.
- Added timestamp-based creation-date comparison with missing/invalid values last in both directions, preserved API order until a sort is selected, and retained the existing stable project tie-breakers and ID-based expansion state.
- Extended focused Project Explorer regression coverage and updated central/frontend/project-planning mapping for the uppercase-header and creation-date sort contract.
- Technically verified the current frontend working tree with all 74 tests, the TypeScript/Vite production build (2.74s), the bundle budget (121.5 KiB gzip eager entry, 110 JavaScript chunks under the 250 KiB ceiling) and `git diff --check`. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.
