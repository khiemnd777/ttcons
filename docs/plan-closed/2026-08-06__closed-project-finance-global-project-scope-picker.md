# [Closed] Project Finance Global Project Scope Picker

## Will be done

- No mandatory implementation or non-browser verification remains.
- Browser/UAT remains `NOT UX ACCEPTED` because browser verification was not requested in this implementation turn.

## Done

- Loaded the mandatory central, backend, frontend and root-cause maps plus accounting-finance and reporting-analytics domain maps.
- Confirmed from current source that global Finance eagerly loads all `/projects`, automatically selects the first result, applies selection immediately through `project`, and places both project selection and create actions in the page header.
- Confirmed the project-scoped route and finance workspace/actions already enforce project permissions, while `/reports/options` is report-permission-scoped and must not be reused as a Finance API.
- Confirmed migration V90's project search/order indexes can serve the new picker, so no database migration is required.
- Produced the evidence-backed review package and Mockup as Text; received explicit `OK triển khai` for the reviewed scope.
- Added `GET /project-finance/projects?search&cursor&limit&selected_id`, guarded by `project_finance.view`; list/search/cursor and direct selected resolution share non-cancelled actor scope plus per-project finance permission and do not expose an inaccessible project name.
- Extracted shared `AsyncProjectPicker` and `ProjectScopeActions`; Reports retains its `/reports/options`, all-project option and existing draft/applied behavior through a thin wrapper.
- Reworked only global `/accounting` to use searchable draft selection, explicit Apply, Reset to no project and no eager `/projects` load or first-project fallback. `/projects/:id/accounting` remains route-fixed.
- Moved material-cost and cash-request create actions into the workspace toolbar; queries, dialogs, actions and CSV remain bound to the applied project ID.
- Added backend permission/search/cursor/selected endpoint coverage plus frontend Finance/Reports source-contract regressions. Updated central/backend/frontend/root-cause and accounting/reporting domain maps; no migration map was needed because the change reuses V90 indexes.
- Technically verified the final source with backend `go test ./...`, frontend 48/48 tests, production TypeScript/Vite build, bundle budget (121.4 KiB gzip entry; 107 JavaScript chunks under the 250 KiB ceiling) and `git diff --check`.
