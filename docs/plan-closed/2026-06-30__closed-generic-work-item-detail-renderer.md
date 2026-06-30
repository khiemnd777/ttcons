# [Closed] Generic Work Item Detail Renderer

Status: closed

Closed at: 2026-06-30

Will be done:
- Keep `WorkItemDetailDialog` as a generic shell for status, context, next action, footer actions, evidence, and activity.
- Add a primary-surface resolver so the main body renders from the work item's `work_type` and workflow metadata instead of mounting every domain panel.
- Keep domain panels available for the correct work item type only, with workflow form, evidence, and activity as supporting surfaces.
- Run `cd frontend && npm run build` after frontend edits; browser verification is skipped unless explicitly requested.

Done:
- Review package and text mockup were approved by the user with `OK triển khai`.
- Identified the current UX bug: `WorkItemDetailDialog` renders all workflow form, reconciliation, material, and acceptance panels for every work item.
- Added `WorkItemPrimarySurface` to resolve the primary panel from `work_type` and workflow metadata.
- Changed `WorkItemDetailDialog` so it renders only one primary surface, then secondary tabs for evidence, activity, and form/PDF when appropriate.
- Preserved existing domain panels and API calls, but only mounts them for the matching work item surface.
- Ran `cd frontend && npm run build` successfully.
- Skipped browser verification per repository rule because it was not explicitly requested in this turn.
