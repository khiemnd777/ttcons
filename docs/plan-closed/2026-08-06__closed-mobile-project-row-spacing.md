# [Closed] Mobile Project Row Spacing

Status: closed

Closed at: 2026-08-06

Verification limitation:

- Browser/UAT was not requested. The implementation was technically verified but remained `NOT UX ACCEPTED` when the plan closed.

Will be done:

- Separate top-level project rows by 12px on narrow screens and present each project plus its optional expanded workspace as one bordered, rounded card.
- Preserve the existing dense divided explorer rows from `lg` upward without changing project data, actions, permissions, filters or API behavior.
- Add focused regression coverage, update central/frontend/root-cause mapping and run non-browser frontend verification. Browser/UAT remains skipped unless explicitly requested in the same turn.

Done:

- Confirmed from the supplied 430×932 Edge screenshot that mobile project rows are visually attached.
- Proved from `ProjectsPage.tsx` that the top-level list owns only `divide-y divide-line`; individual rows already have internal `px-4 py-3`, so the defect is missing inter-row grouping rather than missing content padding.
- Produced the review package and Mockup as Text, then received explicit `OK triển khai` for 12px mobile spacing, one card per project with its expanded content attached, and unchanged desktop density.
- Added a narrow-screen project-list surface with 12px vertical spacing and a quiet slate background; each project row and its optional expanded workspace now share one bordered, rounded white card.
- Restored the existing compact list contract from `lg` upward through zero row spacing, transparent list background, zero outer padding, divider rows and removal of mobile card border/radius/shadow.
- Added a focused source-contract regression that failed before the layout change and passes afterward; updated central/frontend/root-cause/project-planning mapping for the responsive project explorer contract.
- Technically verified the current frontend source with all 43 tests, the production TypeScript/Vite build, the bundle budget (121.3 KiB gzip eager entry, 105 JavaScript chunks under the 250 KiB ceiling) and `git diff --check`.
