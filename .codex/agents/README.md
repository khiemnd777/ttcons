# Consflow Custom Agents

Custom agents in this directory are project-scoped subagent profiles.

Use them explicitly when a task benefits from parallel work, for example:

```text
Spawn consflow-explorer to map the code path, consflow-backend-worker to propose the backend patch, and consflow-reviewer to review risks. Wait for all summaries.
```

Available agents:

- `consflow-explorer`: read-only repo exploration.
- `consflow-reviewer`: read-only correctness, security, migration, and test review.
- `consflow-db-reviewer`: read-only migration and data integrity review.
- `consflow-backend-worker`: workspace-write backend implementation.
- `consflow-frontend-worker`: workspace-write frontend implementation.

Subagents inherit the parent session sandbox and approvals at runtime. The TOML files document intended behavior and may set a narrower sandbox for read-only reviewers.

Routing cache:

- Subagents should use `.agents/skills/consflow-module-memory/references/module-keyword-map.md` before broad scans when prompts mention Consflow modules, features, routes, tables, utilities, or domain terms.

Implementation gate:

- Workspace-write agents must not implement feature, bugfix, or code-changing work until the user has reviewed a proposed change package and explicitly confirmed.
- UI changes additionally require a "Mockup as Text" reviewed and confirmed by the user before implementation. "Mockup as Text" means an ASCII/wireframe-style visual mockup in text, not only a prose UX specification.
