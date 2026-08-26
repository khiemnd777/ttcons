# [Closed] Notification Platform generic đa domain

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED. Closed by user request on 2026-08-26.

## Will be done

- Replace the Work Report-specific notification vertical slice with a generic, versioned domain-event catalog, transactional outbox, audience/visibility policy resolver, template renderer, actor inbox, preferences and delivery-attempt foundation.
- Keep business domains authoritative: notification emission must occur only inside successful source transactions and must not replace audit/activity, approval state, work queues, report counts, stock movements, acceptance history or finance reconciliation.
- Migrate the four existing Work Report events as the first real producer, then register the reviewed cross-domain event catalog without inventing unsupported domain transitions or enabling noisy delivery before each producer is integrated.
- Return server-owned presentation metadata and safe internal action paths so the frontend no longer switches on Work Report event types or constructs domain routes.
- Consolidate shell notification streaming behind one actor-level provider with revision-based reconnect reconciliation; keep Reports and My Work badges source-backed.
- Add additive migrations, backend/frontend regression coverage, update all affected module/domain maps and run current-tree verification. Browser/UAT remains skipped unless separately authorized.

## Done

- User approved the generic architecture, phased rollout, in-app-first delivery boundary, non-mutable action/security notifications and the bell/full-inbox Mockup as Text with `OK triển khai theo review package này`.
- Loaded the mandatory central/backend/frontend/root-cause maps plus Workflow, Planning, Construction, Commercial, Materials, Quality, Finance, Admin and Reporting domain guardrails.
- Confirmed the current V125 implementation is a Work Report vertical slice: producer-owned recipients/templates/actions, a hard-coded four-type service catalog, `work_report.view` inbox visibility and frontend type/action switches.
- Added V126 as the generic platform schema: versioned event catalog/templates, immutable domain events, transactional outbox, generic actor projection metadata, per-event/category preferences, delivery attempts, actor revisions and the future diagnostics permission. The reviewed cross-domain inventory contains 74 event types; only source-integrated events are enabled.
- Added `notification_platform.go` with one domain-event envelope, atomic event/outbox persistence, idempotent dispatch, bounded retry/dead-letter and stale-lease recovery, safe template/route rendering, current-recipient filtering, expiration cleanup and correlation-based action resolution. Enabled audience policies are `explicit_users` and current `approval_deciders`; an enabled unsupported policy fails closed.
- Migrated the four Work Report transitions to the generic producer and removed the old direct notification-insert helper. Work Report revision/event/audit remains authoritative and source commit now includes only immutable notification facts; the asynchronous projector owns recipient rows and targeted invalidation.
- Added V127 and an atomic `activity.mentioned` producer after project-scoped user/role mention resolution. Activity attention/read state remains independent from notification read/archive state, and same-named roles outside the project are not recipients.
- Added V128 and atomic Work Item approval request/outcome producers. The request audience reuses the current approval-decision policy rather than a fixed PM; outcomes notify the original requester and resolve all pending notifications in the same Work Item approval thread. Already-resolved requests are suppressed at delayed dispatch.
- Expanded the actor API with generic state/event/category/project/search/date filters, cursor paging, unresolved action summary, catalog/project options, archive and category/event preferences. `unread_total` and unresolved `action_required` are independent; reading a notification does not remove an outstanding action.
- Replaced frontend event/action switches with server-owned presentation metadata, finite icon/tone design tokens and safe same-origin internal routes. Added an accessible responsive bell panel plus `/notifications` full inbox with action/unread/archive states, categories, project/search/date filters, pagination and preferences. My Work accepts declarative `?task=` selection for notification deep links.
- Consolidated realtime behavior into one app-level actor SSE provider. Targeted invalidation refreshes actor-owned query families; actor revision comparison reconciles persisted inbox state after reconnect. Reports and My Work navigation badges remain their existing source-backed totals and are not notification counts.
- Registered the remaining Workflow, Construction, Commercial/Procurement, Materials/Warehouse, Quality/Acceptance/Handover, Finance and Admin/Security candidates disabled, and documented the authoritative producer/audience boundary in every affected domain map. This prevents unreviewed noisy delivery while giving later modules one catalog + producer integration path instead of another notification implementation.
- Local PostgreSQL migrations V127-V128 applied after V126 and the checksum rerun reports 128/128 skipped. Current catalog evidence is 74 total / 9 enabled events with no pending/retry/dead outbox rows after integration tests.
- Current working-tree verification at HEAD `f1d1f3a` plus the listed dirty-tree changes passed at 2026-08-26 11:48 +07: focused PostgreSQL integrations for Work Reports, project mentions and Work Item approvals; full backend `go test ./...`; 274/274 frontend tests; TypeScript/Vite production build; 250 KiB gzip bundle budget with 136 chunks; migration checksum rerun; and `git diff --check`.

## Skipped / remaining acceptance

- Browser/UAT was not authorized in this turn and remains `NOT UX ACCEPTED`.
- Email/push adapters, digest scheduler, admin dead-letter diagnostics UI and the 65 disabled catalog producers are intentionally not active. They are extension points, not verified delivery channels or completed business integrations.
