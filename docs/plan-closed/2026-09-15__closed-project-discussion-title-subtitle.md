# [Closed] Remove visible title and subtitle from Project Discussion

Status: IMPLEMENTED; FOCUSED TECHNICALLY VERIFIED; FULL FRONTEND SUITE HAS ONE OUT-OF-SCOPE PRE-EXISTING SOURCE-CONTRACT FAILURE; NOT UX ACCEPTED. User approved the review package and Mockup as Text with `OK triển khai`, then requested all plans be closed on 2026-09-15.

## Will be done

- Remove the visible `Trao đổi` icon/title/subtitle block from `ProjectDiscussionPage` while retaining its assistive-only page heading.
- Start the successful and loading states directly with the existing `ActivityThread` layout/skeleton.
- Preserve project identity in `ProjectWorkspaceLayout` and keep all discussion data, permissions, counts, workflow actions and responsive behavior unchanged.
- Add focused source-contract coverage, update the central/frontend/project-planning maps, then run the focused test, full frontend test suite and current frontend build.

## Done

- Review package and Mockup as Text approved by the user.
- Matched Project Planning / Project Activity owners and confirmed the visible duplication in `ProjectDiscussionPage` against `ProjectWorkspaceLayout`.
- Removed the visible icon/title/subtitle and its loading placeholder; retained the assistive-only route heading and made `ActivityThread` the first visible content.
- Added `projectActivityTabs.test.ts` coverage for the visual-title removal, heading retention and loading skeleton contract.
- Updated the central module map, frontend UI/utility map and Project Planning domain map with the new presentation contract.
- Working-tree verification on 2026-09-15: `projectActivityTabs.test.ts` passed 3/3; `projectWorkspaceCompactHeader.test.ts` passed 3/3; `npm run build` passed after TypeScript and Vite production compilation; `git diff --check` passed.
- Full `npm test` ran against the same relevant source state and has exactly one unrelated failure: committed `projectEvidenceColumns.test.ts` expects separate `Giai đoạn`, while committed `EvidenceGallery.tsx` currently renders `Nguồn / Giai đoạn`. Neither out-of-scope file was changed by this plan.
- Browser/UAT was not run because it was not explicitly requested; the result remains NOT UX ACCEPTED.

## Closure

- Closed by explicit user request on 2026-09-15.
- Durable presentation ownership and search guidance were promoted to the central module map, frontend UI/utility map and Project Planning domain map before closure.
