# [Closed] Enum Label Coverage

## Objective

Fill missing enum/status labels in the Consflow admin UI so user-facing tables, chips, filters, audit timelines, and work item panels do not fall back to raw enum keys.

## Will Be Done

- Extend shared enum label groups in `frontend/src/lib/utils/status.ts` for statuses, work item metadata, activity scopes, visibility, and audit event/target types.
- Update local work item presentation helpers to reuse shared enum labels.
- Replace visible raw renders for priority, work type, visibility, scope type, attachment status, and audit/project surfaces where they are currently shown directly.
- Run `cd frontend && npm run build` after frontend edits.

## Done

- Review matrix prepared for missing labels and user approved implementation.
- Extended `frontend/src/lib/utils/status.ts` with missing status, work type, priority, visibility, scope/source/target type, activity/evidence, and audit event labels.
- Updated status/enum chips to handle newly labeled values such as work item waiting states, `store_pending`, `loss`, and new audit targets.
- Reused shared label helpers in work item cards, detail dialogs, activity composer/thread, My Work filters, project evidence, attachment chips, and project approval labels.
- Ran `cd frontend && npm run build`: passed.
