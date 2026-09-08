# [Closed] Project Explorer — Ngày cập nhật và sắp xếp — approved 2026-09-08

Status when closed: TECHNICALLY VERIFIED; NOT UX ACCEPTED because browser/UAT verification was not requested.

Purpose: expose each project master record's `updated_at` in `/projects` and let users sort the server-authoritative Explorer and its export by that timestamp.

## Pending acceptance

- Browser acceptance remains pending explicit authorization. Validate the desktop grid and mobile stacked date labels, then both ascending and descending `Ngày cập nhật` sort requests.

## Done

- User approved the review package with “Duyệt triển khai theo review package này.”
- Added the `Ngày cập nhật` desktop column and mobile label, using the existing `dateText` `dd-mm-yyyy` presentation; sorting still compares the complete persisted timestamp.
- Extended the Explorer sort allowlist and both Explorer/export SQL order builders with `updated_at`, while preserving the existing `created_at desc` default.
- Kept this field distinct from the separate actor-scoped “Gần nhất” activity/update indicator; no activity/read-state, workflow, permission, audit or migration behavior changes.
- Updated the typed frontend `Project` contract, focused tests and the project Explorer mapping references.

## Verification — 2026-09-08

- Environment: local macOS working tree; Go build cache redirected to `backend/.gocache` because the sandbox cannot write the host default cache.
- `cd frontend && node --experimental-strip-types --test tests/projectExplorerSort.test.ts tests/projectExplorerResponsive.test.ts tests/projectExplorerPagination.test.ts`: passed, 14 tests.
- `cd backend && GOCACHE="$PWD/.gocache" go test ./internal/core`: passed.
- `cd frontend && npm run build`: passed TypeScript and Vite `6.4.3`; 1,934 modules transformed, Vite stage 4.64s.
- `git diff --check`: passed with no whitespace errors.
- Skipped: browser/DOM UAT and direct live API/database verification; therefore NOT UX ACCEPTED. The full backend suite was not run; focused core coverage ran instead.
