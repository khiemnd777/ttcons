# Project Planning Domain Map

## Owners

- Frontend modules: `frontend/src/modules/projects`, `frontend/src/modules/planning` including `ConstructionHandoffPanel.tsx`, `frontend/src/modules/planning-daily-reports`, `frontend/src/modules/meeting-minutes`, `frontend/src/modules/planning-checklists`
- Shared work item UI: `frontend/src/modules/work-items`
- Backend routes: `backend/internal/core/routes.go`
- Backend logic: `backend/internal/core/handlers.go`, `service.go`, `repository_domain.go`, `project_progress.go`, `types.go`
- Contracts: `frontend/src/lib/types/domain.ts`, `frontend/src/lib/utils/status.ts`

## Routes

- Projects: legacy `/projects`, paginated `/projects/explorer`, invalidation `/projects/explorer/stream`; rolling-compatible full `/projects/progress-board`, bounded `/projects/progress-board/summary`, `/projects/progress-board/stages/:stageKey`, invalidation `/projects/progress-board/stream`; dynamic `/projects/:id`, `/projects/:id/operations`, `/projects/:id/evidence`
- Project members: `/projects/:id/members`, `/projects/:id/members/:memberId`
- Planning: `/projects/:id/start-pipeline`, `/projects/:id/pipeline-run`, `/projects/:id/work-items`, `/projects/:id/construction-handoff`, `/projects/:id/construction-handoff/activate`
- Planning reports: `/planning/daily-reports`, `/planning/meeting-minutes`, `/planning/checklists`
- Activity: `/projects/:id/activity`, `/projects/:id/activity/options`, `/projects/:id/activity/read-all`, `/projects/:id/activity/stream`, `/activities/:id/read|resolve|reactions|message|withdraw`, `/me/activity-attention`
- Contract workspace: `/projects/:id/contracts`; activating the signed main-client contract updates the project-level `contract_amount` compatibility snapshot.

## Tables And Migrations

- `projects`, `project_members`, `audit_logs`
- Commercial project master: `contracts`, signed `contract_versions`; `projects.contract_amount` is not the version history.
- Runtime planning: `pipeline_runs`, `pipeline_run_nodes`, `pipeline_run_steps`, `work_items`, `construction_handoffs`, `construction_handoff_items`
- Form-backed planning: `workflow_form_instances`, `workflow_form_answers`, `workflow_form_events`
- Activity: `work_item_activities`, `activity_mentions`, `activity_read_states`, `activity_reactions`, `activity_message_revisions`
- Relevant migrations: `V1`, `V5`, `V6`, `V7`, `V8`, `V9`, `V10`, `V11`, `V12`, `V13`, `V58`, `V60`, `V66`, `V89`, project-explorer cache revisions/outbox/indexes/retention in `V97`-`V100`

## Invariants

- Planning reports are project-scoped and permission-filtered.
- The operational project report counts open issues and pending approvals in independent correlated subqueries per accessible non-cancelled project; it must not join both one-to-many sources before counting. Optional `project_id` and CSV export retain the same scope.
- Meeting minutes action items and daily report next-plan outputs remain traceable.
- Planning checklist evidence/status summaries do not replace the underlying submitted form history.
- Project detail and global workspaces must agree on statuses and available actions.
- Global `/progress` is a standalone project-stage board, not a tab inside `/projects` or project detail. It is read-only and gated by `project.view`; cards navigate to project detail and never drag/drop or transition runtime state. Summary and cursor-stage queries exclude cancelled projects through `ProjectAccessFilter`, join only the project's snapshotted `current_pipeline_run_id` and that run's `current_node_id`, and share the same SQL mapping of technical node types into stable presentation stages. Projects without a run remain `Chưa khởi động`; completed runs remain `Hoàn tất`; unknown/custom types remain visible in `Khác`. Search, PM UUID and project status are server-authoritative; a stage page defaults to 25/max 50 and binds its descending `created_at,id` cursor to stage plus normalized filter hash. Blocked/overdue/issues/pending approvals remain independent badges calculated only for returned page IDs. The legacy full endpoint stays temporarily for rolling compatibility, while the page consumes only summary/stage. Frontend keys are actor-scoped, desktop loads visible/near columns, mobile loads one selected stage, card lists virtualize and deep pagination is refreshed only by an explicit cursor reset. The 10k/80%-skew gate passed, so no progress-specific index, migration or Redis response cache exists.
- Project Thành viên uses a dedicated typed API rather than the Operations payload. Active App members with `project.view` see active membership; `project.member.manage` controls inactive history and mutations. Save/reactivate/deactivate synchronizes `project_members` with project-scoped `user_roles` transactionally, keeps global roles untouched, soft-deactivates history, audits changes and protects the current PM from removal. External-only users, role IDs and matching operational labels are excluded from options and rejected on save; V89 soft-deactivates existing linked client memberships without deleting history.
- Project `Kế hoạch` is an action-first queue over every ordered runtime step. Its default `Cần xử lý` view is derived from work-item runtime status, `waiting_approval`/`waiting_review` stay distinct, only `pending` is future work, and terminal/skipped work remains discoverable outside the actionable queue. The one next-work CTA stays blocker -> waiting approval/review -> other open work. Workflow-form draft/submitted/required state and evidence count are supporting metadata; rows only navigate to permission-gated work-item detail, while node transition/approval/block controls remain in `Quy trình`.
- Official workflow revision 2 separates `Kế hoạch thi công` from field `Thi công`. The dedicated `client_construction_handoff_plan` form owns the inspection-item list; Planning exposes one bulk review/activation CTA. The planning node cannot complete until a handoff snapshot is activated. Each activation preserves source form/node, actor/time, form revision, handoff revision and hash; the request carries the previewed form revision and stale-tab activation is rejected after a locked recheck. Replaying the same submitted form revision returns the existing handoff. A newer form revision may supersede only generated rows that are still unstarted and have no evidence/action; otherwise activation is rejected so field history is never overwritten. Current revision-1 construction runs may import submitted QA/assignment data without mutating their runtime history.
- Project Tổng quan leads with one prioritized action ordered blocker -> waiting approval/review -> other open work, shows the current runtime stage beside it, keeps source-backed attention counts separate from budget/contract/schedule reference data, and uses Vietnamese system labels without translating user-entered project/work-item data.
- Project Hồ sơ is a project-scoped work-item evidence workspace: summary/search/filter/cursor results use the same visibility and work-item scope, waiting review sorts first, rejected evidence remains discoverable as history, cancelled-work-item evidence stays excluded, and review controls appear only when backend `CanDecideWorkItem` yields `can_review=true`.
- Project activity defaults to all visible activity and uses total visible count for the project-tab badge; attention counts remain inside `Cần chú ý`, and the `Quyết định` tab uses the dedicated source-backed `decisions` summary count. It uses server-backed search/filter/cursor pagination and permission-scoped picker options. Resolving a question, blocker, or reminder preserves history by appending a system event and audit record.
- Project, global-progress and personal-attention SSE are invalidation-only. Project streams emit only event type, project ID and timestamp after the initial authoritative project check. The global progress stream snapshots `ProjectAccessFilter`-visible project IDs once, maps direct work-item changes plus explicit project/runtime/issue/approval mutations to minimal `project_progress_changed`, and permits a project-agnostic create invalidation without exposing a new project ID; disconnected clients poll the authoritative snapshot every 60 seconds. Personal attention builds the actor's operations-visible project-ID snapshot once when the connection opens and filters in memory, avoiding per-event/per-subscriber DB amplification. Connections consume bounded process/IP/actor capacity, release it idempotently and close at a randomized 80-100% of the configured maximum lifetime (default about four to five minutes); reconnect rebuilds JWT/session/project visibility so revocation lag is bounded without a synchronized reconnect herd. Multi-replica fanout uses Redis Pub/Sub with source-instance deduplication, and Redis outage degrades to replica-local delivery.
- Project `Nhật ký` is a five-event preview backed by the paginated project Audit endpoint; the tab does not treat the preview length as the total and routes full investigation to `/projects/:id/audit`.
- Project `Thảo luận` presents activity as chronological chat with a persistent composer and scroll anchoring. A reply is a new `comment` row linked by `parent_activity_id`; backend copies the parent project/target/work item/visibility and auto-mentions its actor. Reactions use `activity_reactions` active-state upserts, are visibility-gated, and never change read/resolution/workflow status or activity totals. Only the author may edit or withdraw a `comment` during the first 15 minutes; edits/withdrawals append `activity_message_revisions` and audit, withdrawal clears current content/attention/reactions but keeps a tombstone and existing replies. Typed operational activity is immutable.
- Project Detail Workflow is the project history surface: default to all node work items, keep completed and skipped records discoverable with filters, and render skipped work-item detail read-only without weakening action permissions.
- The expanded `/projects` explorer and the project workspace sidebar share one group/child/permission route config. A group landing resolves to its first visible child, with its own target as fallback. Explorer search, status, global sort/direction and page-size 25/50/100 are server-authoritative through `/projects/explorer`; the query key is actor-scoped and a page displays only its returned records. Expansion remains project-ID based and user-controlled: every loaded page starts collapsed, authoritative refetches retain only manually expanded IDs that are still visible, and a search/filter/page change with no retained ID does not open the new first row. The invalidation-only stream refreshes the current authoritative query and a disconnected client polls every 60 seconds. On narrow screens each top-level project and optional expanded workspace remain one bordered rounded card; desktop retains dense rows and the 58/42 folder/dashboard layout.
- Project explorer Redis is a disposable acceleration layer, never an authorization source. The index key includes catalog and actor scope revisions; each row key includes its project revision. Before serving cached rows, backend re-applies `ProjectAccessFilter` and reads project revisions only for IDs currently visible to the actor. Project, derived issue/approval data, membership, staff, role, partner and template writes bump revisions transactionally and enqueue an after-commit SSE invalidation through the durable outbox. Redis outage, corrupt/oversized values, lock contention, circuit-open or scope mismatch falls back to PostgreSQL without changing the response contract.

- Project metadata review/edit on `/projects` has one pencil entry point. Its unified dialog presents the current status, progress and partner snapshot/tax as read-only context above the editable metadata/template form; no separate eye/detail action or additional detail request exists, and the project-workspace folder action remains separate. The selector is labelled `Chọn chủ đầu tư có sẵn`, the read-only summary is `Chủ đầu tư đã chọn`, and the helper describes storing the current name in plain language.

## Cross-Domain Checks

- Pair with workflow orchestration when planning records are backed by workflow form instances, work items, signatures, or PDF exports.
- Pair with construction operations when planning outputs become field execution work or project activity.
- Pair with reporting when changing planning metrics, filters, date scopes, owner scopes, or CSV/reportable fields.

## Golden Sample

- Use `GOLDEN-REAL-001` as the completed flagship project and `GOLDEN-NO-DESIGN-001` as the mutually exclusive branch companion when reviewing project-first navigation, planning runtime and form-backed planning records.
- Use `GOLDEN-LIVE-001` to experience real planning/runtime transitions. It intentionally starts at 0% with only the Khảo sát node active; reset it with local/dev `make seed` after an operational walkthrough.
- Golden Live has fourteen active App members. `client@consflow.local` is inactive and not a project member; client confirmation stays in project documents, forms/evidence and business snapshots.

## Legacy Rev3/Rev4 Project Intake

- `projects.workflow_entry_mode` is `survey_required` by default or `client_boq`; it can change only before a pipeline run exists. Starting `client_boq` fails closed without an active project-scoped `client_boq` document, then uses the template's explicit edge condition to skip survey/meeting/design.
- Historical project creation uploaded BOQ evidence through the protected project-document API before starting the runtime. This remains compatibility context only; new project forms must not author `workflow_entry_mode`.
- Use `GOLDEN-REV3-BOQ-001` (project 023) to review the untouched Rev3 client-BOQ branch. Project 022 is the preservation control and must remain on its original runtime/template snapshot.
- For Rev4/engine-3 UAT, project 024 is the production-shaped entry-state control at `project_intake`; project 025 is a labelled synthetic construction-stage snapshot and must not be used to infer that planning handoff or earlier node gates completed.

## Rev5 Project Intake

- Project create/edit owns metadata and explicit published-template selection only. Backend ignores client-supplied `workflow_entry_mode`; the database column remains for immutable Rev3/Rev4 compatibility.
- After Rev5 starts, the active `Tiếp nhận dự án` work item owns `entry_route` and conditional BOQ evidence. Submitted workflow-form answers are the only Rev5 branch source; Connector configuration chooses `Khảo sát` or `BOQ (Không thiết kế)`.
- Selecting a Rev5 connector skips the complete branch-exclusive descendant set, not only its direct sibling. Nodes shared with or reachable from the selected path remain pending/executable; project workflow progress excludes all skipped steps and keeps their cancelled work/forms available only as read-only history.
- Projects still selecting Rev4 and having no run are moved to Rev5 by V92. Existing project runs are never rebound or rewritten.
- Project estimated budget and contract amount are monetary inputs and use semantic `currency`/shared `CurrencyInput`; project progress, dates and other non-money numeric planning values do not.
