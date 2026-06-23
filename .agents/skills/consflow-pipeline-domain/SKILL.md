---
name: consflow-pipeline-domain
description: Legacy broad guardrail for Consflow construction workflow invariants across pipeline templates, runtime runs, approvals, BOQ, procurement, warehouse, acceptance, rework, handover, accounting, and audit flows. Prefer the narrower production-grade domain skills when a prompt clearly matches workflow orchestration, planning, construction operations, commercial procurement, materials warehouse, quality handover, accounting finance, admin governance, or reporting analytics.
---

# Consflow Pipeline Domain

Use this skill when a task touches broad construction workflow state and no narrower production-grade domain skill clearly owns the request.

## Domain Model

Read `references/domain-model.md` before changing workflow behavior. If the task maps cleanly to a narrower skill, use that skill as the primary domain owner:

- `consflow-workflow-orchestration-domain`
- `consflow-project-planning-domain`
- `consflow-construction-operations-domain`
- `consflow-commercial-procurement-domain`
- `consflow-materials-warehouse-domain`
- `consflow-quality-acceptance-handover-domain`
- `consflow-accounting-finance-domain`
- `consflow-admin-governance-domain`
- `consflow-reporting-analytics-domain`

## Required Workflow

1. Identify the source state, target state, actor, and audit consequence.
2. Determine whether the change affects a template, a project runtime record, or both.
3. For feature, bugfix, or code-changing requests, present a review package and wait for explicit user confirmation before implementation.
4. For UI or interface changes, present a "Mockup as Text" and wait for explicit user confirmation before implementation. "Mockup as Text" means an ASCII/wireframe-style visual mockup in text, not only a prose UX specification.
5. Trace downstream modules with the relevant domain skill:
   - BOQ/unit price/procurement: `consflow-commercial-procurement-domain`.
   - Procurement receive/stock/site balance/surplus: `consflow-materials-warehouse-domain`.
   - Acceptance/rework/replanning/handover: `consflow-quality-acceptance-handover-domain`.
   - Payable/receivable/payment/invoice/report consequences: `consflow-accounting-finance-domain`.
   - Workflow form, signature, PDF, work item gates: `consflow-workflow-orchestration-domain`.
6. Check that the frontend module and backend API agree on status names and allowed actions.
7. Preserve history. Do not replace state transitions with blind updates when actor/time/comment history matters.

## Guardrails

- Template changes must not silently alter already-started project runs.
- Approval actions must record actor intent and result.
- Stock movements must remain traceable and balanced.
- Financial state transitions must be explicit and reconciled by `consflow-accounting-finance-domain`.
- Audit views should remain useful after workflow transitions.

## Output Expectations

- State the affected domain flow.
- Name the status transitions being added or changed.
- List downstream modules checked.
- Call out missing audit or test coverage if present.
