# Controlled Activity / Operational Chat Plan

Status: closed reference as of 2026-06-13. All six batches were implemented and handed off in `docs/plan-closed/controlled-activity-chat-implementation.md`.

This file is the original planning record only. Do not treat any batch below as active implementation scope without a new review package and user confirmation.

## Intent

Add operational communication to Consflow without turning the product into a general social chat tool. Messages must stay tied to project work, decisions, blockers, evidence, reminders, and audit history.

Existing baseline:

- `work_item_activities` stores scoped activity records.
- Work item transitions already write activity and audit records.
- `POST /work-items/:id/activity` can create a basic work-item comment.
- Project Operations Dashboard and `/my-work` show activity timelines.

Missing baseline:

- Composer UI for writing activity/chat messages.
- Typed activity workflow beyond a basic comment.
- Project-level threads.
- Mentions, unread state, notifications, and reminder due dates.
- File/document attachment integration beyond current evidence metadata.
- Realtime delivery via SSE/WebSocket.

## Product Rules

- Chat is scoped to a project, work item, or domain record; no global free-form chat room.
- Every message has actor, timestamp, scope, type, and visibility.
- Decisions and blockers are first-class workflow objects, not loose text.
- Client-visible communication must be explicitly marked; internal cost, supplier, staff, and audit context stays hidden by default.
- Message creation must preserve backend permission checks; frontend hiding is not security.
- Realtime is optional and must not bypass persistence, audit, or RBAC.

## Message Types

- `comment`: normal scoped note.
- `status`: progress/status update.
- `blocker`: blocker report with optional owner and due date.
- `question`: explicit question requiring response.
- `decision`: decision record with approver/owner context.
- `reminder`: follow-up reminder with due date.
- `file_note`: note attached to an evidence/document record.
- `system_event`: generated event from workflow transitions.

## Batch 1: Composer And Typed Activity API

Goal: make existing activity records writable through production UI/API patterns.

Expected work:

- Extend activity request handling to accept typed activity payloads.
- Add validation per message type.
- Add work-item detail composer UI.
- Add project dashboard composer only for project-scoped notes when permission allows.
- Keep current activity timeline display, but label typed records clearly.

Validation:

- Backend tests for type validation, permission scope, and audit creation.
- Frontend build.
- Browser/manual QA only when explicitly requested.

Review gate before implementation:

- Confirm message type list and required fields.
- Provide Mockup as Text for work-item and project composer.

## Batch 2: Project Threads And Domain Record Context

Goal: support conversations tied to the project and important domain records.

Expected work:

- Add project-level activity endpoint and list query.
- Support target scopes for work item, BOQ, procurement, stock movement, acceptance, rework, handover, payable, receivable, and payment records where useful.
- Surface project activity in Project Operations Dashboard without overwhelming command center.
- Preserve visibility rules for client/internal records.

Validation:

- Backend tests for project and target record scope.
- Frontend build.
- Permission-denial tests for out-of-scope users.

Review gate before implementation:

- Confirm target scopes for first pass.
- Confirm client-visible communication rules.

## Batch 3: Mentions, Assignments, And Unread State

Goal: make operational messages actionable.

Expected work:

- Add mention metadata for staff/users/roles.
- Track unread/read state per user and project/work item.
- Generate actionable queues for mentions, unanswered questions, open blockers, and due reminders.
- Add `/my-work` surfaces for communication attention items.

Validation:

- Backend tests for unread isolation and mention visibility.
- Frontend build.
- Project/member scope regression tests.

Review gate before implementation:

- Confirm mention syntax and notification semantics.
- Confirm whether role mentions notify all matching members or only active project members.

## Batch 4: Attachments And Evidence Integration

Goal: connect communication to existing evidence/document workflows.

Expected work:

- Allow activity messages to reference evidence/document records.
- Add file-note UI around existing evidence metadata.
- Avoid binary storage changes unless explicitly approved.
- Ensure client visibility and internal visibility are preserved.

Validation:

- Backend tests for attachment references and visibility.
- Frontend build.
- Evidence requirement regressions still pass.

Review gate before implementation:

- Confirm whether binary upload/storage is in scope.
- Confirm supported attachment types for first pass.

## Batch 5: Realtime Delivery

Goal: optionally make operational chat live without weakening persistence.

Expected work:

- Evaluate SSE vs WebSocket against current Go/Fiber stack.
- Add realtime stream only after message persistence and RBAC are stable.
- Revalidate auth, project membership, reconnect behavior, and backfill after reconnect.
- Keep REST endpoints as source of truth.

Validation:

- Backend tests for stream authorization where feasible.
- Frontend build.
- Manual/browser QA required before calling realtime production-ready.

Review gate before implementation:

- Confirm realtime requirement and transport choice.
- Confirm local/dev deployment expectations.

## Batch 6: QA And Handoff

Goal: lock the operational chat baseline.

Expected work:

- Run backend tests and frontend build.
- Run Docker/migration checks when environment is available and approved.
- Update `docs/current-states.md`.
- Move closed implementation notes to `docs/plan-closed`.

Validation:

- Backend tests.
- Frontend build.
- `git diff --check`.
- Browser/manual QA if explicitly requested.
