# [Closed] Node 12 Construction Runtime Completion

Closed at: 2026-08-05

Acceptance scope: rebuilt local Rev4/engine-3 Admin UAT environment. Legacy project IDs, template revisions and engine versions were excluded. This closure is not an end-user usability study or production acceptance.

## Done

- User approved review points 1-4 for two new Rev4/engine-3 UAT fixtures, a synthetic construction-stage snapshot with an active client contract, additive local data and strict exclusion of legacy IDs/templates/engines.
- User approved the follow-up implementation package: non-recursive local evidence-volume ownership repair, Rev4 linked-action reproduction, relevant-only check filters, explicit technical-impact wording and localized linked-variation status.
- Added `golden_client_workflow_rev4_uat.sql`: project 024 is a true Rev4 entry-state snapshot; project 025 is explicitly synthetic at `construction_execution`, exposes only `construction.item_reporting` as ready and has one active main-client receivable contract. Both snapshot template `...023`, engine 3 and its definition hash without changing legacy records.
- Connected the Rev4 fixture to both Makefile seed paths and extended Golden validation for exact runtime/node/step/work-item/form counts, current-node states, single eligible construction item, active contract and initially empty Operations ledgers.
- Applied all 88 custom migrations and the complete seed sequence on isolated database `consflow_rev4_uat_check`; validator returned `GOLDEN_SAMPLE_VALIDATION_PASSED` with both Rev4 projects at 15 nodes, 51 steps/work items and 49 draft forms.
- Audited the existing Operations daily-report/check/action workspace, direct-report validation, current-node behavior, variation stages and default Supervisor RBAC against source and the supplied Admin screenshot.
- Confirmed the technical issue lifecycle supports report -> check/action -> evidence-backed resolution -> verify/reopen, and the variation path preserves Proposal -> QS -> Purchasing -> PM/Director.
- Produced and received explicit approval for the cross-domain impact review, six-phase implementation plan, acceptance criteria and Mockup as Text.
- Kept official Rev4 Node 12 as one stable `construction.item_reporting` definition while making `/projects/:id/operations` the repeatable runtime workspace for item-linked daily reports, checks, corrective actions and outside-plan proposals.
- Enforced server-side construction timing for new reports/checks/actions. A new daily report requires an eligible active-construction work item, its checklist payload and at least one accepted photo; historical rows remain readable and expose `runtime_compliant` instead of being rewritten.
- Added runtime/node/step business context and `report_eligible` to the construction workspace. The UI is read-only before Node Thi công, filters report/check/action forms to eligible construction items and uploads queued evidence before saving a report.
- Added V88 with traceable construction-action-to-contract-variation linkage and narrow `contract.variation.propose` RBAC for Admin, PM and Supervisor. A proposal from Thi công requires an active client contract, enters `qs_review` atomically with events/audit and does not grant Supervisor contract view/create or contract authoring.
- Added the Operations proposal dialog and action-detail affordance while keeping technical remediation independent from the commercial Proposal -> QS -> Purchasing -> PM/Director path.
- Clarified the workflow-definition PDF with `Revision định nghĩa`, `Trạng thái định nghĩa`, explicit mandatory checklist/document/photo columns and the continuous-repeat semantics of the Thi công step; no runtime/UAT state was added to the template export and the immutable Rev4 definition/hash was not changed.
- Updated seed/Golden validation plus central, backend, frontend, root-cause, workflow, construction, commercial and governance maps for the new route, permission, schema linkage and UI ownership.
- Applied V88 to local PostgreSQL and verified projects 020/021/022 retained template `...021`, their existing runs `...020`/`...021`/`...022`, run-template `...020` and engine version 2.
- Applied all 88 migrations from scratch, replayed seed/Golden validation and passed the full DB-backed `go test ./...` suite on isolated database `consflow_node12_test_20260805_1225`; that disposable database was then removed.
- Passed focused construction runtime/variation regressions, all frontend tests, production frontend build, bundle budget, migration manifest check, PDF Golden fixture check and `git diff --check`.
- Rebuilt the local stack with the intended `make restart` down/build/run sequence. Backend started with `applied=0 skipped=88`, all four services were up and the retained evidence directory belonged to runtime uid/gid 10001.
- Repaired only the retained local Docker path `/app/uploads/evidence` to uid/gid `10001:10001`, non-recursively, resolving the local evidence upload blocker without mutating legacy projects.
- Implemented the approved UX changes: check filters include only `report_eligible` items plus sources referenced by existing checks, impact wording separates technical classification from the explicit commercial CTA, and linked variation status uses the shared Vietnamese mapper. Added focused presentation regression coverage and updated central/frontend/root-cause/construction mappings.
- Final Browser UAT on Rev4 passed as an Admin heuristic review: project 024 remains locked at `Tiếp nhận dự án`; project 025 exposes exactly one eligible construction source, supports evidence-backed reports/checks/actions and outside-plan proposals, and renders linked variation `PS-2608051045 · QS đang rà soát`.
- Desktop and 390x844 mobile had no document overflow; the mobile dialog stayed within 16 px margins with no internal horizontal overflow; console and backend UAT logs had no warning/error or non-200 response.
- Final read-only database reconciliation proved project 024 remains empty across reports/checks/actions/evidence/variations. Project 025 contains exactly one report, one check, one action, one evidence record and two urgent `qs_review` variations; only `PS-2608051045` is linked to the action.
- Final status: `TECHNICALLY VERIFIED` and `UX ACCEPTED` for the rebuilt local Rev4/engine-3 Admin UAT environment.
