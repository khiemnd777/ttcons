# [Closed] Project Dashboard Workflow Order Display

Status: closed

Will be done:
- Keep workflow-step displays in Project dashboard aligned with Pipeline Builder/runtime step order.
- Keep priority ranking (`blocked -> overdue -> waiting approval/review -> waiting evidence -> due date`) only for an explicit attention/priority surface.
- Add runtime node/step order fields to the project work item API response if the frontend does not already receive them.
- Run focused frontend/backend validation without browser verification unless explicitly requested.

Done:
- User confirmed the review package and Mockup as Text with `OK`.
- Confirmed current frontend uses `prioritizeWorkItems` for selected workflow-node work items, which can reorder step #4 ahead of earlier steps.
- Confirmed project work item API currently returns runtime node/step names but not runtime node/step order indexes.
- Added `run_node_order_index` and `run_step_order_index` to the project work item API response.
- Changed project work-item query ordering to runtime node order, then runtime step order.
- Added frontend workflow-order sorting for project work item lists that represent workflow sequence.
- Kept priority ranking only for the overview `Hàng ưu tiên` surface and header primary action.
- Renamed the workflow tab side panel to `Công việc theo thứ tự workflow`.
- Ran `npm run build` in `frontend` successfully.
- Ran `make test` in `backend` successfully.
- Ran `git diff --check` successfully.
- Skipped browser verification because it was not explicitly requested in this turn.
