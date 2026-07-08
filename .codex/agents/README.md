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

Mandatory mapping bootstrap:

- Subagents must use `.agents/skills/consflow-module-memory/SKILL.md` and `.agents/skills/consflow-module-memory/references/module-keyword-map.md` before broad scans, diagnosis, review, implementation planning, or code edits.
- After the central map is loaded, subagents must load the targeted detail maps that match the task:
  - Backend/API/schema/function work: `.agents/skills/consflow-module-memory/references/backend-function-map.md`.
  - Frontend/UI/helper/API usage work: `.agents/skills/consflow-module-memory/references/frontend-ui-utility-map.md`.
  - Bug/regression/root-cause work: `.agents/skills/consflow-module-memory/references/root-cause-triage-map.md`.
  - Business-domain invariants: relevant `.agents/skills/consflow-*-domain/references/domain-map.md`.
- Subagent summaries must name mapping files loaded, matched feature/domain/function group/UI surface, owner files, and first targeted searches.
- Mapping maintenance gate: if a code-changing subagent changes feature/function/helper/UI/route/contract/table/status/domain behavior, it must update relevant mapping files in the same work turn or ask the parent to do so before finalizing.

Implementation gate:

- Workspace-write agents must not implement feature, bugfix, or code-changing work until the user has reviewed a proposed change package and explicitly confirmed.
- UI changes additionally require a "Mockup as Text" reviewed and confirmed by the user before implementation. "Mockup as Text" means an ASCII/wireframe-style visual mockup in text, not only a prose UX specification.
- Bugfixes and unclear behavior changes must include current behavior, expected behavior, evidence, hypotheses, root cause or unresolved risk, clarification questions, implementation plan, and validation plan before any edit.
- If root cause or scope remains unresolved, subagents must stop at diagnosis/review and ask the parent agent for clarification or explicit approval of the unresolved risk.
