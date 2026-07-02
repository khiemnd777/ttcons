---
name: consflow-workflow-orchestration-domain
description: Protect Consflow workflow orchestration across pipeline templates, pipeline runs, work items, approvals, workflow form templates/versions/instances, required role arrays, signatures, PDF exports, and activity/evidence gates. Use when tasks mention pipeline builder, workflow template, runtime pipeline, work item state, approval, workflow form governance, form publishing, form mapping, signature, PDF artifact, required_role_ids, or step completion gates.
---

# Consflow Workflow Orchestration Domain

Use this skill for the system that turns reusable workflow definitions into project-specific runtime work.

## Required Reading

- `AGENTS.md`
- `.agents/skills/consflow-module-memory/references/module-keyword-map.md`
- `references/domain-map.md`
- Pair with `consflow-backend-workflow` and/or `consflow-frontend-workflow` before implementation.

## Workflow

1. Identify whether the change touches template definition, project runtime copy, work item execution, approval, workflow form governance, or artifact generation.
2. Preserve template/runtime separation. Publishing, cloning, or editing templates must not silently mutate historical project runs.
3. Preserve `required_role_ids` as a multi-role array at template, run, work item, and UI layers.
4. For workflow forms, preserve published-version immutability. Only draft versions should be edited; runtime instances must keep stable version references.
5. For form submit/sign/PDF flows, preserve actor, timestamp, revision, payload hash, checksum, and file visibility.
6. Trace downstream effects into planning, construction, commercial/procurement, quality, and finance domains when work item completion unlocks them.
7. Check backend/frontend status labels, allowed actions, and response fields together.
8. For feature, bugfix, code-changing, or UI-changing requests, stop before editing and produce the repo review package. Include current contract/invariant, evidence, root cause or unresolved risk, clarification questions, implementation plan, validation plan, and any required Mockup as Text. Wait for explicit user confirmation.

## Guardrails

- Do not bypass approval, evidence, signature, or required form gates when completing steps or work items.
- Do not replace append-only activity/audit history with blind state updates.
- Do not duplicate API client, toast, form, table, or dialog logic in feature modules.
- Browser verification is skipped unless the user explicitly asks for it in the same turn.

## Output Expectations

- State which orchestration layer changed: template, runtime, work item, approval, form, signature, PDF, or activity.
- Name downstream domains checked.
- Call out any contract changes in `frontend/src/lib/types/domain.ts`.
- Report focused validation commands and any skipped browser checks.
