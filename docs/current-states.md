# Current States

## Detail UI redesign — codex/ui-detail-redesign

- Authorization: user confirmed `OK triển khai` after the UI inventory and dedicated branch creation. Base revision: `5044b6f`.
- Purpose: identify each record immediately, emphasize decisive information, and organize detailed content by its business purpose.
- Scope: BOQ, Procurement, discussion source viewers, supplier quotation, stock document, partner, rework/replanning, planning daily reports/meeting minutes/checklists, role and audit details. Legacy AP/AR/payment scope remains read-only discussion previews.
- Done: shared `RecordDetailLayout.tsx`/`recordDetail.css`, named sections and optional summary/collapsed technical fields in `RecordDetailDialog`; all inventory detail groups integrated. BOQ retains four tabs; Procurement/quotation/stock lines use container-responsive tables/cards; discussion viewers retain source validation and read-only boundaries; planning viewers share `PlanningRecordContext`. Repeated header descriptions removed. Relevant central/frontend and six domain mappings updated.
- Done: read-only source/impact review, inventory and text mockups supplied in conversation. Existing API calls, query/scoping logic, mutation handlers and source-navigation callbacks remain unchanged. No backend, migration, seed or database changes.
- Status: IMPLEMENTED and TECHNICALLY VERIFIED in the local Node/Vite environment. NOT UX ACCEPTED.
- Will be done: browser checks on desktop/narrow dialog/mobile, visual review against the supplied screenshots and user UX acceptance after explicit browser authorization. Keep plan open until acceptance; no achieved visual-quality rating is claimed.

### Verification — 05-09-2026, 16:16 ICT

- Source: uncommitted working tree on `codex/ui-detail-redesign`, base `5044b6f`. SHA-256 of all 301 sorted `frontend/src` file paths/content: `6803fd2d39aff3b4ab2e8a3901c8a87422c3f40db41a29694b579d52f2970c0e`. No frontend source edits after the final build/tests.
- Environment: local macOS shell, Node `v23.11.0`, npm `11.4.2`, Vite `6.4.3`. Existing localhost browser session was not opened/verified. Database/migration state was not queried because this is frontend presentation work.
- `cd frontend && npm test`: 363 passed, 0 failed, 0 skipped; duration 2.35s. Includes four real-component SSR checks in `recordDetailPresentation.test.ts` for numeric source values, read-only BOQ actions, distinct procurement quantities, escaping/false/zero/hidden fields and collapsed audit metadata. SSR uses fixed fixtures, CSS omitted and a defined Vite environment; it is not live API or browser coverage.
- `cd frontend && npm run build:budget`: exit 0; TypeScript + Vite production build succeeded, Vite phase 6.17s. Entry 306.53 kB raw / 81.3 KiB gzip; entry and every JS chunk met the configured 500 kB raw / 250 KiB gzip limits.
- `git diff --check`: exit 0. Logs: `/tmp/consflow-ui-detail-tests.log`, `/tmp/consflow-ui-detail-build.log`.
- Resolved verification issue: first new SSR harness run lacked `import.meta.env` in CommonJS; the test bundler now defines the Vite environment and passes. The initial build preceded final header cleanup and is superseded by the final `build:budget` run.
- Skipped: browser rendering, responsive interaction, keyboard/scroll checks, UAT, Docker runtime rebuild and backend/database tests. No known remaining failure in the executed checks. Visual acceptance remains outstanding.
