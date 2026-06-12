# Current States

## Project Operations Production Plan (Active)

Status: active planning state as of 2026-06-12.

This section is the source of truth for future agents. It replaces the earlier `/my-work`-centered framing. The `/my-work` design remains valid, but it is a subordinate personal inbox, not the operating center of Consflow.

### Latest Objective

Turn Consflow from a broad admin-module demo into a production construction workflow operating system:

1. `Pipeline Builder` creates reusable production templates.
2. Each `Project` receives a project-specific runtime copy of the selected template.
3. The `Project Operations Dashboard` is the main work surface for each construction project.
4. PM/director see a full project/global operating picture.
5. Departments, roles, and personnel see only the project surfaces and work tools relevant to their assigned workflow scope.
6. `/my-work` is a personal cross-project work inbox derived from the same work items, not the primary product concept.
7. Activity/chat is controlled and scoped to reminders, reports, blockers, decisions, evidence, and audit history. It must not become a general social chat room.

### Non-Negotiable Product Bar

- This is a production plan, not an MVP plan.
- UI must be direct, visual, operational, and scannable.
- Do not add lots of always-visible buttons. Each context should expose one dominant next action and keep secondary actions contextual.
- Do not make users browse generic modules to discover what to do next.
- Do not let users see unrelated department/role/personnel surfaces.
- Do not rely on frontend hiding for access control. Backend data scope and permissions must enforce the same boundaries.
- Do not turn the admin UI into a marketing dashboard. Keep it dense, work-focused, and suitable for daily operations.
- Preserve workflow history. Actor, time, evidence, comments, status transitions, and decisions matter.

### Product Hierarchy

```txt
Pipeline Builder
  -> Template definition
     -> Nodes
     -> Steps / checklist items
     -> Role / department / permission ownership
     -> Evidence requirements
     -> Approval rules
     -> Entry / exit conditions
     -> Fail path / rework / replanning behavior

Project Runtime
  -> Project-specific pipeline_run copy
     -> Runtime nodes
     -> Runtime steps
     -> Work items
     -> Assignments
     -> Evidence
     -> Activities
     -> Audit

Project Operations Dashboard
  -> Main project control room
     -> PM / Director overview
     -> Department surfaces
     -> Role/personnel scoped work
     -> Contextual tools
     -> Controlled activity
     -> Project audit and playback

/my-work
  -> Personal inbox across projects
     -> Mine
     -> Department
     -> Pending approval
     -> Watching
```

## Current Repo Reality

### What Already Exists

- Backend has project, pipeline template, pipeline run, approval, BOQ, procurement, warehouse, operation, acceptance, rework, replanning, handover, accounting, staff, RBAC, audit, report, and settings routes.
- Schema already has reusable template tables and runtime pipeline tables.
- `StartPipeline` clones template nodes and steps into runtime records.
- Seed data has a construction template covering survey, design, BOQ, budget approval, procurement, warehouse issue, construction, acceptance, handover, and financial closure.
- Frontend has pages for project detail, planning, BOQ, procurement, warehouse, operations, acceptance, rework, handover, accounting, debt, audit, reports, staff, roles, permissions, and settings.

### What Is Not Production-Ready Yet

- `ProjectDetailPage` is still mostly stat cards plus module links.
- `PlanningPage` is still a generic pipeline node/step screen with simple submit/advance/block actions.
- Sidebar currently shows all modules to all authenticated users.
- Backend route protection is mostly login-only. Only pipeline node/step actions currently have permission checks.
- `pipeline_run_steps` completion is too thin: no typed completion payload, evidence, start/end time, assignment, checklist result, or blocker history.
- `documents` table exists but there is no first-class evidence upload flow tied to work items.
- Seed roles exist, but non-admin operational roles lack complete production permissions.
- Procurement generation currently includes submitted BOQ items; production should generate only from approved BOQ items.
- No first-class controlled activity/thread model exists.
- No first-class cross-domain work item model exists.

## Production Operating Model

### Core Runtime Concepts

Use these concepts consistently across backend, frontend, seed data, and docs:

- `Project`: construction job context.
- `Pipeline Template`: reusable workflow definition.
- `Pipeline Template Node`: major reusable workflow phase.
- `Pipeline Template Step`: reusable checklist/work requirement inside a node.
- `Pipeline Run`: project-specific copy of a template.
- `Pipeline Run Node`: project-specific workflow phase.
- `Pipeline Run Step`: project-specific workflow requirement.
- `Work Item`: actionable unit shown to a person, role, or department.
- `Assignment`: who can act on a work item.
- `Evidence`: documents/images/checklist payload required to complete work.
- `Activity`: controlled project/work conversation event.
- `Approval Request`: explicit decision gate with actor/comment/time.
- `Audit Log`: durable historical record for playback and investigation.

### Required Work Item Model

Future agents should add or derive a first-class work execution model. Prefer a new `work_items` table over overloading `pipeline_run_steps`, because production work spans pipeline steps plus BOQ, procurement, stock, acceptance, rework, handover, and accounting records.

Minimum fields:

- `id`
- `project_id`
- `pipeline_run_id`
- `run_node_id`
- `run_step_id`
- `source_type`
- `source_id`
- `title`
- `description`
- `work_type`
- `status`
- `priority`
- `department`
- `role_ids`
- `assignee_staff_id`
- `watcher_staff_ids`
- `planned_start_at`
- `due_at`
- `started_at`
- `completed_at`
- `blocked_at`
- `blocked_reason`
- `completion_payload_json`
- `evidence_requirement_json`
- `approval_request_id`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

Required statuses:

- `pending`
- `ready`
- `in_progress`
- `waiting_evidence`
- `waiting_review`
- `waiting_approval`
- `blocked`
- `needs_changes`
- `completed`
- `cancelled`

Work type mapping:

- `task`: start, complete, block.
- `checklist`: complete checklist items, add notes, submit.
- `approval`: approve, reject, request changes.
- `document_upload`: upload required file, submit review.
- `photo_upload`: upload images, captions, location/context, submit.
- `design_review`: upload drawing, track revision, internal/client review.
- `budget_check`: inspect budget, confirm within/exceeded, escalate.
- `material_check`: inspect material readiness, create/progress procurement, issue stock.
- `daily_report`: report manpower, progress, photos, issues, material usage.
- `acceptance_check`: pass/fail checklist, evidence, punch list, rework.
- `rework_task`: execute corrective work and verify closure.
- `handover_check`: verify closure requirements and client confirmation.
- `accounting_check`: reconcile budget/payables/receivables/payments.

### Action Rules

Each work item should expose one primary CTA based on user permission, assignment, work type, and current status:

- `Start`
- `Continue`
- `Upload`
- `Submit review`
- `Request approval`
- `Approve`
- `Reject`
- `Request changes`
- `Report blocker`
- `Resolve blocker`
- `Complete`
- `Create rework`
- `Record payment`
- `Confirm receipt`

Secondary actions belong in menus, detail panels, or contextual footers.

## Pipeline Builder Production Contract

The builder must define operational behavior, not only node geometry.

### Template-Level Requirements

Each template should define:

- project type
- version
- publish status
- owner
- expected departments
- expected roles
- default SLA/deadline rules
- allowed node types
- approval policy defaults
- evidence policy defaults
- rework/replanning policy defaults
- client-facing visibility policy

### Node-Level Requirements

Each node should define:

- node type
- title and operational description
- responsible departments
- responsible roles
- optional direct assignee rule
- required permission
- entry conditions
- exit conditions
- approval rule
- rework enabled
- replanning enabled
- fail path
- dependency edges
- dashboard visibility
- client visibility
- expected risk indicators

### Step-Level Requirements

Each step should define:

- step type / work type
- checklist items
- required evidence
- accepted file types
- photo required flag
- document required flag
- required role/permission
- default priority
- default due offset
- required review/approval
- completion payload schema
- activity scope
- audit event type

### Example Template Step Contract

```txt
Node: Thiết kế
Step: Upload bản vẽ R2
Work type: design_review
Owner: Department=Thiết kế, Role=Designer
Visible to: Design, PM, Director
Client visible: only after PM shares
Required evidence:
  - drawing file
  - revision note
  - affected area
Completion payload:
  - drawing_type
  - revision_no
  - change_reason
  - client_confirmation_required
Primary CTA by status:
  - ready -> Upload drawing
  - waiting_review -> PM review
  - needs_changes -> Update revision
  - waiting_approval -> Client confirm
Fail path: replanning
Audit events:
  - design.drawing_uploaded
  - design.review_requested
  - design.revision_requested
  - design.client_confirmed
```

## Project Runtime Production Contract

When a template is applied to a project:

1. Clone template nodes into `pipeline_run_nodes`.
2. Clone template steps into `pipeline_run_steps`.
3. Generate or derive `work_items` from runtime steps.
4. Resolve role/department/personnel scope from project members.
5. Attach project-specific deadlines.
6. Initialize activity scopes.
7. Initialize evidence requirements.
8. Preserve template version references.
9. Do not mutate historical project runs when a template is edited later.
10. Write audit entries for runtime creation.

Runtime generation must support:

- multiple work items from one step when different departments must act separately
- approval work items generated from approval rules
- client-visible work items when applicable
- rework/replanning work items created from failed acceptance or blockers
- domain work items linked to BOQ, procurement, stock, accounting, handover, and payments

## Project Operations Dashboard

The Project Operations Dashboard is the primary project work surface.

Route target: existing `/projects/:id` should evolve from a module directory into an operations control room.

### Dashboard Information Architecture

```txt
/projects/:id
┌────────────────────────────────────────────────────────────────────────────┐
│ Project Header                                                              │
│ Code · Name · Client · PM · Status · Schedule · Budget · Contract           │
│ Main CTA changes by current user's role and project state                   │
├────────────────────────────────────────────────────────────────────────────┤
│ Health Strip                                                                │
│ Need action │ Overdue │ Blocked │ Waiting approval │ Budget risk │ Material │
├──────────────────────────────┬─────────────────────────────┬───────────────┤
│ Workflow Timeline            │ Active Work                 │ Command Center │
│ Node status, progress, risk   │ Work items in current node   │ Blockers       │
│ Click node to scope dashboard │ One dominant CTA per item    │ Approvals      │
│                              │                             │ Reminders      │
│                              │                             │ Latest files   │
│                              │                             │ Audit          │
├──────────────────────────────┴─────────────────────────────┴───────────────┤
│ Department Surfaces                                                         │
│ Visible tabs/cards depend on current user scope                             │
│ Thiết kế | QS | Mua hàng | Kho | Thi công | QA/QC | Kế toán | Client        │
└────────────────────────────────────────────────────────────────────────────┘
```

### Shared Dashboard Requirements

- Header must show project identity and current operating state.
- Health strip must show numbers that require attention, not vanity stats.
- Workflow timeline must show current node, completed nodes, blocked nodes, waiting approval nodes, and next node.
- Active work must prioritize by blocker, overdue, waiting approval, then due date.
- Command center must show action queues, not generic chat.
- Department surfaces must appear only if the current user has project membership, role, department, assignment, or permission scope.
- Module links can exist, but they are secondary. They must not be the primary operating model.

### PM Project Dashboard

PM sees the whole project, but still needs focus.

```txt
PM / Project Manager
┌────────────────────────────────────────────────────────────────────────────┐
│ CF-Q1-001 · Fit-out Văn phòng Q1                         [Resolve blocker] │
│ Health: At risk · Current node: Thiết kế · Due: 2026-08-30                 │
├───────────────┬───────────────┬───────────────┬───────────────┬────────────┤
│ Need action 7 │ Overdue 2     │ Blocked 1     │ Approval 4    │ Risk high 2│
├───────────────────────────────┬────────────────────────────────────────────┤
│ Critical Queue                │ Current Workflow                            │
│ 1. Kho thiếu dây điện         │ Khảo sát done                               │
│ 2. BOQ vượt ngân sách         │ Thiết kế in progress                        │
│ 3. Client chưa confirm R2     │ BOQ waiting                                 │
├───────────────────────────────┴────────────────────────────────────────────┤
│ Department status: Design warning | QS waiting | Warehouse blocked          │
└────────────────────────────────────────────────────────────────────────────┘
```

PM focus:

- blockers
- late work
- cross-department handoff
- approvals needing follow-up
- project health
- material/accounting/acceptance risks
- team workload

PM primary CTAs:

- `Resolve blocker`
- `Request update`
- `Approve`
- `Open work detail`
- `Share with client`
- `Create replanning`

### Director Dashboard

Director sees fewer operational details and more decision context.

```txt
Director
┌────────────────────────────────────────────────────────────────────────────┐
│ Projects requiring decision                              [Approve selected]│
├──────────────────┬──────────────────┬──────────────────┬─────────────────┤
│ Budget exceeded  │ Critical blockers│ Payment approvals│ At-risk projects│
│ 3                │ 2                │ 5                │ 4               │
├──────────────────┴──────────────────┴──────────────────┴─────────────────┤
│ Decision queue                                                             │
│ - CF-Q1-001 · BOQ vượt +42,000,000 · PM recommends approve                 │
│ - CT-TD-002 · Replanning +5 days · Client change request                   │
└────────────────────────────────────────────────────────────────────────────┘
```

Director focus:

- high-level approvals
- budget overruns
- major blockers
- financial exposure
- project risk
- audit trail and decision context

Director primary CTAs:

- `Approve`
- `Reject`
- `Request changes`
- `View decision context`

## Role / Department Project Surfaces

These surfaces live inside the Project Operations Dashboard and can also be reused from `/my-work` detail panels.

### Design / Thiết Kế

```txt
Design Surface
┌────────────────────────────────────────────────────────────────────────────┐
│ Thiết kế · CF-Q1-001                                      [Upload drawing] │
├──────────────────────┬─────────────────────────────────────────────────────┤
│ Design Queue          │ Selected Design Work                                │
│ ● Layout R2           │ Step: Upload bản vẽ                                  │
│ ▲ Pantry revision     │ Drawing type: Layout                                │
│ ■ Client confirm      │ Revision: R2 · Status: Need PM review                │
│                       │ Required: drawing file, revision note, affected area │
│                       │ Files: layout-R1.pdf, layout-R2.dwg                  │
│                       │ [Report blocker]                    [Send review]    │
├──────────────────────┴─────────────────────────────────────────────────────┤
│ Activity: PM requested revise pantry wall · Designer uploaded R2            │
└────────────────────────────────────────────────────────────────────────────┘
```

Design must support:

- concept/design package tracking
- shop drawing tracking
- revision number
- affected area
- client confirmation requirement
- internal PM review
- client review/confirmation
- linked BOQ impact
- design-change-to-replanning path

Design should not expose warehouse/accounting tools except read-only contextual impact where relevant.

### QS

```txt
QS Surface
┌────────────────────────────────────────────────────────────────────────────┐
│ BOQ / Dự toán                                             [Update BOQ]     │
├──────────────────────┬───────────────────────┬─────────────────────────────┤
│ Linked drawings       │ BOQ status             │ Budget impact               │
│ Layout R2 approved    │ 18 approved / 3 open   │ +42,000,000 over baseline   │
├──────────────────────┴───────────────────────┴─────────────────────────────┤
│ Work queue                                                                  │
│ - Bóc tách trần thạch cao                              [Continue]          │
│ - Kiểm tra phát sinh pantry                            [Ask Design]        │
│ - Submit BOQ for PM/accounting approval                 [Submit BOQ]        │
└────────────────────────────────────────────────────────────────────────────┘
```

QS must support:

- BOQ by design revision
- quantity extraction
- unit price input
- cost variance
- budget impact note
- design clarification requests
- approved-only procurement source
- replanning cost impact

### Procurement / Mua Hàng

```txt
Procurement Surface
┌────────────────────────────────────────────────────────────────────────────┐
│ Mua hàng                                                   [Create PR]     │
├──────────────────────┬───────────────────────┬─────────────────────────────┤
│ Need to buy           │ Waiting approval       │ Delivery risk               │
│ 12 items              │ 3 PRs                  │ 2 late                      │
├──────────────────────┴───────────────────────┴─────────────────────────────┤
│ Material demand from approved BOQ                                           │
│ - Dây điện Cadivi · Need 1,200m · Ordered 600m          [Create PR]         │
│ - Tấm thạch cao · Need 220 sheets · Waiting supplier    [Update delivery]   │
└────────────────────────────────────────────────────────────────────────────┘
```

Procurement must support:

- demand from approved BOQ only
- vendor/supplier tracking
- PR approval status
- order status
- delivery status
- budget status
- blocker reporting
- handoff to warehouse receive

### Warehouse / Thủ Kho

```txt
Warehouse Surface
┌────────────────────────────────────────────────────────────────────────────┐
│ Kho vật tư                                                  [Issue stock]  │
├──────────────────────┬───────────────────────┬─────────────────────────────┤
│ Ready to issue        │ Shortage              │ Surplus pending             │
│ 8 materials           │ 2 materials           │ 3 materials                 │
├──────────────────────┴───────────────────────┴─────────────────────────────┤
│ Material balance                                                            │
│ - Dây điện: planned 1200 · received 600 · issued 500 · shortage 600         │
│ - Sơn Dulux: planned 36 · issued 20 · used 18 · surplus 2                   │
│ [Receive] [Issue] [Transfer] [Return surplus]                               │
└────────────────────────────────────────────────────────────────────────────┘
```

Warehouse must support:

- receive from procurement
- issue to project/site
- transfer
- return to warehouse/subcontractor
- shortage visibility
- planned vs received vs issued vs used vs surplus
- stock movement traceability
- no silent balance mutation

### Site Supervisor / Giám Sát

```txt
Site Supervisor Surface
┌────────────────────────────────────────────────────────────────────────────┐
│ Thi công hôm nay                                         [Daily report]    │
├──────────────────────┬───────────────────────┬─────────────────────────────┤
│ Today's tasks         │ Material usage         │ Issues                      │
│ Ceiling install       │ Gypsum 80/120 issued   │ Pantry leak · High          │
│ Paint prep            │ Paint shortage         │ Photo required              │
├──────────────────────┴───────────────────────┴─────────────────────────────┤
│ Required evidence                                                           │
│ Photos: before/after · Manpower · Progress % · Material usage · Blockers    │
│ [Upload photos] [Report issue] [Complete step]                              │
└────────────────────────────────────────────────────────────────────────────┘
```

Site supervisor must support:

- daily report
- site photos
- manpower
- progress percent
- material usage
- issue reporting
- blocker reporting
- construction checklist completion
- handoff to QA/QC acceptance

### QA/QC

```txt
QA/QC Surface
┌────────────────────────────────────────────────────────────────────────────┐
│ Nghiệm thu                                              [Start acceptance] │
├──────────────────────┬───────────────────────┬─────────────────────────────┤
│ Ready for check       │ Failed items           │ Rework status               │
│ 4 areas               │ 2 open                 │ 1 waiting verification      │
├──────────────────────┴───────────────────────┴─────────────────────────────┤
│ Checklist                                                                   │
│ [ ] Trần thạch cao phẳng, không nứt                                         │
│ [ ] Đèn đúng vị trí theo bản vẽ R2                                          │
│ [ ] Ảnh nghiệm thu đã upload                                                │
│ [Fail and create rework]                              [Pass acceptance]     │
└────────────────────────────────────────────────────────────────────────────┘
```

QA/QC must support:

- acceptance checklist
- pass/fail result
- required photo evidence
- failed item notes
- punch list
- create rework
- verify rework completion
- preserve acceptance history

### Accounting / Kế Toán

```txt
Accounting Surface
┌────────────────────────────────────────────────────────────────────────────┐
│ Kế toán                                                   [Approve budget] │
├──────────────────────┬───────────────────────┬─────────────────────────────┤
│ Budget status         │ Payables              │ Receivables                 │
│ Over +42,000,000      │ Due 5                 │ Waiting receipt 2           │
├──────────────────────┴───────────────────────┴─────────────────────────────┤
│ Financial work                                                              │
│ - Review BOQ budget variance                         [Approve budget]      │
│ - Vendor payment due: ABC Materials                   [Record payment]      │
│ - Client milestone 30% not received                  [Confirm receivable]   │
└────────────────────────────────────────────────────────────────────────────┘
```

Accounting must support:

- budget approval
- payable review/approval/payment
- receivable review/confirmation
- payment approval
- milestone readiness
- financial closure
- director escalation when thresholds are exceeded

### Construction Team / Subcontractor

```txt
Construction Team Surface
┌────────────────────────────────────────────────────────────────────────────┐
│ Việc được giao                                             [Start]         │
├────────────────────────────────────────────────────────────────────────────┤
│ Task: Thi công trần thạch cao khu pantry                                   │
│ Due: 2026-06-15 · Required evidence: before/after photos                   │
│ Instructions: follow drawing Layout R2                                     │
│ [Upload evidence] [Report blocker] [Complete]                              │
└────────────────────────────────────────────────────────────────────────────┘
```

Construction/subcontractor users should see only assigned execution tasks, instructions, required evidence, deadlines, and supervisor feedback.

### Client / Chủ Đầu Tư

```txt
Client Surface
┌────────────────────────────────────────────────────────────────────────────┐
│ Client Review                                             [Confirm]       │
├────────────────────────────────────────────────────────────────────────────┤
│ Pending: Confirm layout R2                                                  │
│ Files shared: layout-R2.pdf, render-pantry.jpg                              │
│ Decision options: Confirm / Request changes / Comment                       │
│ [Request changes]                                      [Confirm design]     │
└────────────────────────────────────────────────────────────────────────────┘
```

Client users must see only client-facing files, confirmation requests, acceptance context, and handover context. Internal cost, supplier, staff, and audit details must stay hidden unless explicitly shared.

## `/my-work` Personal Inbox

Keep the detailed `/my-work` plan, but classify it correctly.

Purpose:

- Personal cross-project queue.
- Fast access to work items assigned to the current user.
- Department queue when the user has department responsibility.
- Pending approvals.
- Watched items.
- Quick link back to Project Operations Dashboard.

`/my-work` must not replace project-level operations.

### `/my-work` Shared Layout Contract

```txt
/my-work
┌───────────────────┬────────────────────────────────┬──────────────────────┐
│ Work Queue         │ Selected Work                  │ Context / Activity   │
│ - Mine             │ - Work title                   │ - Project context    │
│ - Department       │ - Node / Step                  │ - Files / Images     │
│ - Pending approval │ - Required checklist/evidence  │ - Controlled thread  │
│ - Watching         │ - One primary action           │ - Audit timeline     │
└───────────────────┴────────────────────────────────┴──────────────────────┘
```

Required shared UI elements:

- Header with current user, active role/department, project filter, and compact summary chips.
- Summary chips: `Need action`, `Overdue`, `Blocked`, `Waiting approval`, `Done today`.
- Filters: project, status, deadline, work type, priority, role/department.
- Work item cards: title, project, node/step, owner/role, deadline, status, blocker, primary CTA.
- Detail panel: description, checklist, required evidence, related domain records, controlled activity thread, audit timeline.
- Project context link: `Open project dashboard`.
- States: loading, empty, no permission, blocked, overdue, waiting approval, completed, API error.

### `/my-work` Role Variants

Render only variants relevant to the current user:

- PM: blockers, delayed work, approvals, cross-department coordination.
- Director: high-level approvals, major risks, financial exposure.
- Design: drawings, revisions, review, client confirmation.
- QS: BOQ, cost impact, budget variance, design clarification.
- Procurement: PRs, suppliers, ordering, delivery blockers.
- Warehouse: receive, issue, transfer, surplus, shortage.
- Site Supervisor: daily reports, photos, issues, construction checklist.
- QA/QC: acceptance, pass/fail, punch list, rework verification.
- Accounting: budget, payables, receivables, payments, closure.
- Construction Team/Subcontractor: assigned tasks and evidence.
- Client: confirmations and handover decisions.

## Controlled Activity / Chat

Chat is implemented as typed operational activity.

Allowed activity types:

- `reminder`
- `status_report`
- `blocker`
- `question`
- `decision`
- `file_note`
- `system_event`

Thread scopes:

- `project`
- `pipeline_run_node`
- `pipeline_run_step`
- `work_item`
- `boq_item`
- `procurement_request`
- `stock_movement`
- `daily_report`
- `issue`
- `acceptance`
- `rework`
- `replanning`
- `handover`
- `payable`
- `receivable`
- `payment`

Production behavior:

- Every activity must have a scope.
- No global room for casual discussion.
- Activity composer should ask for type first when needed.
- System events should be generated by workflow transitions.
- Decision activities should reference approval/action records.
- File notes should reference evidence/document records.
- Reminder activity should be actionable and optionally due-dated.
- Activity visibility must follow project membership, role, department, client visibility, and explicit permissions.

## Evidence / Documents / Images

Evidence must be first-class workflow data, not just file URLs.

Required evidence fields:

- `id`
- `project_id`
- `work_item_id`
- `target_type`
- `target_id`
- `evidence_type`
- `document_type`
- `name`
- `file_url`
- `mime_type`
- `file_size`
- `caption`
- `revision_no`
- `uploaded_by`
- `visibility`
- `status`
- `created_at`

Evidence requirements:

- Work completion must fail if required evidence is missing.
- Evidence should attach to work item and domain record when relevant.
- Design drawings need revision tracking.
- Site photos need caption/context.
- Acceptance photos need checklist-item linkage.
- Client-facing evidence must be explicitly visible to client.
- Avoid binary blobs in JSON fields. Store metadata in database and files through a storage path/service.

## RBAC / Data Scope

UI promise: departments, roles, and personnel do not see each other's unrelated operating surfaces.

Backend enforcement must consider:

- authenticated user
- user roles
- staff record
- project membership
- department
- project role assignment
- work item assignment
- required permission
- client visibility flag
- explicit watcher/approver scope

Frontend enforcement must:

- filter sidebar navigation
- filter project dashboard surfaces
- filter actions in work item detail
- show no-permission states when appropriate
- never be the only control

Seed roles need production permissions for:

- design
- qs
- procurement
- warehouse
- supervisor
- qa_qc
- accounting
- construction_team
- subcontractor
- client
- director
- pm

## Backend/API Direction

Add or refactor toward these contracts:

- `GET /me/work-items`
- `GET /projects/:id/operations`
- `GET /projects/:id/work-items`
- `GET /projects/:id/department-surfaces`
- `GET /work-items/:id`
- `POST /work-items/:id/start`
- `POST /work-items/:id/complete`
- `POST /work-items/:id/block`
- `POST /work-items/:id/resolve-blocker`
- `POST /work-items/:id/request-review`
- `POST /work-items/:id/request-approval`
- `POST /work-items/:id/approve`
- `POST /work-items/:id/reject`
- `POST /work-items/:id/request-changes`
- `GET /work-items/:id/activities`
- `POST /work-items/:id/activities`
- `GET /work-items/:id/evidence`
- `POST /work-items/:id/evidence`
- `GET /projects/:id/activities`
- `POST /projects/:id/activities`

Dashboard payload should include:

- project summary
- health counts
- workflow timeline
- active work queue
- blockers
- approvals
- latest evidence
- department surfaces allowed for current user
- activity summary
- audit summary

## Frontend Direction

Primary frontend targets:

- `frontend/src/modules/projects/ProjectDetailPage.tsx`
- `frontend/src/modules/planning/PlanningPage.tsx`
- `frontend/src/modules/my-work/MyWorkPage.tsx`
- `frontend/src/layouts/AppShell.tsx`
- `frontend/src/app/router.tsx`
- shared work-item components under `frontend/src/modules/work-items` or `frontend/src/components` if reused broadly
- `frontend/src/lib/types/domain.ts`
- `frontend/src/lib/api/client.ts` only if API helper changes are needed

UI component strategy:

- Project dashboard owns project-level layout.
- Department surfaces are modular components.
- Work item detail is reusable in project dashboard and `/my-work`.
- Activity thread is reusable but scoped.
- Evidence uploader is reusable but policy-driven by work type.
- Action footer renders one primary CTA plus contextual secondary actions.
- Dialog/modal patterns should reuse shared components for create/view/edit flows.

## Implementation Slices For Future Agents

### Slice 1: Contract And Scope Audit

Goal: verify exact repo state before code changes.

Expected work:

- Audit current routes, schema, seed roles, frontend modules, and auth context.
- Confirm whether to add `work_items` and activity/evidence tables.
- Confirm production status names and transition rules.
- Produce review package before code changes.

Likely files:

- `backend/migrations`
- `backend/internal/core/routes.go`
- `backend/internal/core/types.go`
- `backend/internal/core/repository_domain.go`
- `backend/internal/core/service.go`
- `frontend/src/lib/types/domain.ts`

### Slice 2: Runtime Work Item Foundation

Goal: create durable work-item storage and generation.

Expected work:

- Add migration for `work_items`, assignments, activities, and evidence if selected.
- Generate initial work items when a pipeline run starts.
- Map runtime steps to work items.
- Preserve template-vs-run history.
- Add repository/service helpers for project work and personal work.

Validation:

- Backend tests for pipeline start creating expected work items.
- Tests for role/project scoped queries.

### Slice 3: Project Operations API

Goal: provide dashboard-ready data.

Expected work:

- Add `GET /projects/:id/operations`.
- Aggregate project health, timeline, active work, blockers, approvals, department surfaces, latest activity, and latest evidence.
- Enforce data scope server-side.
- Keep response shape stable for frontend.

Validation:

- Backend tests for PM, director, department role, and client views.

### Slice 4: Work Item Actions And Audit

Goal: make work execution production-grade.

Expected work:

- Implement start/complete/block/resolve/request-review/request-approval transitions.
- Validate required evidence.
- Write audit logs for every transition.
- Preserve actor/time/comment/payload.
- Stop using blind step submit as the production completion mechanism.

Validation:

- Tests for status transitions.
- Tests for required evidence failure.
- Tests for audit entries.

### Slice 5: RBAC And Data Scope Hardening

Goal: make UI visibility match backend permissions.

Expected work:

- Add route-level permission enforcement across modules.
- Fix seed permissions for operational roles.
- Scope list endpoints by project membership and permission.
- Add no-permission behavior on frontend.

Validation:

- Backend permission tests per seeded demo user.
- Manual API checks with seeded users.

### Slice 6: Project Dashboard Production UI

Goal: replace project detail module directory with the operations dashboard.

Expected work:

- Build project header, health strip, workflow timeline, active work, command center, and department surfaces.
- Keep module links secondary.
- Render PM/director/project-member scoped views.
- Use one-primary-action pattern.
- Add loading, empty, blocked, error, and no-permission states.

Likely files:

- `frontend/src/modules/projects/ProjectDetailPage.tsx`
- reusable components under project/work-item modules

Validation:

- `cd frontend && npm run build`
- Browser checks only if explicitly requested by the user.

### Slice 7: Department Surface Components

Goal: build production role/department surfaces.

Required surfaces:

- PM
- Director
- Design
- QS
- Procurement
- Warehouse
- Site Supervisor
- QA/QC
- Accounting
- Construction Team/Subcontractor
- Client

Expected work:

- Use a registry keyed by role/department/work type.
- Avoid duplicate form, toast, validation, and dialog scaffolding.
- Keep each surface visually focused with one dominant CTA.

### Slice 8: `/my-work` Personal Inbox

Goal: implement `/my-work` as a personal inbox after the project work contract exists.

Expected work:

- Add `/my-work` route.
- Use `GET /me/work-items`.
- Render shared work queue/detail/activity/evidence components.
- Add `Open project dashboard` as a first-class context link.
- Make authenticated default route configurable or role-aware, not blindly `/my-work` for everyone.

Validation:

- Frontend build.
- Permission/data-scope tests for work item visibility.

### Slice 9: Evidence Upload And Document Flow

Goal: make documents/images real workflow evidence.

Expected work:

- Add evidence/document metadata APIs.
- Attach evidence to work items and domain records.
- Support design revisions, site photos, acceptance checklist photos, and client-visible documents.
- Validate evidence requirements before completion.

Validation:

- Missing evidence blocks completion.
- Evidence upload writes activity and audit records.

### Slice 10: Controlled Activity And Reminders

Goal: replace generic chat with scoped operational activity.

Expected work:

- Add typed activity storage and APIs.
- Render activity in project dashboard and `/my-work`.
- Support reminder/status/blocker/question/decision/file note/system event.
- Generate system events from workflow transitions.
- Add anti-spam-by-design behavior through scope and type requirements.

### Slice 11: Domain Transition Fixes

Goal: close domain correctness gaps before broad production usage.

Expected work:

- Procurement generation must use approved BOQ items only.
- Acceptance fail must preserve history and create issue/rework path.
- Stock operations must preserve balance and traceability.
- Handover must check acceptance, punch list, warehouse closure, payables, receivables.
- Accounting transitions must preserve approval and payment actor context.

Validation:

- Backend domain tests for each transition.

### Slice 12: Regression Coverage And Production QA

Goal: prevent workflow, RBAC, and UI regressions.

Tests needed:

- Template start creates/derives work items.
- Role-scoped `/me/work-items`.
- Project operations payload differs by role.
- Permission denial for unrelated module actions.
- BOQ approved-only procurement generation.
- Stock movement balance.
- Acceptance fail creates issue/rework path.
- Handover blockers.
- Frontend build.

Browser/headless verification remains skipped unless the user explicitly requests it in that turn.

## Suggested Skills For Future Agents

- `consflow-repo-architect` for cross-stack slicing.
- `consflow-pipeline-domain` for workflow invariants and transition safety.
- `consflow-backend-workflow` for API, service, repository, migration work.
- `consflow-frontend-workflow` for project dashboard, `/my-work`, navigation, and shared UI.
- `consflow-diagnose` for validation against Docker/API behavior.
- `consflow-handoff` when updating this document after substantial planning or implementation.

## Documentation Notes

- Closed plans should stay outside this active document under `docs/plan-closed`.
- Do not reintroduce closed Pipeline Builder connector/adorner implementation details into this file.
- Future agents should treat this file as active product/implementation context, not as a completed implementation record.
