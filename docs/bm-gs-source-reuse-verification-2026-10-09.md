# BM source reuse verification — 2026-10-09

## Scope and acceptance limits

The user approved the six high-confidence reuse corrections from the all-eight-template field audit. Status: **IMPLEMENTED / TECHNICALLY VERIFIED / NOT UX ACCEPTED**.

Browser, actual-button downloads, native Google Sheets rendering and physical-device camera checks are **NOT_RUN** for this delta. Standard development containers were not rebuilt or restarted; their previously verified build does not prove these new sources. This is not full-client/all-eight-template acceptance. Existing input fields remain; structural field removal is outside this bounded correction. No new user fields, migration, dependency, chooser or export-time source/photo selection was introduced.

## Resulting source contracts

| Template | Reuse | Exact limit |
|---|---|---|
| BM02 | Submitted incoming-acceptance material name, delivered quantity, unit and item comment | Same project, receipt Work Item, effective item Work Item and received business date as the report. Canonical source-view permissions and parent visibility apply. Nonempty manual rows override; empty defaults permit actual linked receipts. More than 200 rows fails explicitly. No project/date-only guessing, usage substitution, accepted quantity, supplier-as-origin or specification fabrication. Missing exact receiving relation remains unavailable. |
| BM03 | Existing row comment as a labeled source note | Equal quality/measurement notes are not duplicated. Generic comment never establishes quality result or measured quantity. |
| BM04 | Linked check location | Only an active check from the same project. Native location override wins; an existing audited explicit location write/clear blocks inheritance. SQL NULL alone cannot distinguish unset and clear, so the canonical location-write audit must remain retained. Effective Work Item visibility remains checked. |
| BM06 | Actual generated Work Item lifecycle state | Persisted form/parent/project creation relation, actor task scope, stable unique row identity. Legacy requires complete unique unchanged original content and index. Explicit state wins; no progress/issue date inference. Cancelled, hidden or ambiguous tasks stay unavailable. |
| BM07 | Live in-app camera shutter instant | ISO instant recorded immediately before drawing the frame, retained through encoding, review, continuous capture, queue, multipart and retry. Manual correction/clear remains. Device clock is the source. Imported files/system-camera fallback never receive mtime/name/upload substitutions. |
| BM08 | Existing item notes in labeled supplemental source sections | Notes remain distinct from defect and physical assessment; no acceptance-to-handover checklist substitution. |

## History and clear handling

Delivery captures reused native facts under the existing report transaction. Actorless repository reads can restore the already-frozen structured snapshot, but perform no live enrichment. Mutable export is actor-aware. Old missing frozen source facts remain missing.

Native workflow submission captures source facts in the existing submitted event metadata after generated task creation, in the same transaction. Amendment creates new current-revision event facts. The loader validates version/project/Work Item/form/revision and never queries current source facts as a substitute for old history. Published definitions, canonical answers, signatures and previous events remain unchanged.

Canonical optional-string normalization removes an empty state. Independent review found that a manual state clear could therefore be rederived. A canonical submit → amend → clear → export test failed before correction. New event facts preserve the clear as `state=''` with internal `state_source.source_type='explicit_clear'`, matched by unique row identity. Later amendments retain that observation, including reordered rows. Initial default-empty state still derives normally. Root reproduced and repaired the same loss in the synchronized current Operations export. That path imports only the current matching explicit-clear marker; other lifecycle values remain live and scoped. Both native export entrypoints are tested.

Source-change comparison uses a private clone enriched under the original capture-author scope and exposes only the changed flag. Missing `delivery_status` falls back through `cleanText(status)`; the previous `fmt(nil)` prevented the branch from running. Unchanged derived facts compare equal; actual task lifecycle change compares different. Comparison never exposes newly derived child facts to a parent-only viewer.

## Source and environment

- Repository: `/Users/khiemnguyen/Works/ttcons/ttcons`, main HEAD `75dc44707785d399ed2408959bf784928a544e96`, retained uncommitted native implementation plus this delta. No Git delivery.
- Source/config/test manifest: `/private/tmp/consflow-bm-reuse-source-manifest.json`, 959 files, SHA256 `f658859b6fb567530ca64c25c0c56df76542898ef7313848cf7d9513cf89e97e`.
- Backend integration: dedicated existing synthetic UAT PostgreSQL container `consflow-bmgs-uat-pg`, loopback port 55532, schema-isolated custom migrations through V157. Each integration fixture uses its own schema and cleanup. Main development/production database was not migrated or modified in this turn.
- Test launcher: `/private/tmp/consflow-bm-reuse-run.py`; credentials are supplied only in process environment and omitted from documentation/output. Go caches use private tmp paths.
- Backend binary: `/private/tmp/consflow-bm-reuse-server`, build mtime `2026-10-09T15:55:48Z`. It was not launched or deployed.
- Frontend build output mtime: `2026-10-09T15:39:03Z`; no frontend source changed afterward.

## Checks run

| Check | Observed result |
|---|---|
| `python3 /private/tmp/consflow-bm-reuse-run.py go test ./internal/core -run 'Test(Native|BMGS|WorkReport|ConstructionDaily)' -count=1` | PASS, 84.441s; real isolated DB plus pure/reopened XLSX cases |
| `node --test tests/cameraCaptureMetadataLifecycle.test.ts tests/nativeEvidenceCaptureMetadata.test.ts tests/constructionReportMobile.test.ts tests/evidenceLifecycle.test.ts` | PASS 34/34, 310.469ms; hook/callback lifecycle, not a physical camera/browser |
| `npm run build` in frontend | TypeScript/Vite PASS; Vite 4.97s |
| `python3 /private/tmp/consflow-bm-reuse-run.py go vet ./internal/core` | PASS, exit 0 |
| `python3 /private/tmp/consflow-bm-reuse-run.py go build -o /private/tmp/consflow-bm-reuse-server ./cmd/server` | PASS, exit 0 |
| Scoped `gofmt -l` on touched Go source/tests | No output |
| `git diff --check` | PASS |
| Independent GPT-6.1 Sol High review | No remaining actionable P0–P2 finding in bounded changes; final focused isolated test run PASS 10.954s |

The initial broader suite exposed a pre-existing assertion that counted all V154 inherited native-field markers as two versions. V157 clones both templates. The corrected test separately checks two original definitions and two assessment definitions with the same actual_quantity assertion; no application behavior or migration was changed to satisfy the test.

## Important owners and mapping

Source derivation/freezing: `native_daily_source_reuse.go`, `native_workflow_source_reuse.go`, `native_work_report_evidence.go`; native export services and repository workflow/report hooks. Projection notes/location: `bm_gs_export_projection.go`, `service_native_bm_gs_owners.go`. Camera: shared `EvidenceCapture.tsx`, using existing `EvidenceUploader`, metadata/multipart and retry owners.

New regressions: `native_bm_reuse_projection_test.go`, `native_daily_source_reuse_test.go`, `native_workflow_source_reuse_test.go`, `cameraCaptureMetadataLifecycle.test.ts`. Existing native capture version assertion is synchronized with V157. Central/backend/frontend/triage and Construction/Quality/Materials/Workflow domain maps were updated in the same turn.

Next acceptance scope: current-build browser flows and unchanged files downloaded from the existing native export buttons, then native Google Sheets checks. Structural removal/merging of added UI fields and unresolved lower-confidence source equivalence remain separate decisions.
