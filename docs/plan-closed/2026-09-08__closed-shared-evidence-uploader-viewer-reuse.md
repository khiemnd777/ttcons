# [Closed] Shared evidence uploader and viewer reuse — approved 2026-09-07

Closed by explicit user request on 2026-09-08: “Đóng plan, commit, push.” Closure archives the implementation record; browser/device UX remains NOT UX ACCEPTED. Durable component ownership, invariants and regression paths are already recorded in AGENTS.md and the central/frontend/backend/triage/domain maps.

Status: stored-card correction IMPLEMENTED and TECHNICALLY VERIFIED within the non-browser scope below; NOT UX ACCEPTED. The user's follow-up screenshot exposed a missed saved-evidence card; earlier verification covered uploader/viewer composition, not all visible attachment UI. User approved the reviewed reuse plan with “Duyệt, triển khai.” Baseline: `a701a3a` plus the existing uncommitted mobile-report changes recorded below. Those changes were preserved, with the report capture branch superseded by the approved shared uploader. The implementation was verified before the closing commit.

Purpose: use the existing WorkItem evidence upload table and full record viewer consistently for photos, PDF, DOCX and Excel across field operations and related workflows.

### Unverified at closure

- Browser/device/camera UAT remains pending explicit browser authorization. Confirm real photo/PDF/DOCX/XLSX content, narrow panel/mobile layout, native chooser/source-picker resume, camera denial/fallback/continuous capture, nested viewer focus/close, and parent-save retry on fixture records. No browser was launched during this implementation turn.
- No offline persistence or idempotency across a lost form/file session is claimed. Legacy DOC/XLS keep original-file fallback. DOCX/XLSX decoding and layout in the user's browser are not proven by server render or hook tests.

### Done

- Follow-up diagnosis: ExistingContextEvidence still rendered ConstructionEvidenceButton's original FileImage/name/Xem markup. The previous change only replaced its opened dialog. Two focused server-render regressions reproduce the omission (2 failed before the correction). The previous broad phrase “Bỏ giao diện demo” overstated saved-card coverage; the demo **upload** branches were removed, while these stored cards were missed.
- Corrected all five callers of that stored-card adapter: editable check context, legacy check result, action before/resolution evidence and action event evidence. The existing EvidenceGallery::EvidenceCard is now exported and reused; the construction adapter owns only context labels and data resolution. Protected photo preview, full read-only viewer and original-file opening use shared hooks. Loading/error/retry lives in the same card presentation, and draft unlink remains independent of viewing/loading; no evidence delete or parent-save contract change. Central/frontend/triage/Construction maps updated.
- Approved source components and callers were traced against local code. Diagnosis demonstrated differing capture branches and reduced viewer output. These diagnosis checks are not implementation verification.
- Extracted the existing WorkItem table to `_shared/EvidenceUploader.tsx`; both EvidencePanel and EvidenceCapture use it. Container-width layout preserves columns in wide hosts and labeled rows in narrow panels. Removed the demo upload cards, reportMode gallery/editor and old camera overlay. The continuous camera/session guard remains shared.
- Migrated all eight capture entrypoints below. Caller source, parent-save timing, selected evidence, context/result roles and caps remain explicit. Action/resolution accepts one file because its mutation links one evidence ID; checks/acceptance cap at 10, reports at 50 including existing selections. Save-time form/source/close locks preserve pending edits.
- Added in-memory File/draft/destination receipts, byte-progress/processing states, in-flight request reuse and independent retry with current name/caption. Partial upload or parent-save failure reuses successful IDs. Report append retains its own serial upload-and-link endpoint, latest completed snapshot and successful-row removal lock. Acceptance retains per-success checklist association updates.
- Routed fourteen reference entrypoints through full record composition. Complete records keep real metadata; ID-only and incomplete summaries resolve `GET /evidence/:id`. A created_at field does not prove a full record: the construction summary omits identity/source joins and is hydrated. Read-only callers receive no review controls.
- Added protected metadata handler/service using GetEvidenceDetail and CanViewWorkItem; the file service uses the same authorization path. Repository detail includes run-node source. No schema/migration changes. Original WorkReport revision record/file paths are preserved.
- Local File previews now reuse photo/PDF/DOCX/XLSX readers with local-only metadata. Protected/draft URL lifecycle tests cover record/path changes, late responses, errors, retry and cleanup.
- Updated AGENTS ownership rules plus central/frontend/backend/triage, Construction, Quality and Workflow maps. Inventory tests reject caller/owner drift and the obsolete capture branches.

### Adoption inventory — code and non-browser coverage

Eight uploader entrypoints were each server-rendered with the actual shared table (one uploader, no nested upload form). Browser interaction is pending for every row.

| # | Entrypoint | Source file |
|---|---|---|
| 1 | Create/edit check | `ConstructionCheckDialogs.tsx::CheckFormDialog` |
| 2 | Check result | `ConstructionCheckDialogs.tsx::CompleteCheckDialog` |
| 3 | Create action | `ConstructionActionDialogs.tsx::ActionFormDialog` |
| 4 | Resolution evidence | `ConstructionActionDialogs.tsx::ResolutionDialog` |
| 5 | Append report evidence | `ConstructionReportEvidenceDialog.tsx` |
| 6 | Report-linked check/action | `ConstructionReportLinkedComposer.tsx` |
| 7 | Acceptance page/outgoing panel editor | `acceptance/AcceptanceEditor.tsx` |
| 8 | Create/edit report | `ConstructionReportFormDialog.tsx` |

Fourteen viewer entrypoints were verified by code inventory and shared adapter/record/draft render and lifecycle tests. These are not fourteen browser interaction tests.

| Entries | Viewer entrypoints | Source file |
|---|---|---|
| 1–2 | Existing check context; legacy check result | `ConstructionCheckDialogs.tsx` |
| 3–5 | Before; resolution; event timeline | `ConstructionActionDialogs.tsx` |
| 6–7 | Acceptance criterion detail; selected evidence editor | `acceptance/AcceptancePage.tsx`, `AcceptanceEditor.tsx` |
| 8 | Linked handover evidence | `handover/HandoverChecklistSourceDialog.tsx` |
| 9–10 | Before/after material evidence | `work-items/MaterialSnapshotPanel.tsx` |
| 11 | Incoming acceptance evidence | `work-items/IncomingAcceptancePanel.tsx` |
| 12 | Surplus evidence | `work-items/SurplusHandlingPanel.tsx` |
| 13–14 | Subcontractor; owner/client reconciliation | `work-items/SubcontractorReconciliationPanel.tsx`, `OwnerClientReconciliationPanel.tsx` |

### Verification — stored-card correction 2026-09-08 00:31–00:32 +07

- Limitations first: NOT UX ACCEPTED. No browser, native file chooser/camera, real authenticated click/open interaction or device layout acceptance ran. SSR/query/transport fixtures do not prove the user's actual browser content. No backend/database/migration change in this correction; backend tests were not rerun and prior results below retain their original scope.
- Tested current source: `a701a3a` plus uncommitted working tree; `frontend/src` SHA-256 over sorted paths, NUL separators and file bytes = `3adf9fb440be2b0ff937db0ccfb83bb1b9affeb6695dce3487a3af4528ab7497`, identical before/after build. macOS, Node v23.11.0, Vite 6.4.3. Only plan notes changed afterwards.
- `node --experimental-strip-types --test` over the same twelve test files listed below: **66 passed, 0 failed, 0 skipped**, 488ms. Added real saved-context/card rendering, typed photo/PDF/DOCX/XLSX actions, thumbnail loading/error/retry, denied metadata, legacy original-file action, draft-only unlink and card-owner regression checks. Initial two red tests reproduced the omission; all assertions pass on current source. These are non-browser checks.
- `cd frontend && npm run build`: TypeScript and Vite passed; 1,934 modules, Vite 4.40s; completed 00:31:58 +07. `git diff --check` passed. Logs: `/private/tmp/evidence-card-final-tests.log`, `/private/tmp/evidence-card-final-build.log`.
- Read-only HTTP check of localhost:5173 served ConstructionEvidenceButton.tsx with HTTP 200, EvidenceCard present and FileImage absent. This confirms dev-server source delivery, not browser rendering. No form reload, user-data mutation, restart or production deploy.

### Prior verification — uploader/viewer source observed 2026-09-08 00:01–00:02 +07

- Limitations first: NOT UX ACCEPTED; no browser/device/native camera validation, live authenticated metadata UAT or production deployment. Tests use controlled transport/query fixtures and a disposable local database, not the user's business records. No new migration was added.
- Previously tested source: `a701a3a` plus the working tree at that checkpoint, including retained mobile edits. Source manifest SHA-256 (sorted paths plus bytes): `frontend/src` = `0b167ca56ea7a43c19e7baa1580164ba812979702fe0a2d4f0c45a02bfea0d40`; `backend/internal/core` = `bc2a1b3eff157c5e52a9ae7afd9128e32297f2a01507a8153e0aeb4e694555a1`. The frontend build below is historical and superseded by the stored-card verification above.
- Frontend environment: macOS, Node v23.11.0, installed dependencies. `node --experimental-strip-types --test` over evidenceLifecycle, evidenceUploadSession, evidenceComponentReuse, constructionReportMobile, constructionOperationsPresentation, evidenceFileTypes, acceptanceWorkItemPicker, evidenceUploadQueue, evidenceReferenceViewer, evidencePdfViewer, evidenceOfficeViewer and formValidationNavigation: **62 passed, 0 failed, 0 skipped**, 475ms final run. Earlier obsolete source assertions and fixture assumptions failed during migration and were corrected; no failure remains in this focused suite.
- `cd frontend && npm run build`: TypeScript and Vite 6.4.3 passed at 00:01:28 +07; 1,934 modules, Vite stage 4.65s. Final source was rebuilt after the last adapter fix. `git diff --check` passed.
- Backend environment: macOS Go 1.25.0, PostgreSQL 17 fixture at localhost:55479, UTF-8 database `consflow_evidence_reuse`, custom migrations 136/136. `GOCACHE=/private/tmp/consflow-evidence-go-cache CONSFLOW_TEST_DATABASE_URL=<fixture> go test ./internal/core -run 'TestEvidenceDetail|TestEvidenceAuthorizationRunsBeforeStorageOpen|TestWorkReport|Test.*EvidenceUpload|Test.*EvidenceFile' -count=1 -v`: **11 top-level tests / 43 including subtests passed**, no failures/skips, 1.268s. Covers actual metadata envelope/no-store, valid/invalid/missing/cancelled/scope denial, storage authorization, uploads, media validation and immutable WorkReport revisions. Fixture server was stopped after testing.
- Local runtime: `docker compose config --quiet` passed; built backend and recreated only `backend` using `docker compose up -d --no-deps backend`. Admin stayed running with bind-mounted frontend; HTTP-served capture includes EvidenceUploader and has no reportMode branch. Backend health returned success; unauthenticated metadata returned HTTP 401. Docker PostgreSQL 16 remains at 136 migrations; startup logged `applied=0 skipped=136`. Admin/backend running, PostgreSQL/Redis/Redis-cache healthy. No business-data test uploads, approvals, commits or production deploy were performed.
- Logs: `/private/tmp/evidence-final-tests.log`, `evidence-final-build.log`, `evidence-backend-tests.log`, `evidence-backend-build.log`. Build/test scopes above are the evidence; runtime health and non-browser tests do not establish UX acceptance.
