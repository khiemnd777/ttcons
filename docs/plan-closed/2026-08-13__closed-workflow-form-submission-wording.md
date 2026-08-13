# [Closed] Workflow Form Submission Wording

## Will be done

- Present workflow-form submission as `Chốt hồ sơ` / `Đã chốt` so users understand that the version is locked inside the work item rather than sent to a recipient or approver.
- Keep the technical `submitted` status, API routes, persistence, permissions, completion gates and approval workflows unchanged.
- Apply the contextual wording consistently across the workflow-form panel, work-item summaries and project workflow sidebar without changing other domains that use `submitted` for real send/review flows.
- Add focused regression coverage, update central/frontend/workflow mapping and run non-browser frontend verification.

## Done

- Confirmed the current workflow-form submit API has no recipient and creates no approval request; it persists the submitted revision, actor/time, workflow-form event, work-item activity, audit and completion payload.
- Produced the approved review package and Mockup as Text; received explicit `OK triển khai` for contextual `Chốt hồ sơ` / `Đã chốt` wording with unchanged backend behavior and historical technical state.
- Added contextual workflow-form status/event helpers and applied `Chốt hồ sơ` / `Đã chốt` across the form action center, locked-version explanation, validation/toast/signature/PDF guidance, form history, project workflow tree, work cards, detail header/support tab, Planning metric and completion-requirement copy.
- Preserved the shared `submitted` -> `Đã gửi` presentation for other domains, the persisted workflow-form `submitted` status and all API, permission, completion and approval behavior. The workflow-specific activity event label and exact legacy system-event comment now render as `Đã chốt hồ sơ nghiệp vụ` without rewriting stored history.
- Added focused helper/source-contract regression coverage and updated central/frontend/workflow mapping, including the optional contextual `StatusBadge` label contract.
- Technically verified the current frontend working tree with all 74 tests, the TypeScript/Vite production build (2.73s), the bundle budget (121.5 KiB gzip eager entry, 110 JavaScript chunks under the 250 KiB ceiling) and `git diff --check`. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.
