# Active Plan: Consflow Module Memory Cache

## Objective

Create a durable repo-scoped cache so agents can map prompt keywords to Consflow modules, features, utilities, routes, handlers, services, repositories, migrations, and domain risks without rereading the whole codebase.

## Will Be Done

- Add a new `consflow-module-memory` skill with concise trigger instructions.
- Add a keyword-to-module reference map covering frontend modules, backend/API owners, tables, shared utilities, and high-risk invariants.
- Wire repo architect docs and custom subagents to use the cache before broad scans.
- Validate agent artifacts with `.agents/scripts/validate.sh`.

## Done

- Review package presented and user confirmed implementation.
- Added `.agents/skills/consflow-module-memory/SKILL.md`.
- Added `.agents/skills/consflow-module-memory/references/module-keyword-map.md`.
- Added `.agents/skills/consflow-module-memory/agents/openai.yaml`.
- Wired repo docs and custom subagent instructions to point at the new module memory cache.
- Ran `bash .agents/scripts/validate.sh`: passed.
- Parsed new skill frontmatter and `agents/openai.yaml` with Ruby YAML: passed.
- Attempted `quick_validate.py`; blocked because the available Python runtimes do not include `yaml` / PyYAML.

# Active Plan: Pipeline Builder Checklist Drag Drop

## Objective

Allow Pipeline Builder node `Checklist / Step` rows to be reordered by drag-drop while preserving the existing persisted `order_index` behavior.

## Will Be Done

- Add drag handle, drag state, drop indicator, and empty/pending states to the node inspector checklist list.
- Reuse the existing pipeline step update endpoint to persist reordered `order_index` values.
- Remove the legacy up/down arrow sort controls after drag-drop is available.
- Validate the frontend with `cd frontend && npm run build`.

## Done

- Review package and text mockup were presented.
- User confirmed implementation with `OK`.
- Added native drag-drop handling and visual drop indicators to the node inspector checklist rows.
- Updated checklist reorder persistence to submit a complete ordered step-id list and update `order_index` through `PUT /pipeline-steps/:id`.
- Ran `cd frontend && npm run build`: passed.
- Browser/Playwright verification skipped per repo instruction because it was not explicitly requested.
- Follow-up review package and text mockup for removing legacy up/down arrow sort controls were presented.
- User confirmed arrow removal with `OK`.
- Removed the up/down arrow sort controls from checklist rows.
- Ran `cd frontend && npm run build` after arrow removal: passed.
