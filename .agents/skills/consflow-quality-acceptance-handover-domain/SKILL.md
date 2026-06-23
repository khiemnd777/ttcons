---
name: consflow-quality-acceptance-handover-domain
description: Protect Consflow quality workflows across incoming acceptance, outgoing acceptance, acceptance checklist items, evidence/signature/PDF gates, pass/conditional-pass/fail decisions, punch list, rework, re-acceptance history, replanning, and handover readiness. Use when tasks mention nghiem thu, acceptance, incoming acceptance, outgoing acceptance, quality, checklist pass fail, punch list, rework, replanning, handover, ban giao, or closure blockers.
---

# Consflow Quality Acceptance Handover Domain

Use this skill for quality gates and final readiness.

## Required Reading

- `AGENTS.md`
- `.agents/skills/consflow-module-memory/references/module-keyword-map.md`
- `references/domain-map.md`
- Pair with `consflow-materials-warehouse-domain` for incoming material acceptance and `consflow-accounting-finance-domain` for handover financial blockers.

## Workflow

1. Identify acceptance type, project, work item, source document, actor, checklist items, evidence, signature/PDF status, and decision.
2. Keep incoming acceptance separate from outgoing acceptance.
3. Preserve fail to punch list/rework to re-acceptance history without erasing previous decisions.
4. Respect handover readiness blockers across acceptance, procurement, warehouse, and accounting states.
5. Keep evidence, signature, and PDF requirements as gates when the local contract requires them.
6. For code or UI changes, follow the repo review package and Mockup as Text gates before editing.

## Guardrails

- Do not silently complete handover when readiness checks fail.
- Do not convert conditional pass or failed acceptance into pass without actor/comment history.
- Do not create implicit stock movement from incoming acceptance unless explicitly required by the existing route/service contract.
- Do not drop acceptance checklist item evidence or rejection reasons.

## Output Expectations

- State affected quality flow: incoming, outgoing, rework, replanning, or handover.
- Name status transitions and blockers.
- List downstream materials, commercial, workflow, and finance domains checked.
- Report focused validation and skipped browser checks.
