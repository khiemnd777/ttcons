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
