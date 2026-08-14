# [Closed] Unified Project Detail And Edit Dialog

## Will be done

- Remove the project explorer eye action and keep the pencil as the only entry point for project metadata review/editing, alongside the existing project-workspace folder action.
- Merge the useful read-only project summary into the existing edit dialog, preserving status, progress, partner snapshot and partner tax-code visibility without another API request.
- Replace the technical `catalog`/`backend snapshot` wording with the approved user-facing labels `Chọn chủ đầu tư có sẵn`, `Chủ đầu tư đã chọn` and a plain-language helper.
- Keep the existing project update payload, permissions, backend behavior and workflow-template selection contract unchanged.
- Add focused frontend coverage, update the central/frontend/project-planning maps and run non-browser frontend tests/build.

## Done

- Confirmed from the supplied Edge screenshots and current source that `ProjectsPage` has separate eye/view and pencil/edit flows over the same list record.
- Produced the approved review package and Mockup as Text; received explicit `OK triển khai` approval.
- Replaced the `view|edit` dialog state with one `editingProject`, removed the eye action and `RecordDetailDialog`, and retained the pencil plus project-workspace folder actions.
- Added a compact read-only summary for status, bounded progress, partner snapshot and partner tax code above the responsive grouped edit form; save/cancel, inline validation, warning/error/success toast and the existing `PUT /projects/:id` payload remain intact.
- Added focused `projectEditDialog.test.ts` coverage and updated the central, frontend and project-planning maps for the single-entry dialog contract.
- Replaced the technical partner wording with `Chọn chủ đầu tư có sẵn`, `Chủ đầu tư đã chọn` and the approved plain-language helper; extended the regression contract and central/frontend/project/admin mappings without changing partner behavior.
- Frontend verification on the current working tree after the approved wording update: `npm test` passes 99/99 and `npm run build` passes. Browser/UAT was not run under the repository rule, so the UI remains `NOT UX ACCEPTED` until a separate browser acceptance pass.
