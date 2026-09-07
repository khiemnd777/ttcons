# Current States

## Mobile construction report — approved 2026-09-07

Status: IMPLEMENTED; prior non-browser verification below is historical; final-source verification is recorded next; NOT UX ACCEPTED. User approved the review package and text mockups with “OK triển khai.” Source baseline: clean `a701a3a`; implementation is included with the shared-evidence changes. Shared-evidence reuse now supersedes this plan's reportMode/gallery branch while preserving the mobile form, dialog and camera protections.

Purpose: supervisors capture several site photos, enter daily work and save/send a report comfortably on a phone.

Final-source verification carried forward (2026-09-08): macOS / Node v23.11.0; 66 focused frontend tests passed, 0 failed/skipped; TypeScript/Vite build passed at 00:31:58 +07 (1,934 modules, 4.40s Vite stage). Frontend source SHA-256 with NUL separators: `3adf9fb440be2b0ff937db0ccfb83bb1b9affeb6695dce3487a3af4528ab7497`. Browser/device checks remain pending. Durable ownership is in `.agents/skills/consflow-module-memory/references/frontend-ui-utility-map.md` and the Construction Operations domain map.

### Will be done / pending acceptance

- Browser/device acceptance remains pending explicit browser authorization. Check 360/390/440px plus landscape, open resource disclosures, software keyboard, camera permission denial/system fallback, repeated photos, nested viewer close and discard cancel/confirm. No browser or camera hardware check has run.

### Done

- Loaded central/frontend/backend/triage and Construction Operations/Quality domain maps; read Dialog, report, shared capture, linked composer and save/upload owners.
- Review identified fixed 760px resource tables, unconstrained grid containment, stacked mobile footer and single-shot camera close. Exact browser overflow measurements remain unverified.
- Baseline 15 existing construction/evidence tests passed on macOS, Node 23.11.0. These are not responsive or camera-hardware acceptance.
- Implemented `Dialog.mobileFullscreen` below 768px with VisualViewport sizing, safe areas and fixed header/footer around one body scroller. Other dialogs retain default sizing.
- Implemented photo-first form, two-action mobile footer, compact recipient selection, optional issue/resource/note disclosures and one shared input tree for desktop resource tables/mobile cards. Populated/error groups open; repeated validation reopens groups; new resource rows focus their first input.
- Originally implemented opt-in `EvidenceCapture.reportMode` with a compact gallery/editor. Shared-evidence reuse removed that branch and migrated every caller to the existing WorkItem table. Continuous camera, fallback, 50-total-report cap and late-callback/overlapping-shutter protections remain shared.
- Added discard confirmation, save-time dismissal/input lock, before-unload warning and per-open form initialization so query refresh cannot erase an in-progress draft. Files still live only in memory until the existing save flow.
- Updated central, frontend, root-cause and Construction Operations domain maps, including removal of the former mobile-wide-table invariant. Backend contracts/schema and Quality/Acceptance business rules did not change.

### Verification — 2026-09-07, observed by 23:07 +07

- Environment: local macOS, Node `v23.11.0`, frontend installed dependencies; source `a701a3a` plus this plan's uncommitted working-tree changes.
- `cd frontend && npm run lint`: passed.
- `cd frontend && node --experimental-strip-types --test tests/constructionReportMobile.test.ts tests/constructionOperationsPresentation.test.ts tests/evidenceFileTypes.test.ts tests/acceptanceWorkItemPicker.test.ts tests/evidenceUploadQueue.test.ts tests/formValidationNavigation.test.ts`: 34 passed, 0 failed; 272ms reported. Includes real React server renders with fixed query-cache fixtures (no HTTP/DOM), responsive single-control-tree assertions and camera session guards. An initial test-harness ESM/CJS React Query context mismatch was corrected; the final run has no failing tests.
- `cd frontend && npm run build`: passed TypeScript and Vite `6.4.3`; 1,929 modules, Vite stage 4.56s. Final build includes all frontend source changes; subsequent edits are plan notes only.
- `git diff --check`: passed. No commits, API mutations, DB reads/writes, migrations, Docker restart or deployment performed. Existing database/migration state and live-served frontend revision were not checked.
- Known limitations: DOM width/scroll, actual touch/focus/keyboard behavior, camera hardware and browser UAT remain unverified. Existing sequential upload/partial-save recovery is unchanged; this work does not add offline persistence or guarantee upload retry deduplication.

Scope: frontend report presentation/capture only. Existing save/send API, required photo/work item, permissions, formal revisions and linked check/action persistence remain authoritative. Mapping updates must include fullscreen opt-in, report disclosures/resource cards, compact capture/continuous capture and discard handling.
