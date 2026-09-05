# [Closed] My Work search panel and scope label — 2026-09-05

Closed by explicit user request: “Đóng tất cả plan, commit.” Both active plans were moved from `docs/current-states.md`; no active plans remain.

Closure is administrative and does not establish UX acceptance. The implementation and verification records below retain their original scope. The later scope-label build supersedes the earlier search-panel build for the final `MyWorkPage.tsx` source.

## [Closed] My Work — Rút gọn nhãn phạm vi Tất cả

- Request: user explicitly requested `Tất cả được phép xem` → `Tất cả` on 2026-09-05.
- Status: IMPLEMENTED; TECHNICALLY VERIFIED on the local working tree; NOT UX ACCEPTED.
- Source: existing working tree based on `42df6b2`, including the earlier search-panel changes below.
- Matched owner: `frontend/src/modules/my-work/MyWorkPage.tsx`; maps: `module-keyword-map.md` and `frontend-ui-utility-map.md`; targeted searches: `Tất cả được phép xem|scopeLabel|scopes.map`.
- Done: changed the shared `all` display label used by the scope rail and quick-filter heading; updated the existing presentation assertion and both mappings.
- Done verification (2026-09-05, build started around 14:14 +07): `cd frontend && npm run build` passed TypeScript and Vite 6.4.3 (1,912 modules; Vite bundling 4.22s). `node --experimental-strip-types --test tests/myWorkListPresentation.test.ts` passed 16/16; `git diff --check` passed. No failing checks; Node emitted its experimental type-stripping warning. Local macOS, Node v23.11.0/npm 11.4.2; tested `MyWorkPage.tsx` SHA-256 `60a14c64f7cc2aadd69040270785379233c440fe5e845f87fc6c1d340f33f43f`. Frontend source was unchanged after verification; subsequent edits only record these results.
- Deferred at closure (not an active plan): browser/UAT acceptance only when requested; no implementation work remains for this label change.
- Scope: wording only on My Work; no API/database/migration changes. Browser/UAT is skipped because it was not requested.

## [Closed] My Work — Canh hàng khối Tìm công việc

- Approval: user confirmed `OK triển khai theo mockup` on 2026-09-05, after the text review and generated image showing collapsed/expanded filters.
- Status: IMPLEMENTED; TECHNICALLY VERIFIED for the local production build and focused existing regressions; NOT UX ACCEPTED on the running app.
- Source baseline: clean working tree at `42df6b2`.
- Matched owner: `frontend/src/modules/my-work/MyWorkPage.tsx` (`FilterBar`, local `FilterSelect`) and `myWorkFilters.css`; shared dependency `modules/_shared/AsyncProjectPicker.tsx`.
- Mapping: central `module-keyword-map.md`, `frontend-ui-utility-map.md`, `root-cause-triage-map.md`. First targeted searches: `Tìm công việc|FilterBar|AsyncProjectPicker|FilterSelect`.
- Evidence/root cause: supplied screenshot shows the search input above the project control and stretched action buttons. The existing grid includes a labelled project picker alongside an unlabelled search form; stretch increases button height, while the input keeps its own height. Record kind is outside that grid.
- Done: inspected the screenshot, owner source, shared project picker, existing My Work test seams and repository state; user approved the visual mockup. Implemented the full-width keyword/search row, labelled primary filter row, 40px controls, 12px column gaps, scoped panel-width container queries, two-column advanced filters and full-width sorting. Reused the unchanged async picker and preserved filter handlers/queries/permissions. Updated central, frontend and triage mappings.
- Done verification: `cd frontend && npm run build` passed (TypeScript + Vite 6.4.3; 1,912 modules; Vite bundling 4.86s). The emitted `dist/index.html` timestamp is `2026-09-05 14:10:38 +07`; the My Work CSS asset includes both named container rules. `node --experimental-strip-types --test tests/myWorkListPresentation.test.ts tests/myWorkQueue.test.ts tests/myWorkAttentionLoading.test.ts` passed 24/24 tests. `git diff --check` passed and the final diff remains scoped to the panel plus plan/mapping maintenance. No failing checks; Node emitted its standard experimental type-stripping warning.
- Verified source/environment: working tree based on `42df6b2`, macOS Darwin 25.6.0 arm64, Node v23.11.0/npm 11.4.2. SHA-256: `MyWorkPage.tsx` = `955563bd769f5a7bffa8511879555ee4ec731c085076bc8b4e940624586f92d6`; `myWorkFilters.css` = `b8885c37f9d7d03c4f83f82b99c79691a3bf6c238038b5fb89445bbfaea9ad91`. Frontend source was not changed after the build; subsequent edits only record verification here.
- Deferred at closure (not an active plan): browser/UAT visual acceptance when requested, covering collapsed/expanded filters, actual panel widths, both accounting visibility variants and project-picker long names/error states.
- Scope limits: browser/UAT skipped because no browser verification was requested in this turn. No backend, API, database or migration changes; database/migration runtime state was not inspected. Local source/build checks do not establish acceptance on the running Docker/browser environment.

## Closure checks

- Baseline: `42df6b23a41dcb40703056f28bd29f90a2e76285` plus the existing My Work source, CSS, test and mapping changes in this commit. No application code was edited during closure.
- Confirmed current SHA-256 values match the recorded verification: `frontend/src/modules/my-work/MyWorkPage.tsx` = `60a14c64f7cc2aadd69040270785379233c440fe5e845f87fc6c1d340f33f43f`; `frontend/src/modules/my-work/myWorkFilters.css` = `b8885c37f9d7d03c4f83f82b99c79691a3bf6c238038b5fb89445bbfaea9ad91`.
- Existing final build artifact `frontend/dist/index.html` has timestamp `2026-09-05 14:15:04 +0700`. Build/test results above are prior implementation evidence; these commands were not rerun for administrative closure. `git diff --check` was rerun and reported no whitespace errors.
- Browser/UAT, backend/DB integration, migrations and Docker/deployment were not verified during closure. UI remains NOT UX ACCEPTED. No runtime database or migration state is inferred from the frontend checks.
- Existing central, frontend and triage mapping updates are included in the commit. Archiving these plans needs no further mapping update because it changes no feature behavior, ownership, route, helper, domain invariant or agent workflow rule.
