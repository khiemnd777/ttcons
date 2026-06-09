# Consflow Repo Agent Artifacts

This directory stores repo-scoped Codex skills for Consflow.

Codex discovers repo skills from `.agents/skills` when a session starts in this repository or a child directory. Each skill package should contain:

- `SKILL.md` with `name` and `description` frontmatter.
- Optional `agents/openai.yaml` metadata for display and invocation policy.
- Optional `references/`, `scripts/`, or `assets/` only when they directly support the workflow.

Keep skills small and task-specific. Durable project rules belong in `AGENTS.md`; specialized reusable workflows belong here.

Run validation from the repo root:

```bash
bash .agents/scripts/validate.sh
```
