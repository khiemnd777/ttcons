# Workflow Orchestration Domain Map

## Owners

- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `types.go`
- Frontend routes: `frontend/src/app/router.tsx`
- Frontend modules: `frontend/src/modules/pipeline-builder`, `frontend/src/modules/planning`, `frontend/src/modules/workflow-forms`, `frontend/src/modules/work-items`, `frontend/src/modules/my-work`, `frontend/src/modules/projects`
- Shared frontend contracts: `frontend/src/lib/api/client.ts`, `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/status.ts`
- Migrations: `V1`-`V16`, `V40`, `V43`-`V46`, `V50`, `V52`, `V53`, `V55`-`V57`

## Routes

- Template builder: `/pipeline-templates`, `/pipeline-nodes`, `/pipeline-steps`, `/pipeline-edges`, `/pipeline-edge-conditions`
- Runtime: `/projects/:id/start-pipeline`, `/projects/:id/pipeline-run`, `/pipeline-run-nodes/:id/*`, `/pipeline-run-steps/:id/submit`
- Work items: `/work-items/:id`, `/work-items/:id/start|complete|block|resolve-blocker|request-review|request-approval|approve|reject|request-change`
- Activity/evidence: `/work-items/:id/activity`, `/projects/:id/activity|activity/options|activity/read-all`, `/projects/:id/evidence`, `/activities/:id/read|resolve|reactions|message|withdraw`, `/work-items/:id/evidence`, `/evidence/:id/file|accept|reject`
- Workflow forms: `/workflow-form-templates`, `/workflow-form-template-library`, `/workflow-form-versions/:id/publish|archive`, `/workflow-form-instances/:id/draft|submit|sign|pdf-exports`
- Artifacts: `/workflow-form-signatures/:id/visual`, `/workflow-form-pdf-exports/:id/file`

## Tables

- `pipeline_templates`, `pipeline_template_nodes`, `pipeline_template_edges`, `pipeline_template_steps`
- `pipeline_runs`, `pipeline_run_nodes`, `pipeline_run_edges`, `pipeline_run_steps`
- `work_items`, `work_item_assignments`, `work_item_activities`, `activity_mentions`, `activity_read_states`, `activity_reactions`, `activity_message_revisions`
- `approval_rules`, `approval_requests`, `approval_actions`
- `workflow_form_templates`, `workflow_form_versions`, `workflow_form_fields`, `workflow_form_instances`, `workflow_form_answers`, `workflow_form_events`, `workflow_form_signatures`, `workflow_form_pdf_exports`
- `evidence_records`, `documents`, `audit_logs`

## Invariants

- Published pipeline template revisions are immutable; structural edits require a new draft revision in the same `template_family_id` lineage.
- Publish records `definition_hash`, `published_at`, and `published_by`; project runtime starts only from an exact published revision.
- `pipeline_runs.engine_version` and `pipeline_runs.template_definition_hash` snapshot the selected execution contract and are not refreshed from later revisions.
- Engine v2 run steps snapshot `form_required` and exact `form_version_id`; a submitted different form or synthetic legacy checklist cannot satisfy the gate.
- Runtime steps are deterministically ordered by `order_index,id`; do not reference a non-existent `pipeline_run_steps.created_at` column in run detail or completion-gate queries.
- UAT for official template automation must invoke stable engine-v2 `domain_action_key` actions through the hard-automation gate; display-name automation is legacy engine-v1 compatibility only.
- Engine v2 work items can mutate only inside the run's current node. Advancing a node requires all run steps/work items, exact forms, node approval, and mandatory automation events to be ready.
- Branch transitions mark non-selected direct alternatives `skipped`, cancel their pending work/form instances, and exclude skipped nodes from progress.
- Project Operations preserves those cancelled branch work items as read-only history and labels them as skipped; action queues and `CanActOnWorkItem` continue to exclude them. Runtime timeline completion treats `submitted` and `completed` steps as satisfied, while skipped/cancelled steps are counted separately and never inflate progress.
- The project Tổng quan presentation reuses runtime work-item priority and timeline data: next action order stays blocker -> waiting approval/review -> other open work, current-stage progress remains source-backed, and navigation labels may be Vietnamese without changing runtime status keys or gates.
- The project `Kế hoạch` route is a navigation-only action queue over the complete ordered pipeline-run step/work-item set. Runtime work-item status—not workflow-form draft state—controls actionable/waiting/future/completed grouping; form requirement/status and evidence remain supporting metadata. Its next-work priority is blocker -> waiting approval/review -> other open work, and it never performs step completion, approval, blocker resolution or node transition outside permission-gated detail/`Quy trình` surfaces.
- The project Workflow tab and `/projects/:id/pipeline` reuse one execution workspace that leads with current node, executable progress and one next-work CTA ordered blocker -> waiting approval/review -> other open work. Active nodes default to `Cần xử lý`; skipped/cancelled nodes default to `Đã bỏ qua` so read-only branch history remains immediately visible. Run-node request-approval, advance and block remain secondary controls and never bypass backend gates; recreate and block require explicit confirmation/input dialogs.
- The project Work tab is an operational queue over the complete scoped Project Operations `work_items` list: actionable work is the default, waiting approval/review and blocked work remain explicit attention queues, future `pending` work stays distinct, and completed/cancelled records remain available as history. List rows expose only navigation; transitions, forms, evidence and approvals remain inside permission-gated work-item detail.
- Engine v2 steps snapshot `surface_kind` and `domain_action_key`; work-item UI must route by this contract before any legacy text heuristic.
- Official contract steps 24 and 29-31/33 use `surface_kind='contract'`; step 32 remains the approval surface. Pending runtime steps receive the new surface/permission snapshot, while completed historical steps are not rewritten.
- Official quality steps 36-37 use `surface_kind='quality'` with `quality.create` and `quality.recurring_manage`; only pending runtime steps receive the new surface/permission snapshot.
- Official finance steps 42, 44-45, 48-51 use `surface_kind='finance'`; closing step 52 uses `surface_kind='handover'`. Only pending runtime steps are remapped and their configured business form/PDF remains a supporting tab.
- Submitting `builtin_construction_daily_report` invokes an idempotent operations-ledger bridge in the same transaction before workflow completion payload is persisted.
- The operations timeline consumes that bridge's structured daily-report answers without changing workflow completion semantics. Direct manual reports use their own typed create/update and report-evidence-upload contract, but still do not create work items, workflow form instances, approvals or completion events; new files remain normal work-item evidence. Any report with `workflow_form_instance_id` is read-only in Operations and rejects direct update/upload, preserving the source form as authoritative. Legacy summary rows remain readable. Outgoing work-item acceptance and the project acceptance workspace share `AcceptanceEditor.tsx`, while form/signature/PDF gates and work-item action permissions remain authoritative.
- Engine v2 hard `domain_action_key` automation runs before work-item completion. Blocked/failed outcomes are recorded idempotently and leave the work item incomplete; display-name dispatch remains legacy-only.
- Engine v2 node approvals snapshot structured `approval_policy_json` into the run and request. Supported modes are `any`, `all`, `quorum`, and `ordered`; stage, actor role, requester separation, comments, actions, and audit remain traceable.
- Publish validation covers graph reachability/root/sink/cycle, required published form versions, answer-condition configuration, and permission-key existence.
- Template edits do not mutate existing run copies.
- Cosmetic official-step text corrections require an additive migration and matching seed correction across template, runtime, work-item and form-instance display snapshots; they do not change statuses, answers, approvals, audit records or PDF artifacts.
- Published workflow form versions are immutable runtime references.
- `required_role_ids` remains an array everywhere.
- Work item completion respects required forms, evidence, signatures, approvals, and blockers.
- Project evidence listing is read/review presentation only: server-filtered history may include rejected evidence but excludes cancelled-work-item evidence; `can_review` mirrors the existing decision gate and never replaces backend enforcement or satisfies completion/form/approval gates.
- Project activity resolution is separate from work-item transitions: question/blocker/reminder metadata may be closed only by an authorized actor, and each resolution appends a system activity plus audit without bypassing work-item blocker or completion gates.
- Project activity replies and reactions are communication-only: replies inherit the parent context/visibility and reactions are active-state acknowledgements; neither can satisfy work-item evidence, form, approval, blocker, completion or runtime-node gates.
- Project activity comment edits and withdrawals are communication-only and author-scoped within 15 minutes. They append revisions/audit and never satisfy, reopen or bypass work-item evidence, form, approval, blocker, completion or runtime-node gates; typed operational activity is not mutable through these endpoints.
- Signatures and PDFs preserve revision, actor, hash, checksum, and protected file access.

## Cross-Domain Checks

- Pair with the source business domain when workflow forms create or summarize planning, construction, commercial, material, acceptance, or finance records.
- Check template authoring, runtime copies, work-item gates, activity, and frontend status labels together when changing workflow transitions.
- Do not let convenience UI bypass approval, required-form, evidence, signature, PDF, material, receivable, or blocker gates.

## Golden Sample

- `GOLDEN-REAL-001` and `GOLDEN-NO-DESIGN-001` jointly cover all 12 official nodes, 52 steps and 51 form-bound steps. The flagship skips the no-design BOQ node; the companion skips the design nodes, preserving real branch semantics.
- `backend/seed/validate_golden_project.sql` asserts exact form-version submission, required answers, accepted evidence and approval/PDF coverage.
- `GOLDEN-LIVE-001` snapshots the same official engine-v2 contract at node `Khảo sát`: node 1 is `in_progress`, all 52 run steps are pending, six first-node work items are ready, 51 exact form instances are draft, and no completion evidence or domain ledger is pre-seeded.
- Official seeded approval policies must use V46 keys (`roles`, `quorum`, `requester_cannot_approve`, `comment_required`, `signature_required`) and mode `any`; legacy aliases such as `required_roles` are invalid for runtime approval normalization.
