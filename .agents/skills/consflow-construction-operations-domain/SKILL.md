---
name: consflow-construction-operations-domain
description: Protect Consflow field execution workflows across operations daily reports, site issues, construction work item execution, evidence capture/review, subcontractor and owner checklist forms, construction daily report forms, and site-facing activity. Use when tasks mention thi cong, operations, daily report thi cong, site issue, work item execution, evidence, photo, checklist thau phu, checklist chu dau tu, crew, actual quantity, or field activity.
---

# Consflow Construction Operations Domain

Use this skill for the field-execution layer that records what happened on site.

## Required Reading

- `AGENTS.md`
- `.agents/skills/consflow-module-memory/references/module-keyword-map.md`
- `references/domain-map.md`
- Pair with `consflow-workflow-orchestration-domain` for work item gates and `consflow-materials-warehouse-domain` for material balance changes.

## Workflow

1. Identify the source work item, project, actor, form instance, evidence, and operational status.
2. Preserve evidence storage and review semantics. File upload, protected file access, accepted/rejected status, checksum, uploader, and reviewer must remain consistent.
3. Keep construction reports and checklist forms backed by real workflow form instances or durable records.
4. Trace any material quantity or surplus implication into the materials/warehouse domain before changing stock or site balance behavior.
5. Keep activity and audit entries useful for site handoff.
6. For feature, bugfix, code-changing, or UI-changing requests, stop before editing and produce the repo review package. Include current contract/invariant, evidence, root cause or unresolved risk, clarification questions, implementation plan, validation plan, and any required Mockup as Text. Wait for explicit user confirmation.

## Guardrails

- Do not add placeholder-only evidence, signature, PDF, checklist, or daily report UI.
- Do not treat form summaries as the only source of truth when submitted answers and evidence records exist.
- Do not create warehouse stock movement from construction UI unless the materials/warehouse domain owns that transition.
- Preserve project-scoped permission checks for every work item and evidence action.

## Output Expectations

- State affected field workflow: daily report, issue, checklist, evidence, work item execution, or activity.
- Name linked form types and evidence requirements.
- List downstream materials, quality, commercial, or finance domains checked.
- Report focused validation and skipped browser checks.
