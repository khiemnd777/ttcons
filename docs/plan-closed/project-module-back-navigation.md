# [Closed] Project Module Back Navigation

## Objective

Add a consistent back button on project-scoped module pages opened from Module Shortcuts so users can return to `/projects/{id}`.

## Will Be Done

- Add a shared project back link component for module headers.
- Show the back link on project-scoped shortcut pages only.
- Keep global module pages unchanged.
- Run the frontend production build.

## Done

- Review package and Mockup as Text presented.
- User confirmed implementation with "OK".
- Added shared `ProjectBackLink` component.
- Wired the back link into project-scoped module headers for planning/pipeline, BOQ, procurement, warehouse, operations, acceptance, rework, handover, accounting, debt, and audit.
- Kept global module routes without the project back link.
- Ran `npm run build` in `frontend`: passed.
- Closed by user request.
