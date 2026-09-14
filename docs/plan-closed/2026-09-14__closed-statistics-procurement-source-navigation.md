# [Closed] Statistics procurement source navigation

Closed: 2026-09-14

Status: TECHNICALLY VERIFIED; NOT UX ACCEPTED.

## Delivered

- `/statistics/procurement` now groups rows by công trình, trạng thái mua sắm, and trạng thái ngân sách; each row contains the source project ID, code, and name.
- Desktop and mobile `Mở mua sắm` links now open the exact project's procurement workspace at `/projects/:id/procurement`, including when the Statistics filter is `Tất cả công trình được phép xem`.
- Statistics PDF, XLSX, and CSV exports use the same project-level procurement grain.
- The legacy `/reports/procurement` aggregate remains grouped only by status and budget status.
- Module and domain mapping references were updated with the new reporting contract and navigation ownership.

## Verification

- `cd backend && go test ./internal/core -run TestProcurementStatisticGroupsByProjectAndKeepsLegacyAggregate -count=1` passed.
- `cd frontend && node --experimental-strip-types --test tests/reportsResponsive.test.ts` passed (3/3).
- `cd frontend && npm run build` passed.
- `git diff --check` passed before closure.

## Scope limitation

Browser/UAT and local-stack restart were not requested, so visual and click-through acceptance against the running application has not been performed.
