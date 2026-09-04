# [Closed] Utility Dock — đóng popup `Gắn vào`/`Nhắc` khi click bên ngoài — 2026-09-04

- Closed on 2026-09-04 at the user's request: “Đóng plan, commit, push”.
- Durable behavior and triage knowledge was promoted to the central, frontend, root-cause and Project Planning maps before closure. Archival itself changes no runtime behavior and requires no additional mapping update.

Status: IMPLEMENTED · TECHNICALLY VERIFIED · NOT UX ACCEPTED (2026-09-04)

Remaining if reopened:

- Browser/UAT verification only when explicitly requested; repository policy kept this implementation non-browser, so UX is not yet accepted.

Completed:

- Confirmed the reported `/projects` screenshot: compact composer popup `Gắn vào` remained open after an outside click.
- Identified the root cause in `frontend/src/modules/work-items/ActivityComposer.tsx`: compact `Gắn vào` and `Nhắc` used uncontrolled native `<details>` without per-instance state, refs or outside-dismiss listeners.
- Built a pre-fix source-contract signal, presented the bugfix review package and received explicit `OK triển khai` confirmation.
- Added reusable controlled disclosure behavior to the shared compact Activity Composer. Document `pointerdown` closes only when the event target is outside the active disclosure; pointer-down inside keeps it open.
- Added `Escape` dismissal with `stopPropagation()` so the global Utility Dock handler does not also minimize the project conversation.
- Kept disclosure state isolated per mounted project composer and preserved draft, selected target/mention, payload, API, permission, activity history and Dock layout contracts.
- Extended focused Utility Dock regression coverage and updated central/frontend/root-cause/Project Planning mappings.
- Verification on the uncommitted working tree based on `c259b3c`: focused Utility Dock tests passed 4/4; full frontend `npm test` passed 317/317; `npm run build:budget` passed TypeScript/Vite and the 250 KiB gzip gate with 126 chunks (entry 159.5 KiB); `git diff --check` passed. Vite emitted its existing informational raw-chunk warning, but the enforced gzip budget had no violation.
- Browser interaction/UAT was not run, so this closed implementation remains `NOT UX ACCEPTED`.
