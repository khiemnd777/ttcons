# [Closed] Mobile field focus without browser auto-zoom — 2026-09-14

- Closed on 2026-09-14 at the user's request: “Đóng plan, commit, push”.
- Durable ownership, behavior and diagnosis knowledge lives in the central, frontend and root-cause maps. Archival changes no product behavior and requires no additional mapping update.

Status: IMPLEMENTED · TECHNICALLY VERIFIED · NOT UX ACCEPTED

## Remaining if reopened

- Browser/device UAT in portrait and landscape when explicitly requested.
- The repository-wide frontend suite has one pre-existing unrelated failure in `projectEvidenceColumns.test.ts`: the test expects separate `Giai đoạn` and `Đầu việc` headers, while untouched `EvidenceGallery.tsx` renders `Nguồn / Giai đoạn` and `Đầu việc`.

## Will be done

- Add one global mobile/touch typography guard for editable `input`, `select`, and `textarea` controls.
- Preserve user pinch-zoom and desktop typography.
- Remove redundant scoped 16px field overrides from Construction Report and Evidence Capture.
- Add a focused CSS regression test.
- Update the central, frontend, and root-cause mapping references.
- Run the focused test, the frontend test suite, and the frontend build.

## Done

- Confirmed the worktree was clean before implementation.
- Confirmed shared form controls commonly use Tailwind `text-sm` (14px).
- Confirmed Construction Report already prevented the symptom locally with 16px controls.
- Confirmed there was no global focus-scale handler and the viewport preserved user zoom.
- Added the focused regression test and observed it fail before the fix because the global guard was absent.
- Added the global mobile/touch 16px field guard without changing viewport metadata.
- Removed the now-redundant scoped 16px declarations while retaining layout rules.
- Updated the central, frontend, and root-cause mapping references for the shared mobile focus typography owner and diagnosis path.
- Verified the compiled production CSS contains the touch/narrow media rule and 16px field declaration while built HTML retains user zoom.
- Focused regression `node --experimental-strip-types --test tests/mobileInputZoom.test.ts` passed.
- Frontend build `npm run build` passed on the current frontend source state in 5.08 seconds.
- All frontend tests except `projectEvidenceColumns.test.ts` passed; rerunning every other frontend test file exited successfully.
- `git diff --check` passed.

## Scope notes

- Shared `SimpleForm`, `CurrencyInput`, `PasswordInput`, Workflow forms and module-local fields inherit the global CSS behavior without JSX or payload changes.
- Checkbox, radio, range, button-like and hidden inputs remain excluded.
- No backend, API, RBAC, audit, report, database, migration or seed behavior changed.
- Browser/device UAT was not authorized or run, so UX remains not accepted.
