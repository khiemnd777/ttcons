# [Closed] Compact Project Workspace Header

Closed: 2026-09-14

Status: IMPLEMENTED; TECHNICALLY VERIFIED; NOT UX ACCEPTED.

## Delivered

- Added opt-in `PageHeader density="compact"` without changing the standard global-page default.
- Applied compact page chrome to all routed `/projects/:id/*` business surfaces that render page headers, including current and legacy Accounting modes.
- Kept `ProjectWorkspaceLayout` as the single visible owner of project code/name/client/PM/status and the compact Developer Sample Data badge.
- Removed the duplicated visible project identity/header from Project Overview, retained an accessible h1, and combined the existing view switcher with secondary `Mở kế hoạch` navigation.
- Kept the sole primary lifecycle CTA in the existing `Cần làm tiếp theo` band.
- Preserved responsive wrapping/truncation, existing actions, permissions, loading, empty and error behavior.
- Updated central, frontend UI and project-planning mapping references plus focused source-contract coverage.

## Verification

- Focused frontend regression set passed: 13/13.
- Fresh `cd frontend && npm run build` passed on the final source working tree (`tsc --noEmit` and Vite).
- Full frontend suite passed 472/473. The remaining unrelated `projectEvidenceColumns.test.ts` expectation looks for `Giai đoạn`, while the unchanged `EvidenceGallery.tsx` source currently renders `Nguồn / Giai đoạn`.
- `git diff --check` passed before closure.

## Scope limitation

Browser/UAT was not requested, so visual and responsive acceptance against the running application has not been performed.
