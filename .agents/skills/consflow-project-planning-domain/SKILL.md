---
name: consflow-project-planning-domain
description: Protect Consflow project and planning workflows across projects, project members, planning pipeline workspace, planning daily reports, meeting minutes, planning checklists, project activity, and generated planning work items. Use when tasks mention cong trinh, project setup, project member, planning, ke hoach, daily report ke hoach, meeting minutes, bien ban hop, planning checklist, or project activity.
---

# Consflow Project Planning Domain

Use this skill for project setup and planning-phase business work before or around field execution.

## Required Reading

- `AGENTS.md`
- `.agents/skills/consflow-module-memory/references/module-keyword-map.md`
- `references/domain-map.md`
- Pair with `consflow-workflow-orchestration-domain` when planning data comes from workflow form runtime.

## Workflow

1. Identify whether the task is project master data, project membership, planning pipeline status, planning report workspace, or project activity.
2. Preserve project visibility and project-scoped permission checks in backend queries.
3. Treat planning daily reports, meeting minutes, and planning checklists as workflow-form-backed business views unless local code says otherwise.
4. Keep generated action items or next-plan work items traceable to the source form/activity.
5. Preserve dense admin UI patterns, shared dialogs, inline validation, and toast feedback for create/edit flows.
6. For code or UI changes, follow the repo review package and Mockup as Text gates before editing.

## Guardrails

- Do not expose cancelled, deleted, or inactive project records in user-visible lists.
- Do not make planning report pages mutate workflow form submissions unless the task explicitly changes that contract.
- Do not bypass project member or project permission semantics.
- Keep project activity useful for handoff and audit review.

## Output Expectations

- State whether the change affects project setup, planning workspace, planning reports, checklist, meeting minutes, or activity.
- Name the source workflow form type when applicable.
- Call out any generated work item or activity consequences.
- Report focused validation and skipped browser checks.
