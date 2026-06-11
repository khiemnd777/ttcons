---
name: consflow-pipeline-domain
description: Protect Consflow construction workflow invariants across pipeline templates, pipeline runs, approvals, BOQ, procurement, warehouse, acceptance, rework, handover, accounting, and audit flows.
---

# Consflow Pipeline Domain

Use this skill when a task touches construction workflow state, approvals, financial or inventory consequences, or cross-module transitions.

## Domain Model

Read `references/domain-model.md` before changing workflow behavior.

## Required Workflow

1. Identify the source state, target state, actor, and audit consequence.
2. Determine whether the change affects a template, a project runtime record, or both.
3. For feature, bugfix, or code-changing requests, present a review package and wait for explicit user confirmation before implementation.
4. For UI or interface changes, present a "Mockup as Text" and wait for explicit user confirmation before implementation. "Mockup as Text" means an ASCII/wireframe-style visual mockup in text, not only a prose UX specification.
5. Trace downstream modules:
   - BOQ can generate procurement.
   - Procurement can affect warehouse stock.
   - Acceptance failure can create rework.
   - Replanning and rework can affect schedule and approval state.
   - Accounting and debt screens depend on payable/receivable/payment states.
6. Check that the frontend module and backend API agree on status names and allowed actions.
7. Preserve history. Do not replace state transitions with blind updates when actor/time/comment history matters.

## Guardrails

- Template changes must not silently alter already-started project runs.
- Approval actions must record actor intent and result.
- Stock movements must remain traceable and balanced.
- Financial state transitions must be explicit.
- Audit views should remain useful after workflow transitions.

## Output Expectations

- State the affected domain flow.
- Name the status transitions being added or changed.
- List downstream modules checked.
- Call out missing audit or test coverage if present.
