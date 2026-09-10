# [Closed] Discussion-driven workflow action

Status at closure: IMPLEMENTED · TECHNICALLY VERIFIED · NOT UX ACCEPTED

Known limitations at closure:

- Browser/UAT was not authorized; the interface remains `NOT UX ACCEPTED`.
- The unrelated pre-existing `projectEvidenceColumns.test.ts` expectation must be reconciled separately before the complete frontend test suite can pass.

Done:

- Review package and Mockup as Text approved by the user on 2026-09-09.
- User confirmed the prior loading fix opens the Work Item dialog; follow-up UAT exposed the post-submit artifact-action inconsistency.
- Review package and Mockup as Text for the streamlined generic post-submit artifact flow were approved with `OK triển khai`.
- Current behavior, owner files, API capability checks and standard component reuse paths confirmed from local code.
- Added `DiscussionWorkflowActionCard` above `Việc của tôi` with startup, current Work Item, read-only, complete/no-action, loading and retry presentation.
- Routed Project Detail startup entrypoints through the shared `ProjectStartupWizard`; the Discussion tab owns its lifecycle CTA and keeps only `Mở kế hoạch` in the page header.
- Reused `DiscussionWorkDialog`, the existing Work Item discussion scope/banner, `StatusBadge`, shared API client and server capabilities.
- Preserved the actor-scoped personal `responsible` queue and workflow/RBAC/audit contracts.
- Updated the central, frontend, Project Planning and Workflow Orchestration mappings plus focused regression coverage.
- User UAT found `DiscussionWorkDialog` frozen with simultaneous loading/error after opening the project-priority Work Item. Backend logs showed the exact Work Item detail requests returning 200 in 23–39 ms.
- Reproduced the root cause at the React Query boundary: the lifecycle card had warmed the shared cache inside the global 30-second stale window, leaving the dialog observer with data but `isFetchedAfterMount=false` and no new request.
- Fixed the dialog to force its post-mount access refetch and derive mutually exclusive loading/ready/error presentation through `discussionWorkDetailLoadState`; cached content remains hidden until the access/project check succeeds and retry returns to loading.
- Preserved the shared `Dialog`, `WorkflowFormPanel`, `EvidencePanel`, `DesignReleasePanel`, `WorkItemDetailWorkspace`, Work Item query key and all backend authorization/workflow transitions.
- Added server-owned, actor-aware workflow-form capabilities for adding confirmation, generating PDF and deleting PDF alongside the existing amendment capability.
- Kept revision-1 confirmation/PDF mutations behind normal actor/action/permission/current-node checks, with one generic exception for submitted forms whose matching engine-v2+ Work Item and runtime node are completed and whose node snapshot has `auto_advance_when_ready=true`.
- Kept ordinary completed/history/cancelled/skipped revision-1 forms locked, kept PDF deletion current-node-only and preserved audited revision 2+ amendment artifact behavior.
- Split `WorkflowFormPanel` content read-only state from artifact capability state through `workflowFormArtifactAccess`; missing capabilities fail closed and next-action copy never advertises a denied mutation.
- Kept submitted business fields and Evidence read-only in Discussion while enabling authorized confirmation/PDF controls in the same canonical `WorkflowFormPanel`; `Xem toàn bộ & lịch sử` now opens the existing `WorkItemDetailWorkspace` only as an optional path.
- Reused the existing `SignatureCaptureDialog`, confirmation grouping/preferences, protected PDF viewer/export path, `EvidencePanel`, shared `Dialog` and `WorkItemDetailWorkspace`; no parallel modal or artifact implementation was introduced.
- Confirmed the affected `Xác nhận điểm bắt đầu` record qualifies for the generic rule: engine 4, Work Item/node completed, form submitted revision 1, matching run/node IDs and `auto_advance_when_ready=true`.
- Updated central, backend, frontend, root-cause, Project Planning and Workflow Orchestration mappings. No schema or migration change was required because the existing runtime node snapshot and form-detail response envelope carry the needed state.
- Rebuilt and restarted the local Docker stack through the required `down -> build -> run` pipeline. Backend startup applied 0 and skipped 142 existing migrations; admin, backend, Postgres, Redis and Redis cache are running.
- Extracted `WorkItemRequirementPanel` as the canonical responsive presentation shared by full Work Item detail and supported direct Discussion Work editors.
- Rendered `Điều kiện bắt buộc` directly above the focused editor, auto-expanded it when blocker/warning rows remain, and retained a collapsed clean state when no blockers remain.
- Wired workflow-form/checklist and photo/document condition actions into the already-mounted `WorkflowFormPanel` and `EvidencePanel`; action-panel/unsupported targets fall back to full detail only after the existing dirty/busy protection.
- Renamed the optional navigation to `Xem toàn bộ & lịch sử`, preserved permissions as informational rows, and kept backend completion requirements/RBAC/workflow transitions unchanged.
- Updated central, frontend, root-cause, Project Planning and Workflow Orchestration mappings for the shared requirement panel. No backend mapping change was required for this follow-up because its API contract and behavior did not change.

Validation:

- PASS: `node --experimental-strip-types --test tests/discussionWork.test.ts tests/projectActivityTabs.test.ts tests/projectStartupWizard.test.ts` — 15/15 tests, including warm-cache refetch and mutually exclusive dialog states.
- PASS: warm-cache `QueryObserver` reproduction after the fix — one forced fetch transitions `success/fetching/isFetchedAfterMount=false` to `success/idle/isFetchedAfterMount=true` with fresh data.
- PASS: focused frontend artifact/discussion checks — 13/13 tests, including fail-closed capability mapping and optional detail-copy integration.
- PASS: focused backend integration tests for both auto-advanced access and ordinary-completed denial.
- PASS: `cd backend && GOCACHE="$PWD/.gocache" go test ./...` on the final working-tree source state.
- PASS: `cd frontend && npm run build` — TypeScript and Vite production build on the final working-tree source state.
- PASS: focused Discussion Work/requirement/layout frontend checks after the shared-panel change — 37/37 tests.
- PASS: `cd frontend && npm run build` after the shared-panel change — TypeScript and Vite production build.
- FAIL outside changed scope: `cd frontend && node --experimental-strip-types --test --test-reporter=dot tests/*.test.ts` — exactly one existing `tests/projectEvidenceColumns.test.ts` assertion expects separate `Hồ sơ`, `Giai đoạn`, `Đầu việc` headers while current untouched `EvidenceGallery.tsx` renders `Nguồn / Giai đoạn`; the approved discussion files are not involved.
- PASS: `docker compose config --quiet`, `make restart`, `docker compose ps`, frontend HTTP 200 and backend `/api/health` success. Backend startup log reports local storage ready and migrations `applied=0 skipped=142`.
- SKIPPED: browser/UAT was not authorized for this turn; UX status remains `NOT UX ACCEPTED`.
