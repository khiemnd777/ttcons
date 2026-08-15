# [Closed] Work Item Permission Information Semantics

## Will be done

- Present the work-item required permission as access information rather than an incomplete checklist state.
- Replace the hollow neutral circle on the permission row with a blue shield, rename the row to `Quyền thao tác`, add the `Thông tin truy cập` badge, and use `Khi không thể thao tác` guidance.
- Preserve source-backed permission metadata without claiming that the current actor has or lacks the permission; keep API, RBAC, workflow gates and available actions unchanged.
- Add focused frontend regression coverage, update the central/frontend/workflow-orchestration maps, and run non-browser frontend tests/build.

## Done

- Confirmed from the supplied Edge screenshot and current source that the permission row is assigned `neutral`, while every neutral requirement renders as a hollow `Circle`; the row is not counted as an incomplete blocker despite looking like an unchecked task.
- Produced the review package and Mockup as Text, then received explicit `OK triển khai` approval for the UI-only design.
- Replaced the hollow permission marker with a dedicated blue shield, renamed the row to `Quyền thao tác`, added the responsive `Thông tin truy cập` badge and changed its helper label to `Khi không thể thao tác`.
- Updated source-backed permission copy and its missing-metadata fallback without exposing the raw permission key or inferring whether the current actor has the permission. API, RBAC, available actions and workflow gates remain unchanged.
- Added focused presentation/source-contract regressions and updated the central, frontend and workflow-orchestration maps for the new access-information semantic.
- Technically verified the current frontend source with focused tests (15/15), the complete frontend suite (108/108), the TypeScript/Vite production build and `git diff --check`. Browser/UAT was not requested and remains `NOT UX ACCEPTED`.
