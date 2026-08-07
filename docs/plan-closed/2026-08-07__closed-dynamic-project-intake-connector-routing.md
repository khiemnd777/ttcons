# [Closed] Dynamic Project Intake Connector Routing

Closed on 2026-08-07 by explicit user request.

## Final status

- No mandatory implementation or verification work remains. The final Rev5 Browser/UAT fixtures are retained as traceable evidence.

## Done

- Confirmed the current hardcoded path across Project create UI, project service validation, pipeline startup, Connector Inspector and runtime `project_entry_mode` evaluation.
- Confirmed the existing generic `workflow_form_answer_equals` evaluator, source-form field picker and publish-time source-field validation can own the new branch contract.
- Produced the architecture review, impact matrix and Mockup as Text; received explicit approval for implementation points 1-7.
- Removed route selection and BOQ upload from project create/edit; backend now ignores client-supplied `workflow_entry_mode` for the current path.
- Added schema-driven Connector configuration and validation, a registry-based runtime dispatcher, and isolated `project_entry_mode` in a legacy-only compatibility registry for immutable Rev3/Rev4 snapshots.
- Added generic workflow-form visibility and `required_when` rules on backend/frontend, with focused arbitrary-field/value regression tests.
- Added generic node `auto_advance_when_ready`: submitted forms reuse normal work-item completion, evidence, permission, node completion and Connector gates, and return an explicit advanced/blocked result.
- Applied V92 and published immutable Rev5 `40000000-0000-0000-0000-000000000024` (engine 4) with 15 unique nodes, 51 unique steps, a required intake form, conditional BOQ evidence and two `workflow_form_answer_equals` root connectors.
- Updated central/backend/frontend/root-cause and workflow/project domain maps. Backend full tests, frontend 59 tests, frontend production build, migration execution and a rollback-isolated PostgreSQL branch-evaluator test passed on the current source state.
- Restarted the Docker stack from `HEAD 36fa151` plus the current Rev5 working tree; frontend/backend/PostgreSQL/Redis started successfully, V92 was already applied (`applied=0 skipped=92`) and localhost returned HTTP 200.
- Browser/UAT created fresh Rev5 projects `UAT-R5-SURVEY-0807A` (`ea4409ce-c761-4c8d-96c7-6f667e9c78e6`) and `UAT-R5-BOQ-0807A` (`62243998-b665-45c1-863e-b5a9215b2d8a`). Project create defaulted to Rev5 and exposed no legacy entry-route/BOQ controls.
- Browser/UAT passed required-field validation, conditional BOQ evidence visibility/validation, PDF evidence upload/selection, automatic transition to Node 2 for `survey_required`, automatic transition to Node 5 for `client_boq`, and both Rev5 Connector inspectors. The builder validation action reported `Pipeline hợp lệ`; browser console warnings/errors were zero and runtime logs showed successful 2xx/201 requests without application errors.
- Browser/UAT found two unresolved defects, so this plan is not UX accepted: the submitted `entry_route` select resets visually to `Chọn` and hides the submitted BOQ evidence reference even though PostgreSQL contains both answers; `normalizeValue` only JSON-decodes object/array byte values, leaving scalar JSONB strings quoted. The BOQ transition skips only Node 2 (6 steps), while the branch-exclusive Nodes 3, 4 and 6 also remain `pending`; `skipUnselectedDirectBranchesTx` updates only direct alternative targets instead of the full branch-exclusive subgraph.
- Produced the focused two-defect review package and Mockup as Text; received explicit approval for points 1-5 including targeted JSON decoding, graph-derived branch-exclusive skipping, no hardcoded Rev5 identifiers, no runtime backfill and fresh Browser UAT.
- Added failing regressions that reproduced both defects on a disposable PostgreSQL database: only 1/4 exclusive nodes was skipped, and submitted scalar answer `Worker` was returned as `"\"Worker\""`.
- Added a column-specific workflow-answer JSON decoder covering native string, number, boolean, null, object, array and evidence-reference values without changing generic repository normalization.
- Replaced direct-alternative skipping with a generic selected-reachability versus alternative-reachability set difference. The focused graph test now proves the Survey route does not skip a later-reachable BOQ node, the BOQ route skips all four exclusive nodes, convergence nodes remain pending, cascade statuses are correct and reruns are idempotent.
- Updated central/backend/frontend/root-cause plus workflow/project domain maps for native workflow-answer values and branch-exclusive runtime semantics.
- Technically verified the current source with the focused PostgreSQL-backed regressions, full `go test ./...` on disposable database `consflow_bugfix_20260807`, frontend 59/59 tests, production build, bundle budget (121.4 KiB gzip eager entry; 108 chunks under the 250 KiB ceiling) and `git diff --check`. The disposable database was removed after verification.
- Restarted the full Docker stack with the required `down -> build -> run` sequence. PostgreSQL/Redis became healthy, backend/admin started, all 92 migrations were already applied and `/api/health` returned success.
- Browser/UAT on the rebuilt source created fresh Rev5/engine-4 projects `UAT-R5-SURVEY-FIX-0807A` (`cc398d3e-cf80-4548-be23-ae3472641baa`) and `UAT-R5-BOQ-FIX-0807A` (`6f3238a2-8010-4015-ac42-2c33a29525f3`). Project creation defaulted to Rev5 and exposed no legacy route controls.
- Survey acceptance passed after submit and explicit reload: `Khảo sát trước báo giá` remained selected, the conditional BOQ evidence field stayed hidden, Node 2 became `in_progress`, all other nodes remained reachable/pending and progress was `1/51 · 2%` with zero skipped steps.
- BOQ acceptance passed after conditional required validation, PDF evidence upload/selection, submit and explicit reload: both `entry_route=client_boq` and the evidence reference remained selected; Nodes 2, 3, 4 and 6 were skipped, Node 5 became `in_progress`, Node 7 and later convergence stayed pending, and progress was `1/35 · 3%` with 16/51 steps skipped.
- PostgreSQL confirmed the BOQ cascade: 16 steps `skipped`, work items under Nodes 2/3/4/6 `cancelled`, their draft forms `cancelled`, intake form `submitted`, Node 5 work ready and Node 7 onward pending. Stored answer values are native JSON strings (`survey_required`, `client_boq` and the evidence UUID). Browser console warnings/errors were zero; backend UAT requests completed with 2xx/201 and no 5xx/application error.
