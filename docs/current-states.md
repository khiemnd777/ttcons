# Current States

## Project-First Folder UI

Status: complete

Will be done:
- Make `/projects` the primary post-login entry for users with `project.view`.
- Reduce the main sidebar to high-level entry points instead of exposing every project domain module as a global navigation item.
- Convert `frontend/src/modules/projects/ProjectsPage.tsx` from a dense table into a folder/tree explorer for construction projects.
- Convert the project detail overview in `frontend/src/modules/projects/ProjectDetailPage.tsx` to a compact project operations summary that sits inside the project workspace layout.
- Add project focus mode for `/projects/:id/*` so the project business tree becomes the main menu and the global app sidebar/header is hidden.
- Keep existing project-scoped routes and backend APIs unchanged.
- Run `cd frontend && npm run build` after frontend edits.

Done:
- User approved the visual mockup direction: `/projects` as project explorer and project detail as a folder-style business tree.
- Updated `HomeRedirect` so users with `project.view` land on `/projects`.
- Reduced the main sidebar to project-first high-level navigation and removed the project domain modules from the primary global menu.
- Reworked `ProjectsPage` into a searchable/filterable folder tree explorer with expandable construction project rows.
- Reworked `ProjectDetailPage` overview into a folder-based operations workspace with a persistent business tree and compact work table.
- Added `ProjectWorkspaceLayout` focus mode for `/projects/:id/*` with project top bar, fixed business tree menu, and scoped content outlet.
- Nested all project-scoped routes under `ProjectWorkspaceLayout` while preserving their existing URLs and permission guards.
- Updated `AppShell` so project focus mode hides the global sidebar/header, matching the focused `pipeline-builder` behavior.
- Removed the old embedded business tree from `ProjectDetailPage`; the project workspace layout is now the single owner of that menu.
- Added query-tab support for project members/files links from the workspace tree.
- Kept backend APIs, routes, migrations, and project-scoped module routes unchanged.
- Ran `cd frontend && npm run build` successfully after both project-first and focus-mode changes.

## Consflow Impact Analysis Skill

Status: complete

Will be done:
- Add `consflow-impact-analysis` as a repo-scoped skill for feature overlap and logic collision analysis.
- Add UI metadata for the new skill.
- Wire the new skill into the Consflow skill README, module routing, and repo architecture workflow.
- Validate the skill artifact with targeted file inspection and `rg`.

Done:
- Review package approved by the user.
- Added `.agents/skills/consflow-impact-analysis/SKILL.md`.
- Added `.agents/skills/consflow-impact-analysis/agents/openai.yaml`.
- Wired `consflow-impact-analysis` into `.agents/skills/README.md`.
- Added impact-analysis pairing guidance to `consflow-module-memory`.
- Added cross-feature collision routing to `consflow-repo-architect`.
- Validated `SKILL.md` frontmatter and `openai.yaml` with Ruby YAML checks because the Python validator is missing `PyYAML` in this environment.
