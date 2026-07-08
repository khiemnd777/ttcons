# Consflow Agent Operating Notes

This repo uses local Codex customization surfaces:

- `AGENTS.md` for durable repository instructions.
- `.agents/skills` for repo-scoped reusable workflows.
- `.codex/agents` for project-scoped custom subagents.
- `.agents/skills/consflow-module-memory/references/module-keyword-map.md` for mandatory central keyword-to-module routing before broad code scans, diagnosis, review, planning, or code edits.
- `.agents/skills/consflow-module-memory/references/backend-function-map.md` for backend route, handler, service, repository, helper, and schema ownership.
- `.agents/skills/consflow-module-memory/references/frontend-ui-utility-map.md` for frontend route, page, UI surface, component, helper, API usage, type, and status ownership.
- `.agents/skills/consflow-module-memory/references/root-cause-triage-map.md` for bug/regression symptom routing and root-cause search recipes.

Memory policy:

- Durable agent memory belongs in `AGENTS.md` and `.agents/skills/**/references/*.md`.
- `docs/current-states.md` is only for active plan handoff.
- `docs/plan-closed/*` is historical and may be deleted. Future agents must not depend on closed plans as a source of truth.

Mandatory mapping policy:

- Agents and subagents must load `consflow-module-memory` and the central map before broad scans, diagnosis, review, implementation planning, or code edits.
- Agents then load backend/frontend/root-cause/domain detail maps according to the task.
- Agent/subagent summaries should include mapping files loaded, matched feature/domain/function group/UI surface, owner files, and first targeted searches.
- Mapping maintenance gate: map-affecting code changes must update the relevant mapping files in the same work turn, or explicitly state why no mapping update was needed.

Human review gates:

- Feature, bugfix, or code-changing requests require a review package and explicit user confirmation before implementation.
- UI or visual-interface changes require a "Mockup as Text" and explicit user confirmation before implementation.

References used when creating this setup:

- OpenAI Codex Agent Skills: https://developers.openai.com/codex/skills
- OpenAI Codex AGENTS.md guide: https://developers.openai.com/codex/guides/agents-md
- OpenAI Codex Subagents: https://developers.openai.com/codex/subagents
- Reference skill catalog: https://github.com/khiemnd777/skills

Recommended prompts:

```text
Use the Consflow repo skills to implement this backend feature end to end.
```

```text
Review this change with subagents: one Consflow backend worker, one frontend worker, and one reviewer. Wait for all summaries.
```

```text
Use consflow-diagnose to reproduce and fix this failing admin workflow.
```

```text
Use consflow-module-memory to map "nghiem thu fail tao rework" to the right modules, routes, tables, and domain risks before proposing changes.
```

```text
Use mandatory Consflow mapping bootstrap first. Load the central map, targeted detail maps, owner files, and mapping maintenance gate before reviewing or editing.
```

Validation:

```bash
bash .agents/scripts/validate.sh
```
