# [Closed] Work Item Detail Workflow Sidebar Active Node Border

Status: closed

Mục đích chính:
- Make the workflow node containing the active Checklist / Step visually identifiable by highlighting the node border in the left work item detail timeline.

Will be done:
- Highlight the outer border/ring for the workflow node that contains the active work item step.
- Keep the active step highlight unchanged.
- Keep other workflow nodes on the neutral border style.
- Preserve expand/collapse, auto-scroll, project-scoped links, backend APIs, workflow gates, form/evidence/signature/PDF behavior, and action semantics.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `Approve`.
- Re-read Consflow frontend workflow, workflow orchestration, module memory, AGENTS, module keyword map, domain map, UI guardrails, and current `ProjectWorkspaceLayout` sidebar code.
- Updated `frontend/src/modules/projects/ProjectWorkspaceLayout.tsx` so the node containing the active work item step gets a blue border/ring, blue-tinted header line, and blue-tinted internal dividers.
- Kept active step highlight, expand/collapse, auto-scroll, backend APIs, workflow gates, forms, evidence, signatures, PDFs, and action semantics unchanged.
- Ran `cd frontend && npm run build` successfully; Vite reported only the existing large chunk warning.
- Browser verification was skipped per repository rule because it was not explicitly requested.
