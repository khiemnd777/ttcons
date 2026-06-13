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

# Active Plan: Project Detail Tabbed Workspace

## Objective

Refactor the project detail screen so overview, workflow, work items, communication, files, audit, and members are separated into tabs instead of one long mixed page. Keep Command Center focused only on immediate coordination signals.

## Will Be Done

- Move project summary and KPI strip into an Overview tab.
- Replace the old mixed `Command Center` with a focused coordination panel that excludes activity and audit.
- Move `ActivityThread` into a dedicated `Trao đổi` tab.
- Move audit content into a dedicated `Audit` tab using existing audit summary data and a link to the scoped audit route.
- Move department work surfaces, files, module shortcuts, and project members into separate tab panels.
- Run `cd frontend && npm run build` after frontend changes.

## Done

- Review package and text mockup were presented.
- User approved implementation.
- Refactored `ProjectDetailPage` into tabbed panels: `Tổng quan`, `Workflow`, `Công việc`, `Trao đổi`, `Hồ sơ`, `Audit`, and `Thành viên`.
- Removed `ActivityThread` and audit content from `Command Center`.
- Added dedicated `Trao đổi`, `Hồ sơ`, and `Audit` tab panels using existing API/data contracts.
- Kept backend/API unchanged.
- Ran `cd frontend && npm run build`: passed.
